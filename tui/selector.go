package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/yyYank/pinline/ailog"
)

const previewMaxLen = 80

// SelectorModel は AssistantEntry のリストから1件を選択する TUI モデル。
type SelectorModel struct {
	entries   []ailog.AssistantEntry
	filtered  []ailog.AssistantEntry
	cursor    int
	selected  *ailog.AssistantEntry
	cancelled bool
	filter    []rune
}

// NewSelectorModel は entries を表示する選択モデルを返す。
func NewSelectorModel(entries []ailog.AssistantEntry) SelectorModel {
	cp := make([]ailog.AssistantEntry, len(entries))
	copy(cp, entries)
	return SelectorModel{
		entries:  entries,
		filtered: cp,
	}
}

func (m SelectorModel) Init() tea.Cmd {
	return nil
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEscape || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q" && len(m.filter) == 0):
			m.cancelled = true
			return m, tea.Quit

		case msg.Type == tea.KeyEnter:
			if len(m.filtered) > 0 {
				e := m.filtered[m.cursor]
				m.selected = &e
			}
			return m, tea.Quit

		case msg.Type == tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case msg.Type == tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}

		case msg.Type == tea.KeyBackspace:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}

		case msg.Type == tea.KeyRunes:
			r := msg.Runes[0]
			if len(m.filter) == 0 && (r == 'j' || r == 'k') {
				if r == 'j' && m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
				if r == 'k' && m.cursor > 0 {
					m.cursor--
				}
			} else {
				m.filter = append(m.filter, msg.Runes...)
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m *SelectorModel) applyFilter() {
	query := string(m.filter)
	if query == "" {
		cp := make([]ailog.AssistantEntry, len(m.entries))
		copy(cp, m.entries)
		m.filtered = cp
	} else {
		source := make([]string, len(m.entries))
		for i, e := range m.entries {
			source[i] = e.Text
		}
		matches := fuzzy.Find(query, source)
		result := make([]ailog.AssistantEntry, 0, len(matches))
		for _, match := range matches {
			result = append(result, m.entries[match.Index])
		}
		m.filtered = result
	}
	m.cursor = 0
}

func (m SelectorModel) View() string {
	if len(m.filtered) == 0 && len(m.filter) == 0 {
		return "履歴が見つかりません。\n"
	}

	var b strings.Builder
	b.WriteString("履歴を選択してください (↑↓/jk: 移動, Enter: 選択, Esc: 中断)\n")

	if len(m.filter) > 0 {
		fmt.Fprintf(&b, "filter: %s\n", string(m.filter))
	}
	b.WriteByte('\n')

	if len(m.filtered) == 0 {
		b.WriteString("  (一致する履歴がありません)\n")
	}
	for i, e := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		preview := truncate(e.Text, previewMaxLen)
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Fprintf(&b, "%s%s  %s\n", cursor, e.Timestamp, preview)
	}

	return b.String()
}

// Cursor は現在のカーソル位置を返す。
func (m SelectorModel) Cursor() int { return m.cursor }

// Selected は選択されたエントリを返す。未選択なら nil。
func (m SelectorModel) Selected() *ailog.AssistantEntry { return m.selected }

// Cancelled はユーザーが中断したかどうかを返す。
func (m SelectorModel) Cancelled() bool { return m.cancelled }

// FilteredEntries は現在のフィルタ条件に一致するエントリを返す。
func (m SelectorModel) FilteredEntries() []ailog.AssistantEntry { return m.filtered }

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
