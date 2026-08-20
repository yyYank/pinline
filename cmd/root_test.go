package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yyYank/pinline/ailog"
	"github.com/yyYank/pinline/editor"
)

// fakeOpenEditor は CI 等の実 tty が無い環境でも cmd パッケージの
// テストが動くよう、tty フォールバックを持たない editor.Open を
// そのまま使う openEditor 関数（テスト用）。tty 経由の起動確認は
// editor パッケージ側の TestOpenInteractive が担う。
func fakeOpenEditor(command []string, path string) error {
	return editor.Open(command, path, nil, io.Discard, io.Discard)
}

// TestNewRootCmd は pinline / pi のどちらの名前でも起動できるように
// Use が "pinline"、Aliases に "pi" が含まれ、--clipboard フラグが
// 登録されていることを検証する。
func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd()

	if cmd.Use != "pinline" {
		t.Errorf("Use = %q, want %q", cmd.Use, "pinline")
	}

	found := false
	for _, alias := range cmd.Aliases {
		if alias == "pi" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain %q", cmd.Aliases, "pi")
	}

	if cmd.Flags().Lookup("clipboard") == nil {
		t.Error("--clipboard flag is not registered")
	}
}

// fakeAILog はテスト用の ailog.Source 偽実装。
type fakeAILog struct {
	text string
	err  error
}

func (f fakeAILog) LastAnswer() (string, error) {
	return f.text, f.err
}

var _ ailog.Source = fakeAILog{}

// stubClipboardReader はテスト用にクリップボード読み取りを差し替えるスタブを作る。
func stubClipboardReader(text string, err error) func([]string) (string, error) {
	return func([]string) (string, error) {
		return text, err
	}
}

// TestResolveAnswer は SPEC §21 に準拠した入力元の優先順位
// (stdin pipe > --clipboard > ailog.Source > エラー) を検証する。
func TestResolveAnswer(t *testing.T) {
	t.Run("stdinがpipeなら他の指定より優先してstdinを使う", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:   true,
			ClipboardFlag: true, // 同時指定でもstdinが優先される
			Stdin:         strings.NewReader("stdin answer"),
			ReadClipboard: stubClipboardReader("should not be used", nil),
			AILog:         fakeAILog{text: "should not be used"},
		}

		got, err := resolveAnswer(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "stdin answer" {
			t.Errorf("resolveAnswer() = %q, want %q", got, "stdin answer")
		}
	})

	t.Run("clipboardフラグがあればクリップボードから取得する", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:      false,
			ClipboardFlag:    true,
			ReadClipboard:    stubClipboardReader("clipboard answer", nil),
			ClipboardCommand: []string{"fake-pbpaste"},
			AILog:            fakeAILog{text: "should not be used"},
		}

		got, err := resolveAnswer(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "clipboard answer" {
			t.Errorf("resolveAnswer() = %q, want %q", got, "clipboard answer")
		}
	})

	t.Run("clipboard取得コマンドが失敗した場合はエラーを返す", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: true,
			ReadClipboard: stubClipboardReader("", errors.New("pbpaste failed")),
		}

		_, err := resolveAnswer(src)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("clipboardの内容が空白のみの場合はエラーを返す", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: true,
			ReadClipboard: stubClipboardReader("   \n", nil),
		}

		_, err := resolveAnswer(src)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("stdinもclipboardも無い場合はAILog(Source)から取得する", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: false,
			AILog:         fakeAILog{text: "ログ由来の回答"},
		}

		got, err := resolveAnswer(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ログ由来の回答" {
			t.Errorf("resolveAnswer() = %q, want %q", got, "ログ由来の回答")
		}
	})

	t.Run("AILogがエラーを返す場合はエラーを返す", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: false,
			AILog:         fakeAILog{err: errors.New("log not found")},
		}

		_, err := resolveAnswer(src)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("AILogがnilの場合はエラーを返す", func(t *testing.T) {
		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: false,
			AILog:         nil,
		}

		_, err := resolveAnswer(src)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestIsPipe は os.Stdin 相当のファイルが端末（tty）ではないことを
// ModeCharDevice の有無で判定できることを検証する。
func TestIsPipe(t *testing.T) {
	t.Run("os.Pipeの読み込み側はpipe扱いになる", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("failed to create pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()

		if !isPipe(r) {
			t.Error("isPipe(pipe reader) = false, want true")
		}
	})

	t.Run("通常ファイルもchar deviceではないためpipe扱いになる", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "not-a-tty")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer f.Close()

		if !isPipe(f) {
			t.Error("isPipe(regular file) = false, want true")
		}
	})
}

