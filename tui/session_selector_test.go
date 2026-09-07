package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yyYank/pinline/ailog"
)

func testSessions() []ailog.SessionInfo {
	return []ailog.SessionInfo{
		{SessionID: "aaa-111", GitBranch: "main", StartedAt: "2026-01-01T10:00:00.000Z", ModTime: time.Now()},
		{SessionID: "bbb-222", GitBranch: "feature/x", StartedAt: "2026-01-01T09:00:00.000Z", ModTime: time.Now().Add(-1 * time.Hour)},
		{SessionID: "ccc-333", GitBranch: "fix/y", StartedAt: "2026-01-01T08:00:00.000Z", ModTime: time.Now().Add(-2 * time.Hour)},
	}
}

// Enterで先頭のセッションが選択される
func TestSessionSelectorModel_選択(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(SessionSelectorModel)

	if m.Cancelled() {
		t.Fatal("expected not cancelled")
	}
	sel := m.Selected()
	if sel == nil {
		t.Fatal("expected selected, got nil")
	}
	if sel.SessionID != "aaa-111" {
		t.Errorf("Selected().SessionID = %q, want %q", sel.SessionID, "aaa-111")
	}
}

// Escでキャンセルされる
func TestSessionSelectorModel_キャンセル(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(SessionSelectorModel)

	if !m.Cancelled() {
		t.Fatal("expected cancelled")
	}
	if m.Selected() != nil {
		t.Fatal("expected nil selected")
	}
}

// ↓で移動してEnterで2番目が選択される
func TestSessionSelectorModel_カーソル移動(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(SessionSelectorModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(SessionSelectorModel)

	sel := m.Selected()
	if sel == nil {
		t.Fatal("expected selected, got nil")
	}
	if sel.SessionID != "bbb-222" {
		t.Errorf("Selected().SessionID = %q, want %q", sel.SessionID, "bbb-222")
	}
}

// Viewにセッション情報が含まれる
func TestSessionSelectorModel_View(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	view := m.View()

	if len(view) == 0 {
		t.Fatal("View() is empty")
	}
}
