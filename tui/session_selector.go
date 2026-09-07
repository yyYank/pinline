package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"

	"github.com/yyYank/pinline/ailog"
)

type sessionMatch struct {
	entry   ailog.SessionInfo
	snippet string
}

// SessionSelectorModel は SessionInfo のリストから1件を選択する TUI モデル。
type SessionSelectorModel struct {
	entries   []ailog.SessionInfo
	filtered  []sessionMatch
	cursor    int
	selected  *ailog.SessionInfo
	cancelled bool
	filter        []rune
	width         int
	height        int
	preview       bool
	previewOffset int
}

func NewSessionSelectorModel(entries []ailog.SessionInfo) SessionSelectorModel {
	matches := make([]sessionMatch, len(entries))
	for i, e := range entries {
		matches[i] = sessionMatch{entry: e}
	}
	return SessionSelectorModel{
		entries:  entries,
		filtered: matches,
	}
}

func (m SessionSelectorModel) Init() tea.Cmd {
	return nil
}

func (m SessionSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				e := m.filtered[m.cursor].entry
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

func (m *SessionSelectorModel) applyFilter() {
	query := string(m.filter)
	if query == "" {
		matches := make([]sessionMatch, len(m.entries))
		for i, e := range m.entries {
			matches[i] = sessionMatch{entry: e}
		}
		m.filtered = matches
	} else {
		source := make([]string, len(m.entries))
		for i, e := range m.entries {
			source[i] = e.SearchText
		}
		fuzzyMatches := fuzzy.Find(query, source)
		result := make([]sessionMatch, 0, len(fuzzyMatches))
		for _, fm := range fuzzyMatches {
			entry := m.entries[fm.Index]
			snippet := extractSnippet(source[fm.Index], fm.MatchedIndexes, 60)
			result = append(result, sessionMatch{entry: entry, snippet: snippet})
		}
		m.filtered = result
	}
	m.cursor = 0
}

func extractSnippet(text string, matchedIndexes []int, maxLen int) string {
	if len(matchedIndexes) == 0 {
		return truncate(text, maxLen)
	}
	runes := []rune(text)
	firstMatch := matchedIndexes[0]

	start := max(firstMatch-20, 0)
	end := min(start+maxLen, len(runes))

	snippet := string(runes[start:end])
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet = snippet + "..."
	}
	return snippet
}

func (m SessionSelectorModel) View() string {
	if len(m.filtered) == 0 && len(m.filter) == 0 {
		return "セッションが見つかりません。\n"
	}

	helpKeys := "↑↓/jk: 移動, C-p: プレビュー, C-d/C-u: スクロール, Enter: 選択, Esc: 中断"
	var header strings.Builder
	fmt.Fprintf(&header, "セッションを選択してください (%s)\n", helpKeys)
	if len(m.filter) > 0 {
		fmt.Fprintf(&header, "filter: %s\n", string(m.filter))
	}
	header.WriteByte('\n')

	var leftLines []string
	if len(m.filtered) == 0 {
		leftLines = append(leftLines, "  (一致するセッションがありません)")
	}
	for i, sm := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		e := sm.entry
		ts := e.StartedAt
		if len(ts) >= 16 {
			ts = ts[:10] + " " + ts[11:16]
		}
		sid := e.SessionID
		if len(sid) > 8 {
			sid = sid[:8] + "..."
		}
		line := fmt.Sprintf("%s%s  %-20s  %s", cursor, ts, e.GitBranch, sid)
		leftLines = append(leftLines, line)
		if sm.snippet != "" {
			leftLines = append(leftLines, fmt.Sprintf("      \"%s\"", sm.snippet))
		}
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
		allPreviewLines = wrapTextByWidth(m.filtered[m.cursor].entry.SearchText, rightWidth)
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

func (m SessionSelectorModel) previewPageSize() int {
	ps := m.height - 3
	if ps < 5 {
		ps = 5
	}
	return ps / 2
}

func wrapTextByWidth(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	paragraphs := strings.Split(text, "\n")
	var lines []string
	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(para)
		for len(runes) > 0 {
			w := 0
			end := 0
			for end < len(runes) {
				rw := runewidth.RuneWidth(runes[end])
				if w+rw > width {
					break
				}
				w += rw
				end++
			}
			if end == 0 {
				end = 1
			}
			lines = append(lines, string(runes[:end]))
			runes = runes[end:]
		}
	}
	return lines
}

func padToWidth(s string, width int) string {
	sw := runewidth.StringWidth(s)
	if sw > width {
		return runewidth.Truncate(s, width, "")
	}
	return s + strings.Repeat(" ", width-sw)
}

func (m SessionSelectorModel) Cursor() int                  { return m.cursor }
func (m SessionSelectorModel) Selected() *ailog.SessionInfo { return m.selected }
func (m SessionSelectorModel) Cancelled() bool              { return m.cancelled }
func (m SessionSelectorModel) Filter() string               { return string(m.filter) }
func (m SessionSelectorModel) FilteredEntries() []ailog.SessionInfo {
	result := make([]ailog.SessionInfo, len(m.filtered))
	for i, sm := range m.filtered {
		result[i] = sm.entry
	}
	return result
}
