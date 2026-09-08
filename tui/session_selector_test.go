package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yyYank/pinline/ailog"
)

func testSessions() []ailog.SessionInfo {
	return []ailog.SessionInfo{
		{SessionID: "aaa-111", GitBranch: "main", StartedAt: "2026-01-01T10:00:00.000Z", ModTime: time.Now(), SearchText: "hello world\n\nfrom session one"},
		{SessionID: "bbb-222", GitBranch: "feature/x", StartedAt: "2026-01-01T09:00:00.000Z", ModTime: time.Now().Add(-1 * time.Hour), SearchText: "fuzzy search testing session two"},
		{SessionID: "ccc-333", GitBranch: "fix/y", StartedAt: "2026-01-01T08:00:00.000Z", ModTime: time.Now().Add(-2 * time.Hour), SearchText: "bug fix session three"},
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

// デフォルトではプレビュー非表示（│がない）
func TestSessionSelectorModel_プレビュー初期非表示(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(SessionSelectorModel)

	view := m.View()
	if strings.Contains(view, "│") {
		t.Fatal("expected no divider before pressing p")
	}
}

// pキーでプレビュー表示がトグルされる
func TestSessionSelectorModel_プレビュートグル(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(SessionSelectorModel)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(SessionSelectorModel)

	view := m.View()
	if !strings.Contains(view, "│") {
		t.Fatal("expected vertical divider after pressing p")
	}
	if !strings.Contains(view, "hello world") {
		t.Fatalf("expected preview of first session's SearchText, got:\n%s", view)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(SessionSelectorModel)

	view = m.View()
	if strings.Contains(view, "│") {
		t.Fatal("expected no divider after pressing p again")
	}
}

// プレビューで改行が保持される
func TestSessionSelectorModel_プレビュー改行(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(SessionSelectorModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(SessionSelectorModel)

	view := m.View()
	lines := strings.Split(view, "\n")
	foundEmpty := false
	for _, l := range lines {
		if strings.Contains(l, "│") {
			parts := strings.SplitN(l, "│", 2)
			right := strings.TrimSpace(parts[1])
			if right == "" {
				foundEmpty = true
				break
			}
		}
	}
	if !foundEmpty {
		t.Fatal("expected empty preview line from \\n\\n in SearchText")
	}
}

// Ctrl+Dでプレビューが下スクロールする
func TestSessionSelectorModel_プレビュースクロール(t *testing.T) {
	var sb strings.Builder
	for i := range 50 {
		fmt.Fprintf(&sb, "line-%d\n", i)
	}
	longText := sb.String()
	sessions := []ailog.SessionInfo{
		{SessionID: "aaa", GitBranch: "main", StartedAt: "2026-01-01T10:00:00.000Z", ModTime: time.Now(), SearchText: longText},
	}
	m := NewSessionSelectorModel(sessions)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m = next.(SessionSelectorModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(SessionSelectorModel)

	view1 := m.View()

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = next.(SessionSelectorModel)

	view2 := m.View()
	if view1 == view2 {
		t.Fatal("expected preview to change after Ctrl+D scroll")
	}
}

// プレビュー中にカーソル移動でプレビュー内容が変わる
func TestSessionSelectorModel_プレビュー切替(t *testing.T) {
	m := NewSessionSelectorModel(testSessions())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(SessionSelectorModel)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = next.(SessionSelectorModel)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(SessionSelectorModel)

	view := m.View()
	if !strings.Contains(view, "fuzzy search") {
		t.Fatalf("expected preview of second session after cursor down, got:\n%s", view)
	}
}
