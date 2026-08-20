package transport

import "io"

// WriteStdout は s を w へそのまま書き出す。UI メッセージは含めず、
// 編集済み Markdown 本文のみを機械可読な形で出力する用途に使う。
func WriteStdout(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
