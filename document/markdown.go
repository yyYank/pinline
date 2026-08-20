// Package document は AI 回答テキストを Markdown blockquote へ変換する処理を提供する。
package document

import "strings"

// ToBlockquote は入力テキストを Markdown の blockquote 形式へ変換する。
//
// 変換ルール（SPEC.md §26）:
//   - 各行の先頭に "> " を付与する
//   - 空行は "> "（末尾スペースなし）の ">" のみにする
//   - 入力の末尾改行は、余分な空の引用行を生まない
//   - 空入力は空文字を返す
func ToBlockquote(s string) string {
	if s == "" {
		return ""
	}

	body := s
	if strings.HasSuffix(body, "\n") {
		body = body[:len(body)-1]
	}

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + line
		}
	}

	return strings.Join(lines, "\n")
}
