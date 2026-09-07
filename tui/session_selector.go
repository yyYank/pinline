package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/yyYank/pinline/ailog"
)

// SessionSelectorModel は SessionInfo のリストから1件を選択する TUI モデル。
type SessionSelectorModel struct {
	entries   []ailog.SessionInfo
	filtered  []ailog.SessionInfo
	cursor    int
	selected  *ailog.SessionInfo
	cancelled bool
	filter    []rune
}

func NewSessionSelectorModel(entries []ailog.SessionInfo) SessionSelectorModel {
	cp := make([]ailog.SessionInfo, len(entries))
	copy(cp, entries)
	return SessionSelectorModel{
		entries:  entries,
		filtered: cp,
	}
}

func (m SessionSelectorModel) Init() tea.Cmd {
	return nil
}

func (m SessionSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *SessionSelectorModel) applyFilter() {
	query := string(m.filter)
	if query == "" {
		cp := make([]ailog.SessionInfo, len(m.entries))
		copy(cp, m.entries)
		m.filtered = cp
	} else {
		source := make([]string, len(m.entries))
		for i, e := range m.entries {
			source[i] = e.SessionID + " " + e.GitBranch + " " + e.StartedAt + " " + e.SearchText
		}
		matches := fuzzy.Find(query, source)
		result := make([]ailog.SessionInfo, 0, len(matches))
		for _, match := range matches {
			result = append(result, m.entries[match.Index])
		}
		m.filtered = result
	}
	m.cursor = 0
}

func (m SessionSelectorModel) View() string {
	if len(m.filtered) == 0 && len(m.filter) == 0 {
		return "セッションが見つかりません。\n"
	}

	var b strings.Builder
	b.WriteString("セッションを選択してください (↑↓/jk: 移動, Enter: 選択, Esc: 中断)\n")

	if len(m.filter) > 0 {
		fmt.Fprintf(&b, "filter: %s\n", string(m.filter))
	}
	b.WriteByte('\n')

	if len(m.filtered) == 0 {
		b.WriteString("  (一致するセッションがありません)\n")
	}
	for i, e := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		ts := e.StartedAt
		if len(ts) >= 16 {
			ts = ts[:10] + " " + ts[11:16]
		}
		sid := e.SessionID
		if len(sid) > 8 {
			sid = sid[:8] + "..."
		}
		fmt.Fprintf(&b, "%s%s  %-20s  %s\n", cursor, ts, e.GitBranch, sid)
	}

	return b.String()
}

func (m SessionSelectorModel) Cursor() int                   { return m.cursor }
func (m SessionSelectorModel) Selected() *ailog.SessionInfo  { return m.selected }
func (m SessionSelectorModel) Cancelled() bool               { return m.cancelled }
func (m SessionSelectorModel) FilteredEntries() []ailog.SessionInfo { return m.filtered }
