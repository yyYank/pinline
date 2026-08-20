package transport

import (
	"errors"
	"strings"
	"testing"
)

// TestReadStdin は io.Reader から全内容を文字列として読み込めることを検証する。
func TestReadStdin(t *testing.T) {
	t.Run("通常の入力をそのまま読み込む", func(t *testing.T) {
		r := strings.NewReader("hello\nworld\n")
		got, err := ReadStdin(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "hello\nworld\n"
		if got != want {
			t.Errorf("ReadStdin() = %q, want %q", got, want)
		}
	})

	t.Run("空入力は空文字になる", func(t *testing.T) {
		r := strings.NewReader("")
		got, err := ReadStdin(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("ReadStdin() = %q, want empty string", got)
		}
	})

	t.Run("読み込みエラーはそのまま返す", func(t *testing.T) {
		r := errReader{}
		_, err := ReadStdin(r)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// errReader は常にエラーを返す io.Reader のスタブ。
type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("boom")
}