// TestRun は MVP の中心フローを、偽エディタ・偽クリップボードコマンド・
// 偽 ailog.Source を使って統合的に検証する。
func TestRun(t *testing.T) {
	writeFakeEditor := func(t *testing.T, dir, appended string) string {
		t.Helper()
		script := filepath.Join(dir, "fake-editor.sh")
		content := "#!/bin/sh\nprintf '" + appended + "' >> \"$1\"\n"
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}
		return script
	}

	t.Run("stdinがpipeの場合は従来通りstdinをblockquote化して返す", func(t *testing.T) {
		dir := t.TempDir()
		script := writeFakeEditor(t, dir, "\\nfollow up")

		getenv := func(key string) string {
			if key == "PINLINE_EDITOR" {
				return script
			}
			return ""
		}

		src := inputSource{
			StdinIsPipe: true,
			Stdin:       strings.NewReader("hello\nworld\n"),
		}
		var stdout, stderr bytes.Buffer

		if err := run(src, &stdout, &stderr, getenv, fakeOpenEditor); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := "> hello\n> world\nfollow up"
		if stdout.String() != want {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	})

	t.Run("clipboardフラグ指定時はクリップボード内容をblockquote化して返す", func(t *testing.T) {
		dir := t.TempDir()
		script := writeFakeEditor(t, dir, "")

		getenv := func(key string) string {
			if key == "PINLINE_EDITOR" {
				return script
			}
			return ""
		}

		src := inputSource{
			StdinIsPipe:      false,
			ClipboardFlag:    true,
			ReadClipboard:    stubClipboardReader("clip text", nil),
			ClipboardCommand: []string{"fake-pbpaste"},
		}
		var stdout, stderr bytes.Buffer

		if err := run(src, &stdout, &stderr, getenv, fakeOpenEditor); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if stdout.String() != "> clip text" {
			t.Errorf("stdout = %q, want %q", stdout.String(), "> clip text")
		}
	})

	t.Run("stdinもclipboardも無い場合はAILogから取得してblockquote化する", func(t *testing.T) {
		dir := t.TempDir()
		script := writeFakeEditor(t, dir, "")

		getenv := func(key string) string {
			if key == "PINLINE_EDITOR" {
				return script
			}
			return ""
		}

		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: false,
			AILog:         fakeAILog{text: "セッションログの回答"},
		}
		var stdout, stderr bytes.Buffer

		if err := run(src, &stdout, &stderr, getenv, fakeOpenEditor); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if stdout.String() != "> セッションログの回答" {
			t.Errorf("stdout = %q, want %q", stdout.String(), "> セッションログの回答")
		}
	})

	t.Run("どの入力元からも取得できない場合はエラーを返しstderrに使い方を出しエディタを起動しない", func(t *testing.T) {
		dir := t.TempDir()
		marker := filepath.Join(dir, "editor-invoked")
		script := filepath.Join(dir, "fake-editor-marker.sh")
		// もしエディタが呼ばれてしまったらマーカーファイルを作る。
		scriptContent := "#!/bin/sh\ntouch \"" + marker + "\"\n"
		if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}

		getenv := func(key string) string {
			if key == "PINLINE_EDITOR" {
				return script
			}
			return ""
		}

		src := inputSource{
			StdinIsPipe:   false,
			ClipboardFlag: false,
			AILog:         fakeAILog{err: errors.New("no log found")},
		}
		var stdout, stderr bytes.Buffer

		err := run(src, &stdout, &stderr, getenv, fakeOpenEditor)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if stdout.String() != "" {
			t.Errorf("stdout = %q, want empty", stdout.String())
		}
		if stderr.Len() == 0 {
			t.Error("stderr is empty, want usage/error message")
		}
		if _, statErr := os.Stat(marker); statErr == nil {
			t.Error("editor was invoked, want it not to be invoked when no answer source is available")
		}
	})
}
