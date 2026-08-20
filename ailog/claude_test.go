package ailog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestEncodeProjectDir は cwd の絶対パスに含まれる "/" "." "_" を "-" へ
// 置換してディレクトリ名を得られることを検証する
// （実際の Claude Code のディレクトリ名はアンダースコアも置換される）。
func TestEncodeProjectDir(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "スラッシュ・ドット・アンダースコアがハイフンに置換される",
			cwd:  "/Users/yy_yank/ghq/github.com/yyYank/pinline",
			want: "-Users-yy-yank-ghq-github-com-yyYank-pinline",
		},
		{
			name: "ドットを含まないパスも置換される",
			cwd:  "/Users/yy_yank/work",
			want: "-Users-yy-yank-work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeProjectDir(tt.cwd)
			if got != tt.want {
				t.Errorf("EncodeProjectDir(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}

// TestLatestSessionLogPath は logRoot/<encoded-cwd>/ 配下の *.jsonl のうち
// 最終更新が最も新しいファイルを選べることを検証する。
func TestLatestSessionLogPath(t *testing.T) {
	t.Run("最終更新が最新のjsonlファイルを選ぶ", func(t *testing.T) {
		logRoot := t.TempDir()
		cwd := "/Users/tester/project"
		dir := filepath.Join(logRoot, EncodeProjectDir(cwd))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create log dir: %v", err)
		}

		older := filepath.Join(dir, "session-old.jsonl")
		newer := filepath.Join(dir, "session-new.jsonl")
		other := filepath.Join(dir, "notes.txt")

		for _, p := range []string{older, newer, other} {
			if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
				t.Fatalf("failed to write %s: %v", p, err)
			}
		}

		now := time.Now()
		if err := os.Chtimes(older, now, now.Add(-1*time.Hour)); err != nil {
			t.Fatalf("failed to chtimes older: %v", err)
		}
		if err := os.Chtimes(newer, now, now); err != nil {
			t.Fatalf("failed to chtimes newer: %v", err)
		}

		got, err := LatestSessionLogPath(logRoot, cwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != newer {
			t.Errorf("LatestSessionLogPath() = %q, want %q", got, newer)
		}
	})

	t.Run("ディレクトリが存在しない場合はエラーを返す", func(t *testing.T) {
		logRoot := t.TempDir()
		_, err := LatestSessionLogPath(logRoot, "/no/such/project")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("jsonlファイルが1つも無い場合はエラーを返す", func(t *testing.T) {
		logRoot := t.TempDir()
		cwd := "/Users/tester/empty"
		dir := filepath.Join(logRoot, EncodeProjectDir(cwd))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create log dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err := LatestSessionLogPath(logRoot, cwd)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestLastAssistantText は jsonl ログから最後に現れる assistant の
// テキストを取り出せることを検証する。
func TestLastAssistantText(t *testing.T) {
	t.Run("最後に現れるassistantのtextを採用する", func(t *testing.T) {
		content := `{"type":"user","message":{"content":[{"type":"text","text":"質問1"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"回答1"}]}}
{"type":"user","message":{"content":[{"type":"text","text":"質問2"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"回答2"}]}}
`
		path := writeTempLog(t, content)

		got, err := LastAssistantText(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "回答2" {
			t.Errorf("LastAssistantText() = %q, want %q", got, "回答2")
		}
	})

	t.Run("1メッセージ内の複数text要素は連結される", func(t *testing.T) {
		content := `{"type":"assistant","message":{"content":[{"type":"text","text":"前半"},{"type":"text","text":"後半"}]}}
`
		path := writeTempLog(t, content)

		got, err := LastAssistantText(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "前半後半" {
			t.Errorf("LastAssistantText() = %q, want %q", got, "前半後半")
		}
	})

	t.Run("textを持たないassistantメッセージは直前のtextを上書きしない", func(t *testing.T) {
		content := `{"type":"assistant","message":{"content":[{"type":"text","text":"最後のテキスト"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","text":""}]}}
`
		path := writeTempLog(t, content)

		got, err := LastAssistantText(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "最後のテキスト" {
			t.Errorf("LastAssistantText() = %q, want %q", got, "最後のテキスト")
		}
	})

	t.Run("パースできない行はスキップされる", func(t *testing.T) {
		content := `not a json line
{"type":"assistant","message":{"content":[{"type":"text","text":"有効な回答"}]}}
{broken json
`
		path := writeTempLog(t, content)

		got, err := LastAssistantText(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "有効な回答" {
			t.Errorf("LastAssistantText() = %q, want %q", got, "有効な回答")
		}
	})

	t.Run("assistantのtextが1つも見つからない場合はエラーを返す", func(t *testing.T) {
		content := `{"type":"user","message":{"content":[{"type":"text","text":"質問のみ"}]}}
`
		path := writeTempLog(t, content)

		_, err := LastAssistantText(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("存在しないファイルはエラーを返す", func(t *testing.T) {
		_, err := LastAssistantText(filepath.Join(t.TempDir(), "missing.jsonl"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestReadLastClaudeAnswer は LatestSessionLogPath と LastAssistantText を
// 組み合わせて、logRoot と cwd から直前の AI 回答を取得できることを検証する。
func TestReadLastClaudeAnswer(t *testing.T) {
	t.Run("最新ログの最後のassistant回答を返す", func(t *testing.T) {
		logRoot := t.TempDir()
		cwd := "/Users/tester/project"
		dir := filepath.Join(logRoot, EncodeProjectDir(cwd))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create log dir: %v", err)
		}

		content := `{"type":"assistant","message":{"content":[{"type":"text","text":"直前の回答"}]}}
`
		if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write log file: %v", err)
		}

		got, err := ReadLastClaudeAnswer(logRoot, cwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "直前の回答" {
			t.Errorf("ReadLastClaudeAnswer() = %q, want %q", got, "直前の回答")
		}
	})

	t.Run("ログディレクトリが存在しない場合はエラーを返す", func(t *testing.T) {
		logRoot := t.TempDir()
		_, err := ReadLastClaudeAnswer(logRoot, "/no/such/project")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, nil) {
			t.Fatal("error should not be nil")
		}
	})
}

// TestClaudeLog_LastAnswer は ClaudeLog が Source インターフェースの実装
// として、LogRoot/Cwd から直前の assistant 回答を取得できることを検証する。
func TestClaudeLog_LastAnswer(t *testing.T) {
	t.Run("Sourceインターフェース経由でも直前のassistant回答を取得できる", func(t *testing.T) {
		logRoot := t.TempDir()
		cwd := "/Users/tester/project"
		dir := filepath.Join(logRoot, EncodeProjectDir(cwd))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create log dir: %v", err)
		}

		content := `{"type":"assistant","message":{"content":[{"type":"text","text":"ClaudeLog経由の回答"}]}}
`
		if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write log file: %v", err)
		}

		var src Source = ClaudeLog{LogRoot: logRoot, Cwd: cwd}

		got, err := src.LastAnswer()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ClaudeLog経由の回答" {
			t.Errorf("LastAnswer() = %q, want %q", got, "ClaudeLog経由の回答")
		}
	})

	t.Run("ログが見つからない場合はエラーを返す", func(t *testing.T) {
		src := ClaudeLog{LogRoot: t.TempDir(), Cwd: "/no/such/project"}

		if _, err := src.LastAnswer(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// writeTempLog はテスト用の一時 jsonl ファイルを作成しそのパスを返す。
func writeTempLog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}
	return path
}
