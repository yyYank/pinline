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
	entries       []ailog.AssistantEntry
	filtered      []ailog.AssistantEntry
	cursor        int
	selected      *ailog.AssistantEntry
	cancelled     bool
	filter        []rune
	width         int
	height        int
	preview       bool
	previewOffset int
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlP:
			m.preview = !m.preview
			m.previewOffset = 0
			return m, nil

		case msg.Type == tea.KeyCtrlD && m.preview:
			m.previewOffset += m.previewPageSize()
			return m, nil

		case msg.Type == tea.KeyCtrlU && m.preview:
			m.previewOffset -= m.previewPageSize()
			if m.previewOffset < 0 {
				m.previewOffset = 0
			}
			return m, nil

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
				m.previewOffset = 0
			}

		case msg.Type == tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
				m.previewOffset = 0
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
					m.previewOffset = 0
				}
				if r == 'k' && m.cursor > 0 {
					m.cursor--
					m.previewOffset = 0
				}
			} else {
				m.filter = append(m.filter, msg.Runes...)
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m SelectorModel) previewPageSize() int {
	ps := m.height - 3
	if ps < 5 {
		ps = 5
	}
	return ps / 2
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

	helpKeys := "↑↓/jk: 移動, C-p: プレビュー, C-d/C-u: スクロール, Enter: 選択, Esc: 中断"
	var header strings.Builder
	fmt.Fprintf(&header, "履歴を選択してください (%s)\n", helpKeys)
	if len(m.filter) > 0 {
		fmt.Fprintf(&header, "filter: %s\n", string(m.filter))
	}
	header.WriteByte('\n')

	var leftLines []string
	if len(m.filtered) == 0 {
		leftLines = append(leftLines, "  (一致する履歴がありません)")
	}
	for i, e := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		role := ""
		if e.Role != "" {
			role = "[" + e.Role + "] "
		}
		preview := truncate(e.Text, previewMaxLen)
		preview = strings.ReplaceAll(preview, "\n", " ")
		leftLines = append(leftLines, fmt.Sprintf("%s%s  %s%s", cursor, e.Timestamp, role, preview))
	}

	if !m.preview || m.width < 60 {
		var b strings.Builder
		b.WriteString(header.String())
		for _, l := range leftLines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		return b.String()
	}

	leftWidth := m.width/2 - 1
	rightWidth := m.width - leftWidth - 1

	var allPreviewLines []string
	if m.cursor < len(m.filtered) {
		allPreviewLines = wrapTextByWidth(m.filtered[m.cursor].Text, rightWidth)
	}

	visibleRows := m.height - 3
	if visibleRows < 1 {
		visibleRows = len(leftLines)
	}

	offset := m.previewOffset
	if offset > len(allPreviewLines) {
		offset = len(allPreviewLines)
	}
	previewLines := allPreviewLines[offset:]

	maxRows := max(len(leftLines), min(len(previewLines), visibleRows))

	var b strings.Builder
	b.WriteString(header.String())
	for row := 0; row < maxRows; row++ {
		left := ""
		if row < len(leftLines) {
			left = leftLines[row]
		}
		left = padToWidth(left, leftWidth)

		right := ""
		if row < len(previewLines) {
			right = previewLines[row]
		}
		right = padToWidth(right, rightWidth)

		fmt.Fprintf(&b, "%s│%s\n", left, right)
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
