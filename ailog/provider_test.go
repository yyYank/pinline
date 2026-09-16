package ailog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_PINLINE_PROVIDER環境変数で強制指定(t *testing.T) {
	t.Run("codex指定でCodexProviderを返す", func(t *testing.T) {
		getenv := func(key string) string {
			if key == "PINLINE_PROVIDER" {
				return "codex"
			}
			return ""
		}
		p := Detect(getenv, "/some/cwd")
		if _, ok := p.(*CodexProvider); !ok {
			t.Errorf("got %T, want *CodexProvider", p)
		}
	})

	t.Run("claude指定でClaudeProviderを返す", func(t *testing.T) {
		getenv := func(key string) string {
			if key == "PINLINE_PROVIDER" {
				return "claude"
			}
			return ""
		}
		p := Detect(getenv, "/some/cwd")
		if _, ok := p.(*ClaudeProvider); !ok {
			t.Errorf("got %T, want *ClaudeProvider", p)
		}
	})
}

func TestDetect_CLAUDE_CODE_SESSION_IDでClaudeProvider(t *testing.T) {
	sid := "test-session-123"
	getenv := func(key string) string {
		if key == "CLAUDE_CODE_SESSION_ID" {
			return sid
		}
		return ""
	}
	p := Detect(getenv, "/some/cwd")
	cp, ok := p.(*ClaudeProvider)
	if !ok {
		t.Fatalf("got %T, want *ClaudeProvider", p)
	}
	if cp.SessionID != sid {
		t.Errorf("SessionID = %q, want %q", cp.SessionID, sid)
	}
}

func TestDetect_ディレクトリ存在による自動検出(t *testing.T) {
	t.Run("claudeのみ存在する場合はClaudeProvider", func(t *testing.T) {
		home := t.TempDir()
		os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o755)

		t.Setenv("HOME", home)
		p := Detect(func(string) string { return "" }, "/some/cwd")
		if _, ok := p.(*ClaudeProvider); !ok {
			t.Errorf("got %T, want *ClaudeProvider", p)
		}
	})

	t.Run("codexのみ存在する場合はCodexProvider", func(t *testing.T) {
		home := t.TempDir()
		os.MkdirAll(filepath.Join(home, ".codex", "sessions"), 0o755)

		t.Setenv("HOME", home)
		p := Detect(func(string) string { return "" }, "/some/cwd")
		if _, ok := p.(*CodexProvider); !ok {
			t.Errorf("got %T, want *CodexProvider", p)
		}
	})

	t.Run("どちらも存在しない場合はnil", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		p := Detect(func(string) string { return "" }, "/some/cwd")
		if p != nil {
			t.Errorf("got %T, want nil", p)
		}
	})
}

func TestClaudeProvider_Provider実装(t *testing.T) {
	logRoot := t.TempDir()
	cwd := "/Users/tester/project"
	dir := filepath.Join(logRoot, EncodeProjectDir(cwd))
	os.MkdirAll(dir, 0o755)

	content := `{"type":"assistant","timestamp":"2026-01-01T10:00:00.000Z","message":{"content":[{"type":"text","text":"回答1"}]}}
{"type":"assistant","timestamp":"2026-01-01T10:02:00.000Z","message":{"content":[{"type":"text","text":"回答2"}]}}
`
	os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(content), 0o644)

	p := &ClaudeProvider{LogRoot: logRoot, Cwd: cwd}

	t.Run("LastAnswer", func(t *testing.T) {
		got, err := p.LastAnswer()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "回答2" {
			t.Errorf("got %q, want %q", got, "回答2")
		}
	})

	t.Run("CurrentLogPath", func(t *testing.T) {
		path, err := p.CurrentLogPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(path) {
			t.Errorf("got relative path %q", path)
		}
	})

	t.Run("ReadAssistantTexts", func(t *testing.T) {
		path, _ := p.CurrentLogPath()
		entries, err := p.ReadAssistantTexts(path, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}
	})
}

