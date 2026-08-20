package editor

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestResolveCommand は環境変数の優先順位 (PINLINE_EDITOR > VISUAL > EDITOR > vi) で
// エディタコマンドが解決され、"zed --wait" のような引数付きコマンドが
// スペース区切りで分割されることを検証する。
func TestResolveCommand(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "何も設定されていなければ vi にフォールバックする",
			env:  map[string]string{},
			want: []string{"vi"},
		},
		{
			name: "EDITOR のみ設定されていれば EDITOR を使う",
			env:  map[string]string{"EDITOR": "nvim"},
			want: []string{"nvim"},
		},
		{
			name: "VISUAL が設定されていれば EDITOR より優先する",
			env:  map[string]string{"EDITOR": "nvim", "VISUAL": "code"},
			want: []string{"code"},
		},
		{
			name: "PINLINE_EDITOR が設定されていれば VISUAL/EDITOR より優先する",
			env:  map[string]string{"EDITOR": "nvim", "VISUAL": "code", "PINLINE_EDITOR": "zed --wait"},
			want: []string{"zed", "--wait"},
		},
		{
			name: "引数を含むコマンドはスペースで分割される",
			env:  map[string]string{"EDITOR": "zed --wait"},
			want: []string{"zed", "--wait"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) string {
				return tt.env[key]
			}
			got := ResolveCommand(lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResolveCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOpen は解決済みコマンドでエディタを起動し、終了（保存完了）を待ってから
// 制御を返すこと、および引数付きコマンドで対象パスが末尾に渡ることを検証する。
func TestOpen(t *testing.T) {
	t.Run("エディタの終了を待ちファイルへの書き込みが反映される", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-editor.sh")
		scriptContent := "#!/bin/sh\necho added >> \"$1\"\n"
		if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}

		target := filepath.Join(dir, "target.md")
		if err := os.WriteFile(target, []byte("original\n"), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}

		if err := Open([]string{script}, target, nil, io.Discard, io.Discard); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("failed to read target file: %v", err)
		}
		want := "original\nadded\n"
		if string(got) != want {
			t.Errorf("target file content = %q, want %q", string(got), want)
		}
	})

	t.Run("引数付きコマンドは引数の後に対象パスを付けて実行される", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-editor-flag.sh")
		scriptContent := `#!/bin/sh
if [ "$1" != "--wait" ]; then
  echo "unexpected first arg: $1" 1>&2
  exit 1
fi
echo ok >> "$2"
`
		if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}

		target := filepath.Join(dir, "target.md")
		if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}

		if err := Open([]string{script, "--wait"}, target, nil, io.Discard, io.Discard); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("failed to read target file: %v", err)
		}
		if string(got) != "ok\n" {
			t.Errorf("target file content = %q, want %q", string(got), "ok\n")
		}
	})

	t.Run("空のコマンドはエラーを返す", func(t *testing.T) {
		if err := Open(nil, "somepath", nil, io.Discard, io.Discard); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("存在しないエディタコマンドはエラーを返す", func(t *testing.T) {
		err := Open([]string{"pinline-nonexistent-editor-xyz"}, "somepath", nil, io.Discard, io.Discard)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
