package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yyYank/pinline/ailog"
	"github.com/yyYank/pinline/tui"
)

func setupTestSessions(t *testing.T) (string, string) {
	t.Helper()
	logRoot := t.TempDir()
	cwd := "/Users/tester/project"
	dir := filepath.Join(logRoot, ailog.EncodeProjectDir(cwd))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}

	s1 := `{"type":"user","timestamp":"2026-01-01T09:00:00.000Z","sessionId":"session-1","gitBranch":"main"}
{"type":"assistant","timestamp":"2026-01-01T09:01:00.000Z","message":{"content":[{"type":"text","text":"セッション1の回答A"}]}}
{"type":"assistant","timestamp":"2026-01-01T09:02:00.000Z","message":{"content":[{"type":"text","text":"セッション1の回答B"}]}}
`
	s2 := `{"type":"user","timestamp":"2026-01-01T10:00:00.000Z","sessionId":"session-2","gitBranch":"feature/x"}
{"type":"assistant","timestamp":"2026-01-01T10:01:00.000Z","message":{"content":[{"type":"text","text":"セッション2の回答"}]}}
`
	os.WriteFile(filepath.Join(dir, "session-1.jsonl"), []byte(s1), 0o644)
	os.WriteFile(filepath.Join(dir, "session-2.jsonl"), []byte(s2), 0o644)

	return logRoot, cwd
}

func fakeRunSessionTUI(selectIndex int) func(tui.SessionSelectorModel) (tui.SessionSelectorModel, error) {
	return func(m tui.SessionSelectorModel) (tui.SessionSelectorModel, error) {
		for i := 0; i < selectIndex; i++ {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = next.(tui.SessionSelectorModel)
		}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(tui.SessionSelectorModel)
		return m, nil
	}
}

func fakeRunSessionTUICancelled() func(tui.SessionSelectorModel) (tui.SessionSelectorModel, error) {
	return func(m tui.SessionSelectorModel) (tui.SessionSelectorModel, error) {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		return next.(tui.SessionSelectorModel), nil
	}
}

// セッション選択→応答選択→blockquote化→エディタ→stdout
func TestRunSessions_正常フロー(t *testing.T) {
	logRoot, cwd := setupTestSessions(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-editor.sh")
	os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755)

	deps := sessionsDeps{
		LogRoot: logRoot,
		Cwd:     cwd,
		N:       10,
		HasTTY:  alwaysHasTTY,
		Getenv: func(key string) string {
			if key == "PINLINE_EDITOR" {
				return script
			}
			return ""
		},
		OpenEditor:    fakeOpenEditor,
		RunSessionTUI: fakeRunSessionTUI(0),
		RunTUI:        fakeRunTUI(0),
	}

	var stdout, stderr bytes.Buffer
	err := runSessions(deps, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Error("stdout is empty, expected blockquoted output")
	}
}

// セッション選択でキャンセルした場合
func TestRunSessions_セッション選択キャンセル(t *testing.T) {
	logRoot, cwd := setupTestSessions(t)

	deps := sessionsDeps{
		LogRoot:       logRoot,
		Cwd:           cwd,
		N:             10,
		HasTTY:        alwaysHasTTY,
		Getenv:        func(string) string { return "" },
		RunSessionTUI: fakeRunSessionTUICancelled(),
	}

	var stdout, stderr bytes.Buffer
	err := runSessions(deps, &stdout, &stderr)
	if err != errSessionsCancelled {
		t.Errorf("err = %v, want errSessionsCancelled", err)
	}
}

// ログが見つからない場合はエラー
func TestRunSessions_ログなし(t *testing.T) {
	deps := sessionsDeps{
		LogRoot: t.TempDir(),
		Cwd:     "/no/such/project",
		N:       10,
	}

	var stdout, stderr bytes.Buffer
	err := runSessions(deps, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// sessionsサブコマンドが登録されている
func TestNewRootCmd_sessionsサブコマンド登録(t *testing.T) {
	cmd := NewRootCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "sessions" {
			found = true
			if sub.Flags().Lookup("number") == nil {
				t.Error("-n/--number flag is not registered")
			}
		}
	}
	if !found {
		t.Error("sessions subcommand is not registered")
	}
}
