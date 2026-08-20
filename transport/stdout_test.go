package transport

import (
	"bytes"
	"testing"
)

// TestWriteStdout は io.Writer へ文字列をそのまま書き出すことを検証する。
func TestWriteStdout(t *testing.T) {
	t.Run("文字列をそのまま書き込む", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WriteStdout(&buf, "> hello\n> world"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "> hello\n> world"
		if buf.String() != want {
			t.Errorf("WriteStdout() wrote %q, want %q", buf.String(), want)
		}
	})

	t.Run("空文字を書き込んでもエラーにならない", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WriteStdout(&buf, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.String() != "" {
			t.Errorf("WriteStdout() wrote %q, want empty", buf.String())
		}
	})
}