func TestCodexProvider_Provider実装(t *testing.T) {
	logRoot := t.TempDir()
	cwd := "/Users/tester/project"

	sessionDir := filepath.Join(logRoot, "2026", "01", "01")
	os.MkdirAll(sessionDir, 0o755)

	content := `{"timestamp":"2026-01-01T10:00:00.000Z","type":"session_meta","payload":{"session_id":"sess-1","cwd":"/Users/tester/project","timestamp":"2026-01-01T10:00:00.000Z","git":{"branch":"main"}}}
{"timestamp":"2026-01-01T10:01:00.000Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"質問です"}]}}
{"timestamp":"2026-01-01T10:02:00.000Z","type":"response_item","payload":{"role":"assistant","content":[{"type":"output_text","text":"回答1"}]}}
{"timestamp":"2026-01-01T10:03:00.000Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"もう一つ"}]}}
{"timestamp":"2026-01-01T10:04:00.000Z","type":"response_item","payload":{"role":"assistant","content":[{"type":"output_text","text":"回答2"}]}}
`
	os.WriteFile(filepath.Join(sessionDir, "rollout-2026-01-01T10-00-00-sess-1.jsonl"), []byte(content), 0o644)

	p := &CodexProvider{LogRoot: logRoot, Cwd: cwd}

	t.Run("LastAnswer", func(t *testing.T) {
		got, err := p.LastAnswer()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "回答2" {
			t.Errorf("got %q, want %q", got, "回答2")
		}
	})

	t.Run("CurrentLogPath", func(t *testing.T) {
		path, err := p.CurrentLogPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(path) {
			t.Errorf("got relative path %q", path)
		}
	})

	t.Run("ListSessions", func(t *testing.T) {
		sessions, err := p.ListSessions(10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(sessions))
		}
		if sessions[0].SessionID != "sess-1" {
			t.Errorf("SessionID = %q, want %q", sessions[0].SessionID, "sess-1")
		}
		if sessions[0].GitBranch != "main" {
			t.Errorf("GitBranch = %q, want %q", sessions[0].GitBranch, "main")
		}
	})

	t.Run("ReadAssistantTexts", func(t *testing.T) {
		path, _ := p.CurrentLogPath()
		entries, err := p.ReadAssistantTexts(path, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}
		if entries[0].Text != "回答2" {
			t.Errorf("entries[0].Text = %q, want %q", entries[0].Text, "回答2")
		}
	})

	t.Run("ReadEntries", func(t *testing.T) {
		path, _ := p.CurrentLogPath()
		entries, err := p.ReadEntries(path, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 4 {
			t.Fatalf("got %d entries, want 4", len(entries))
		}
		if entries[0].Role != "assistant" {
			t.Errorf("entries[0].Role = %q, want %q", entries[0].Role, "assistant")
		}
	})

	t.Run("cwdが一致しないセッションは除外される", func(t *testing.T) {
		otherP := &CodexProvider{LogRoot: logRoot, Cwd: "/other/project"}
		sessions, err := otherP.ListSessions(10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(sessions))
		}
	})
}

func TestCodexProvider_システムメッセージはフィルタされる(t *testing.T) {
	logRoot := t.TempDir()
	cwd := "/Users/tester/project"
	sessionDir := filepath.Join(logRoot, "2026", "01", "01")
	os.MkdirAll(sessionDir, 0o755)

	content := `{"timestamp":"2026-01-01T10:00:00.000Z","type":"session_meta","payload":{"session_id":"sess-1","cwd":"/Users/tester/project","timestamp":"2026-01-01T10:00:00.000Z","git":{"branch":"main"}}}
{"timestamp":"2026-01-01T10:01:00.000Z","type":"response_item","payload":{"role":"developer","content":[{"type":"input_text","text":"システムプロンプト"}]}}
{"timestamp":"2026-01-01T10:02:00.000Z","type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"<permissions>system content</permissions>"},{"type":"input_text","text":"ユーザーの質問"}]}}
{"timestamp":"2026-01-01T10:03:00.000Z","type":"response_item","payload":{"role":"assistant","content":[{"type":"output_text","text":"AIの回答"}]}}
`
	os.WriteFile(filepath.Join(sessionDir, "rollout-sess-1.jsonl"), []byte(content), 0o644)

	p := &CodexProvider{LogRoot: logRoot, Cwd: cwd}
	path, _ := p.CurrentLogPath()

	entries, err := p.ReadEntries(path, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, e := range entries {
		if e.Role == "developer" {
			t.Error("developer role should be filtered out")
		}
		if e.Text == "<permissions>system content</permissions>" {
			t.Error("XML-prefixed system content should be filtered out")
		}
	}
}
