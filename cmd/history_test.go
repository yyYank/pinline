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

func fakeRunTUI(selectIndex int) func(tui.SelectorModel) (tui.SelectorModel, error) {
	return func(m tui.SelectorModel) (tui.SelectorModel, error) {
		for i := 0; i < selectIndex; i++ {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = next.(tui.SelectorModel)
		}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(tui.SelectorModel)
		return m, nil
	}
}

func fakeRunTUICancelled() func(tui.SelectorModel) (tui.SelectorModel, error) {
	return func(m tui.SelectorModel) (tui.SelectorModel, error) {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		return next.(tui.SelectorModel), nil
	}
}

func alwaysHasTTY() bool { return true }

func setupTestLog(t *testing.T) (string, string) {
	t.Helper()
	logRoot := t.TempDir()
	cwd := "/Users/tester/project"
	dir := filepath.Join(logRoot, ailog.EncodeProjectDir(cwd))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}

	content := `{"type":"assistant","timestamp":"2026-01-01T10:00:00.000Z","message":{"content":[{"type":"text","text":"最初の回答"}]}}
{"type":"assistant","timestamp":"2026-01-01T10:02:00.000Z","message":{"content":[{"type":"text","text":"二番目の回答"}]}}
{"type":"assistant","timestamp":"2026-01-01T10:04:00.000Z","message":{"content":[{"type":"text","text":"三番目の回答"}]}}
`
	if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}

	return logRoot, cwd
}

// 選択したエントリがblockquote化されてエディタで開かれ、stdoutに出力される
func TestRunHistory_選択してエディタで開く(t *testing.T) {
	logRoot, cwd := setupTestLog(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake editor: %v", err)
	}

	deps := historyDeps{
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
		OpenEditor: fakeOpenEditor,
		RunTUI:     fakeRunTUI(0),
	}

	var stdout, stderr bytes.Buffer
	err := runHistory(deps, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.String() != "> 三番目の回答" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "> 三番目の回答")
	}
}

// キャンセルした場合はerrHistoryCancelledを返す
func TestRunHistory_キャンセル(t *testing.T) {
	logRoot, cwd := setupTestLog(t)

	deps := historyDeps{
		LogRoot: logRoot,
		Cwd:     cwd,
		N:       10,
		HasTTY:  alwaysHasTTY,
		Getenv:  func(string) string { return "" },
		RunTUI:  fakeRunTUICancelled(),
	}

	var stdout, stderr bytes.Buffer
	err := runHistory(deps, &stdout, &stderr)
	if err != errHistoryCancelled {
		t.Errorf("err = %v, want errHistoryCancelled", err)
	}
}

// ログが見つからない場合はエラーを返す
func TestRunHistory_ログなし(t *testing.T) {
	deps := historyDeps{
		LogRoot: t.TempDir(),
		Cwd:     "/no/such/project",
		N:       10,
	}

	var stdout, stderr bytes.Buffer
	err := runHistory(deps, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// セッションID環境変数が設定されている場合、そのセッションのログを使う
func TestRunHistory_セッションID環境変数で特定(t *testing.T) {
	logRoot := t.TempDir()
	cwd := "/Users/tester/project"
	dir := filepath.Join(logRoot, ailog.EncodeProjectDir(cwd))
	os.MkdirAll(dir, 0o755)

	// ターゲットセッション（セッションID指定で選ばれるべき）
	targetID := "target-session-id"
	os.WriteFile(filepath.Join(dir, targetID+".jsonl"), []byte(
		`{"type":"assistant","timestamp":"2026-01-01T10:00:00.000Z","message":{"content":[{"type":"text","text":"ターゲットの回答"}]}}
`), 0o644)

	// mtimeが新しい別セッション（セッションID未指定なら こちらが選ばれる）
	otherFile := filepath.Join(dir, "other.jsonl")
	os.WriteFile(otherFile, []byte(
		`{"type":"assistant","timestamp":"2026-01-01T11:00:00.000Z","message":{"content":[{"type":"text","text":"別セッションの回答"}]}}
`), 0o644)

	deps := historyDeps{
		LogRoot: logRoot,
		Cwd:     cwd,
		N:       10,
		HasTTY:  alwaysHasTTY,
		Getenv: func(key string) string {
			if key == "CLAUDE_CODE_SESSION_ID" {
				return targetID
			}
			if key == "PINLINE_EDITOR" {
				return "true"
			}
			return ""
		},
		OpenEditor: fakeOpenEditor,
		RunTUI:     fakeRunTUI(0),
	}

	var stdout, stderr bytes.Buffer
	err := runHistory(deps, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "> ターゲットの回答" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "> ターゲットの回答")
	}
}

// historyサブコマンドが登録されている
func TestNewRootCmd_historyサブコマンド登録(t *testing.T) {
	cmd := NewRootCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "history" {
			found = true
			if sub.Flags().Lookup("number") == nil {
				t.Error("-n/--number flag is not registered")
			}
		}
	}
	if !found {
		t.Error("history subcommand is not registered")
	}
}

// TTY無しかつtmux無しの場合はエラーメッセージを返す
func TestRunHistory_TTY無しtmux無し(t *testing.T) {
	logRoot, cwd := setupTestLog(t)

	deps := historyDeps{
		LogRoot:       logRoot,
		Cwd:           cwd,
		N:             10,
		HasTTY:        func() bool { return false },
		Getenv:        func(string) string { return "" },
		TmuxAvailable: func() bool { return false },
	}

	var stdout, stderr bytes.Buffer
	err := runHistory(deps, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
