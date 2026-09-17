package ailog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CodexProvider は Codex (OpenAI) のセッションログに対する Provider 実装。
type CodexProvider struct {
	LogRoot string
	Cwd     string
}

type codexLogLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	Git       struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

type codexResponsePayload struct {
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (p *CodexProvider) LastAnswer() (string, error) {
	logPath, err := p.CurrentLogPath()
	if err != nil {
		return "", err
	}
	return codexLastAssistantText(logPath)
}

func (p *CodexProvider) ListSessions(n int) ([]SessionInfo, error) {
	files, err := codexFindLogs(p.LogRoot)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, f := range files {
		meta, searchText, ok := codexReadSessionMeta(f)
		if !ok || meta.Cwd != p.Cwd {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		sessions = append(sessions, SessionInfo{
			SessionID:  meta.SessionID,
			LogPath:    f,
			GitBranch:  meta.Git.Branch,
			StartedAt:  meta.Timestamp,
			ModTime:    info.ModTime(),
			SearchText: searchText,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	if n > 0 && n < len(sessions) {
		sessions = sessions[:n]
	}
	return sessions, nil
}

func (p *CodexProvider) CurrentLogPath() (string, error) {
	files, err := codexFindLogs(p.LogRoot)
	if err != nil {
		return "", err
	}

	var latest string
	var latestMod time.Time

	for _, f := range files {
		meta, _, ok := codexReadSessionMeta(f)
		if !ok || meta.Cwd != p.Cwd {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if latest == "" || info.ModTime().After(latestMod) {
			latest = f
			latestMod = info.ModTime()
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no codex session log found for %s", p.Cwd)
	}
	return latest, nil
}

func (p *CodexProvider) ReadAssistantTexts(logPath string, n int) ([]AssistantEntry, error) {
	return codexLastNAssistantTexts(logPath, n)
}

func (p *CodexProvider) ReadEntries(logPath string, n int) ([]AssistantEntry, error) {
	return codexLastNEntries(logPath, n)
}

var _ Provider = (*CodexProvider)(nil)

func codexFindLogs(logRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(logRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk codex session directory %s: %w", logRoot, err)
	}
	return files, nil
}

func codexReadSessionMeta(path string) (codexSessionMeta, string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return codexSessionMeta{}, "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var meta codexSessionMeta
	var foundMeta bool
	var textParts []string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var logLine codexLogLine
		if err := json.Unmarshal(line, &logLine); err != nil {
			continue
		}

		if !foundMeta && logLine.Type == "session_meta" {
			if err := json.Unmarshal(logLine.Payload, &meta); err == nil {
				foundMeta = true
			}
		}

		if logLine.Type == "response_item" {
			var resp codexResponsePayload
			if err := json.Unmarshal(logLine.Payload, &resp); err != nil {
				continue
			}
			if resp.Role == "user" {
				for _, c := range resp.Content {
					if c.Type == "input_text" && c.Text != "" && !strings.HasPrefix(c.Text, "<") {
						textParts = append(textParts, c.Text)
					}
				}
			} else if resp.Role == "assistant" {
				for _, c := range resp.Content {
					if c.Type == "output_text" && c.Text != "" {
						textParts = append(textParts, c.Text)
					}
				}
			}
		}
	}

	if !foundMeta {
		return codexSessionMeta{}, "", false
	}
	return meta, strings.Join(textParts, "\n\n"), true
}

func codexLastAssistantText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open codex session log: %w", err)
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

		var logLine codexLogLine
		if err := json.Unmarshal(line, &logLine); err != nil {
			continue
		}
		if logLine.Type != "response_item" {
			continue
		}

		var resp codexResponsePayload
		if err := json.Unmarshal(logLine.Payload, &resp); err != nil {
			continue
		}
		if resp.Role != "assistant" {
			continue
		}

		var texts []string
		for _, c := range resp.Content {
			if c.Type == "output_text" && c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			lastText = strings.Join(texts, "")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read codex session log: %w", err)
	}
	if lastText == "" {
		return "", errors.New("no assistant text found in codex session log")
	}
	return lastText, nil
}

func codexLastNAssistantTexts(logPath string, n int) ([]AssistantEntry, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open codex session log: %w", err)
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

		var logLine codexLogLine
		if err := json.Unmarshal(line, &logLine); err != nil {
			continue
		}
		if logLine.Type != "response_item" {
			continue
		}

		var resp codexResponsePayload
		if err := json.Unmarshal(logLine.Payload, &resp); err != nil {
			continue
		}
		if resp.Role != "assistant" {
			continue
		}

		var texts []string
		for _, c := range resp.Content {
			if c.Type == "output_text" && c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			all = append(all, AssistantEntry{
				Text:      strings.Join(texts, ""),
				Timestamp: logLine.Timestamp,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read codex session log: %w", err)
	}
	if len(all) == 0 {
		return nil, errors.New("no assistant text found in codex session log")
	}

	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if n > 0 && n < len(all) {
		all = all[:n]
	}
	return all, nil
}

func codexLastNEntries(logPath string, n int) ([]AssistantEntry, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open codex session log: %w", err)
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

		var logLine codexLogLine
		if err := json.Unmarshal(line, &logLine); err != nil {
			continue
		}
		if logLine.Type != "response_item" {
			continue
		}

		var resp codexResponsePayload
		if err := json.Unmarshal(logLine.Payload, &resp); err != nil {
			continue
		}
		if resp.Role != "user" && resp.Role != "assistant" {
			continue
		}

		var texts []string
		for _, c := range resp.Content {
			if c.Text == "" {
				continue
			}
			if resp.Role == "user" {
				if c.Type == "input_text" && !strings.HasPrefix(c.Text, "<") {
					texts = append(texts, c.Text)
				}
			} else {
				if c.Type == "output_text" {
					texts = append(texts, c.Text)
				}
			}
		}
		if len(texts) > 0 {
			joined := strings.Join(texts, "")
			if isSystemMessage(joined) {
				continue
			}
			all = append(all, AssistantEntry{
				Text:      joined,
				Timestamp: logLine.Timestamp,
				Role:      resp.Role,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read codex session log: %w", err)
	}
	if len(all) == 0 {
		return nil, errors.New("no text entries found in codex session log")
	}

	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if n > 0 && n < len(all) {
		all = all[:n]
	}
	return all, nil
}
