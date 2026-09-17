package clipboard

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadClipboard は指定したコマンドを実行し、その標準出力を
// クリップボード内容として取得できることを検証する。
// 実際の pbpaste の代わりに t.TempDir() 内へ作った偽コマンドを使う。
func TestReadClipboard(t *testing.T) {
	t.Run("偽コマンドの標準出力をクリップボード内容として返す", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-pbpaste.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'clipboard content'\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake command: %v", err)
		}

		got, err := ReadClipboard([]string{script})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "clipboard content" {
			t.Errorf("ReadClipboard() = %q, want %q", got, "clipboard content")
		}
	})

	t.Run("コマンドが失敗した場合はエラーを返す", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-fail.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake command: %v", err)
		}

		_, err := ReadClipboard([]string{script})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("空のコマンドはエラーを返す", func(t *testing.T) {
		_, err := ReadClipboard(nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("存在しないコマンドはエラーを返す", func(t *testing.T) {
		_, err := ReadClipboard([]string{"pinline-nonexistent-clipboard-cmd-xyz"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestWriteClipboard(t *testing.T) {
	t.Run("テキストをコマンドのstdinに書き込む", func(t *testing.T) {
		dir := t.TempDir()
		outFile := filepath.Join(dir, "clipboard.txt")
		script := filepath.Join(dir, "fake-pbcopy.sh")
		os.WriteFile(script, []byte("#!/bin/sh\ncat > \""+outFile+"\"\n"), 0o755)

		err := WriteClipboard([]string{script}, "copied text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := os.ReadFile(outFile)
		if string(got) != "copied text" {
			t.Errorf("got %q, want %q", string(got), "copied text")
		}
	})

	t.Run("空のコマンドはエラーを返す", func(t *testing.T) {
		err := WriteClipboard(nil, "text")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("コマンドが失敗した場合はエラーを返す", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-fail.sh")
		os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755)

		err := WriteClipboard([]string{script}, "text")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
