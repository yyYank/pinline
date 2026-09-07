// Package ailog は「直前の AI 回答テキストを取得する」処理の抽象と、
// その実装（Claude Code セッションログ等）を提供する。
package ailog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// claudeLogLine は Claude Code のセッションログ（jsonl）1行分のうち、
// 本実装で必要なフィールドのみを表す。未知のフィールドは無視する。
type claudeLogLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// AssistantEntry は assistant 応答1件分のテキストとタイムスタンプを保持する。
type AssistantEntry struct {
	Text      string
	Timestamp string
}

// EncodeProjectDir は Claude Code のセッションログ格納ディレクトリ名の
// エンコード規則に従い、cwd の絶対パスに含まれる "/" "." "_" を "-" へ
// 置換する。
func EncodeProjectDir(cwd string) string {
	r := strings.NewReplacer("/", "-", ".", "-", "_", "-")
	return r.Replace(cwd)
}

// LatestSessionLogPath は logRoot/<encoded-cwd>/ 配下にある *.jsonl の中で
// 最終更新（mtime）が最も新しいファイルのパスを返す。
func LatestSessionLogPath(logRoot, cwd string) (string, error) {
	dir := filepath.Join(logRoot, EncodeProjectDir(cwd))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read claude session log directory %s: %w", dir, err)
	}

	var latestPath string
	var latestModTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if latestPath == "" || info.ModTime().After(latestModTime) {
			latestPath = filepath.Join(dir, e.Name())
			latestModTime = info.ModTime()
		}
	}

	if latestPath == "" {
		return "", fmt.Errorf("no claude session log (*.jsonl) found in %s", dir)
	}
	return latestPath, nil
}

// LastAssistantText は path の jsonl ログをスキャンし、最後に現れる
// "type":"assistant" 行の message.content 内 text を返す。
//
// 1メッセージ内に複数の text 要素がある場合はそのまま連結する
// （要素間に区切り文字は挿入しない）。text を含まない assistant
// メッセージ（tool_use のみ等）は、直前に見つかった text を上書きしない。
// パースできない行は無視してスキップする。
func LastAssistantText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open claude session log: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var lastText string
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var entry claudeLogLine
		if err := json.Unmarshal(line, &entry); err != nil {
			// パースできない行はスキップする。
			continue
		}
		if entry.Type != "assistant" {
			continue
		}

		var texts []string
		for _, c := range entry.Message.Content {
			if c.Type == "text" && c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			lastText = strings.Join(texts, "")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read claude session log: %w", err)
	}

	if lastText == "" {
		return "", errors.New("no assistant text found in claude session log")
	}
	return lastText, nil
}

// LastNAssistantTexts は path の jsonl ログをスキャンし、テキストを含む
// assistant メッセージを直近 n 件、新しい順で返す。
func LastNAssistantTexts(path string, n int) ([]AssistantEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open claude session log: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var all []AssistantEntry
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var entry claudeLogLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" {
			continue
		}

		var texts []string
		for _, c := range entry.Message.Content {
			if c.Type == "text" && c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			all = append(all, AssistantEntry{
				Text:      strings.Join(texts, ""),
				Timestamp: entry.Timestamp,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read claude session log: %w", err)
	}

	if len(all) == 0 {
		return nil, errors.New("no assistant text found in claude session log")
	}

	// 新しい順に並べ替え、先頭n件を返す
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if n < len(all) {
		all = all[:n]
	}
	return all, nil
}

// ReadLastClaudeAnswer は logRoot/<encoded-cwd>/ から最も新しいセッション
// ログを選び、そこに含まれる最後の assistant 回答テキストを返す。
func ReadLastClaudeAnswer(logRoot, cwd string) (string, error) {
	path, err := LatestSessionLogPath(logRoot, cwd)
	if err != nil {
		return "", err
	}
	return LastAssistantText(path)
}

// ClaudeLog は Claude Code のセッションログから直前の AI 回答を取得する
// Source の実装。
type ClaudeLog struct {
	// LogRoot はセッションログのルートディレクトリ
	// （通常は ~/.claude/projects）。
	LogRoot string
	// Cwd は対象プロジェクトの作業ディレクトリの絶対パス。
	Cwd string
}

// LastAnswer は Source インターフェースの実装であり、ReadLastClaudeAnswer
// へ処理を委譲する。
func (c ClaudeLog) LastAnswer() (string, error) {
	return ReadLastClaudeAnswer(c.LogRoot, c.Cwd)
}

var _ Source = ClaudeLog{}
