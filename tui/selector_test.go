package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yyYank/pinline/ailog"
)

func makeEntries() []ailog.AssistantEntry {
	return []ailog.AssistantEntry{
		{Text: "最初の回答です", Timestamp: "2026-01-01T10:00:00.000Z"},
		{Text: "二番目の回答です", Timestamp: "2026-01-01T10:02:00.000Z"},
		{Text: "三番目の回答です", Timestamp: "2026-01-01T10:04:00.000Z"},
	}
}

func update(m SelectorModel, msg tea.Msg) (SelectorModel, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(SelectorModel), cmd
}

func keyRunes(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// 初期状態ではカーソルが0番目にある
func TestSelectorModel_初期状態(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	if m.Cursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.Cursor())
	}
	if m.Selected() != nil {
		t.Error("initial selected should be nil")
	}
	if m.Cancelled() {
		t.Error("initial cancelled should be false")
	}
}

// jキーまたは↓キーでカーソルが下に移動する
func TestSelectorModel_カーソル下移動(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, _ = update(m, keyRunes('j'))
	if m.Cursor() != 1 {
		t.Errorf("cursor after j = %d, want 1", m.Cursor())
	}

	m2 := NewSelectorModel(makeEntries())
	m2, _ = update(m2, tea.KeyMsg{Type: tea.KeyDown})
	if m2.Cursor() != 1 {
		t.Errorf("cursor after down = %d, want 1", m2.Cursor())
	}
}

// kキーまたは↑キーでカーソルが上に移動する
func TestSelectorModel_カーソル上移動(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, _ = update(m, keyRunes('j'))
	m, _ = update(m, keyRunes('k'))
	if m.Cursor() != 0 {
		t.Errorf("cursor after j,k = %d, want 0", m.Cursor())
	}
}

// カーソルは先頭・末尾でクランプされる
func TestSelectorModel_カーソルクランプ(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, _ = update(m, keyRunes('k'))
	if m.Cursor() != 0 {
		t.Errorf("cursor should not go below 0, got %d", m.Cursor())
	}

	for i := 0; i < 10; i++ {
		m, _ = update(m, keyRunes('j'))
	}
	if m.Cursor() != 2 {
		t.Errorf("cursor should clamp at max, got %d", m.Cursor())
	}
}

// Enterで選択される
func TestSelectorModel_Enter選択(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, _ = update(m, keyRunes('j'))
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel == nil {
		t.Fatal("selected should not be nil after Enter")
	}
	if sel.Text != "二番目の回答です" {
		t.Errorf("selected text = %q, want %q", sel.Text, "二番目の回答です")
	}
	if cmd == nil {
		t.Error("Enter should produce a tea.Quit command")
	}
}

// Escで中断される
func TestSelectorModel_Escキャンセル(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEscape})

	if !m.Cancelled() {
		t.Error("should be cancelled after Esc")
	}
	if cmd == nil {
		t.Error("Esc should produce a tea.Quit command")
	}
}

// Viewは空でないことだけ確認する
func TestSelectorModel_View出力(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	v := m.View()
	if v == "" {
		t.Error("View should not be empty")
	}
}

// 文字入力でfuzzy絞り込みが行われる
func TestSelectorModel_fuzzy絞り込み(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	// "二番" と入力して絞り込む
	for _, r := range "二番" {
		m, _ = update(m, keyRunes(r))
	}
	filtered := m.FilteredEntries()
	if len(filtered) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(filtered))
	}
	if filtered[0].Text != "二番目の回答です" {
		t.Errorf("filtered[0].Text = %q, want %q", filtered[0].Text, "二番目の回答です")
	}
}

// フィルタが空の時は全件表示される
func TestSelectorModel_フィルタ空で全件表示(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	filtered := m.FilteredEntries()
	if len(filtered) != 3 {
		t.Errorf("filtered count = %d, want 3", len(filtered))
	}
}

// フィルタ入力中も↑↓で移動できる
func TestSelectorModel_フィルタ中の矢印キー移動(t *testing.T) {
	entries := []ailog.AssistantEntry{
		{Text: "ABC回答", Timestamp: "2026-01-01T10:00:00.000Z"},
		{Text: "ABC応答", Timestamp: "2026-01-01T10:02:00.000Z"},
		{Text: "XYZ回答", Timestamp: "2026-01-01T10:04:00.000Z"},
	}
	m := NewSelectorModel(entries)
	// "ABC" で絞り込み → 2件
	for _, r := range "ABC" {
		m, _ = update(m, keyRunes(r))
	}
	if len(m.FilteredEntries()) != 2 {
		t.Fatalf("filtered count = %d, want 2", len(m.FilteredEntries()))
	}
	// ↓で移動
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Errorf("cursor after down = %d, want 1", m.Cursor())
	}
	// Enterで選択 → 絞り込み後の2番目
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel == nil {
		t.Fatal("selected should not be nil")
	}
	if sel.Text != "ABC応答" {
		t.Errorf("selected = %q, want %q", sel.Text, "ABC応答")
	}
}

// Backspaceでフィルタ文字を削除できる
func TestSelectorModel_Backspaceフィルタ削除(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	for _, r := range "二番" {
		m, _ = update(m, keyRunes(r))
	}
	if len(m.FilteredEntries()) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(m.FilteredEntries()))
	}
	// Backspace 2回でフィルタクリア → 全件に戻る
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if len(m.FilteredEntries()) != 3 {
		t.Errorf("filtered count after backspace = %d, want 3", len(m.FilteredEntries()))
	}
}

// Ctrl+Pでプレビューがトグルされる
func TestSelectorModel_プレビュートグル(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, _ = update(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	view1 := m.View()
	if strings.Contains(view1, "│") {
		t.Fatal("expected no divider before Ctrl+P")
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	view2 := m.View()
	if !strings.Contains(view2, "│") {
		t.Fatal("expected divider after Ctrl+P")
	}
	if !strings.Contains(view2, "最初の回答です") {
		t.Fatalf("expected preview text, got:\n%s", view2)
	}
}

// プレビュー中にカーソル移動でプレビューが切り替わる
func TestSelectorModel_プレビュー切替(t *testing.T) {
	m := NewSelectorModel(makeEntries())
	m, _ = update(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})

	view := m.View()
	if !strings.Contains(view, "二番目の回答です") {
		t.Fatalf("expected second entry preview, got:\n%s", view)
	}
}
