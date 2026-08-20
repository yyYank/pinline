package editor

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// TestOpenInteractive は、pinline 自身の stdin/stdout/stderr が
// パイプ（非端末）の場合でも、git が $EDITOR を起動する際と同様に
// /dev/tty へ接続してエディタを起動できることを検証する。
// 実際の /dev/tty の代わりに t.TempDir() 内の偽ファイルと
// 偽の isTerminal 判定関数を注入してテストする。
func TestOpenInteractive(t *testing.T) {
	t.Run("stdin_stdout_stderrがすべて端末なら従来通りそのまま使いttyは開かない", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-editor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf edited >> \"$1\"\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}
		target := filepath.Join(dir, "target.md")
		if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("failed to create pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()

		alwaysTerminal := func(*os.File) bool { return true }
		ttyOpened := false
		openTTY := func() (*os.File, error) {
			ttyOpened = true
			return nil, errors.New("should not be called")
		}

		opts := InteractiveOptions{IsTerminal: alwaysTerminal, OpenTTY: openTTY}
		err = OpenInteractive([]string{script}, target, r, w, w, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ttyOpened {
			t.Error("openTTY was called, want it not to be called when all streams are terminals")
		}

		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("failed to read target file: %v", readErr)
		}
		if string(got) != "edited" {
			t.Errorf("target file content = %q, want %q", string(got), "edited")
		}
	})

	t.Run("いずれかが端末でなければdev_ttyを開いてエディタのstdin_stdout_stderrへ接続する", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-editor-cat.sh")
		// 標準入力の内容をそのまま対象ファイルへ書き出すことで、
		// エディタに実際に渡された stdin が何かを検証できるようにする。
		if err := os.WriteFile(script, []byte("#!/bin/sh\ncat > \"$1\"\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}
		target := filepath.Join(dir, "target.md")

		fakeTTYPath := filepath.Join(dir, "fake-tty")
		if err := os.WriteFile(fakeTTYPath, []byte("FAKE-TTY-DATA"), 0o644); err != nil {
			t.Fatalf("failed to write fake tty file: %v", err)
		}

		neverTerminal := func(*os.File) bool { return false }
		openTTY := func() (*os.File, error) {
			return os.OpenFile(fakeTTYPath, os.O_RDWR, 0)
		}

		opts := InteractiveOptions{IsTerminal: neverTerminal, OpenTTY: openTTY}
		err := OpenInteractive([]string{script}, target, nil, nil, nil, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("failed to read target file: %v", readErr)
		}
		if string(got) != "FAKE-TTY-DATA" {
			t.Errorf("target file content = %q, want %q (editor did not receive tty stdin)", string(got), "FAKE-TTY-DATA")
		}
	})

	t.Run("ttyが開けない場合はエラーにせず警告を出しStdinなしでエディタを起動する", func(t *testing.T) {
		// TTY が無い環境（sandbox / CI 等）でも zed --wait のような
		// GUI エディタは TTY を必要としないため、tty が開けないことを
		// 理由にエラーで打ち切ってはいけない。警告を出した上でエディタは
		// 起動し、Stdin は割り当てず（nil）、Stdout/Stderr は
		// pinline 自身の stderr 相当へ接続する（pinline の stdout は汚さない）。
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-gui-editor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf gui-edited >> \"$1\"\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}
		target := filepath.Join(dir, "target.md")
		if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}

		stderrR, stderrW, err := os.Pipe()
		if err != nil {
			t.Fatalf("failed to create pipe: %v", err)
		}

		neverTerminal := func(*os.File) bool { return false }
		openTTY := func() (*os.File, error) {
			return nil, errors.New("no such device")
		}

		// tmux は使えない前提（Getenv/TmuxAvailable未設定）にして、
		// 従来通りの警告+Stdinなし起動フォールバックを検証する。
		opts := InteractiveOptions{IsTerminal: neverTerminal, OpenTTY: openTTY}
		runErr := OpenInteractive([]string{script}, target, nil, nil, stderrW, opts)
		stderrW.Close()

		if runErr != nil {
			t.Fatalf("unexpected error: %v", runErr)
		}

		warned, readErr := io.ReadAll(stderrR)
		if readErr != nil {
			t.Fatalf("failed to read stderr pipe: %v", readErr)
		}
		if !strings.Contains(string(warned), "GUI") {
			t.Errorf("stderr = %q, want it to contain a warning recommending a GUI editor", string(warned))
		}

		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("failed to read target file: %v", readErr)
		}
		if string(got) != "gui-edited" {
			t.Errorf("target file content = %q, want %q", string(got), "gui-edited")
		}
	})

	t.Run("ttyが開けなくてもエディタコマンド自体が失敗すればそのエラーを返す", func(t *testing.T) {
		// vi 等のターミナルエディタは Stdin が無いため失敗し得る。
		// その場合は tty 不在を理由に握りつぶさず、従来通りエラーを返す。
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-failing-editor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}

		neverTerminal := func(*os.File) bool { return false }
		openTTY := func() (*os.File, error) {
			return nil, errors.New("no such device")
		}

		opts := InteractiveOptions{IsTerminal: neverTerminal, OpenTTY: openTTY}
		err := OpenInteractive([]string{script}, filepath.Join(dir, "target.md"), nil, nil, nil, opts)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("ttyもtmuxも無い場合(tmux未使用時)は従来通りStdinなし起動になる", func(t *testing.T) {
		// TMUX 環境変数が空、または tmux コマンドが使えない場合は
		// tmux popup を試みず、従来のフォールバックへ進むことを確認する。
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-editor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf edited >> \"$1\"\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}
		target := filepath.Join(dir, "target.md")
		if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}

		neverTerminal := func(*os.File) bool { return false }
		openTTY := func() (*os.File, error) { return nil, errors.New("no such device") }
		tmuxPopupCalled := false
		runTmuxPopup := func([]string, string, *os.File, *os.File) error {
			tmuxPopupCalled = true
			return nil
		}

		t.Run("TMUX環境変数が空の場合", func(t *testing.T) {
			tmuxPopupCalled = false
			getenv := func(string) string { return "" } // TMUX未設定
			tmuxAvailable := func() bool { return true }

			opts := InteractiveOptions{
				IsTerminal:    neverTerminal,
				OpenTTY:       openTTY,
				Getenv:        getenv,
				TmuxAvailable: tmuxAvailable,
				RunTmuxPopup:  runTmuxPopup,
			}
			if err := OpenInteractive([]string{script}, target, nil, nil, nil, opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tmuxPopupCalled {
				t.Error("RunTmuxPopup was called, want it not to be called when TMUX is unset")
			}
		})

		t.Run("tmuxコマンドが利用不可の場合", func(t *testing.T) {
			tmuxPopupCalled = false
			getenv := func(key string) string {
				if key == TmuxEnvVar {
					return "/tmp/tmux-1000/default,1234,0"
				}
				return ""
			}
			tmuxAvailable := func() bool { return false } // tmuxコマンドが見つからない

			opts := InteractiveOptions{
				IsTerminal:    neverTerminal,
				OpenTTY:       openTTY,
				Getenv:        getenv,
				TmuxAvailable: tmuxAvailable,
				RunTmuxPopup:  runTmuxPopup,
			}
			if err := OpenInteractive([]string{script}, target, nil, nil, nil, opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tmuxPopupCalled {
				t.Error("RunTmuxPopup was called, want it not to be called when tmux is unavailable")
			}
		})
	})

	t.Run("ttyが開けずtmuxが使える場合はtmux_display-popupでエディタを起動する", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fake-editor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf popup-edited >> \"$1\"\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake editor script: %v", err)
		}
		target := filepath.Join(dir, "target.md")
		if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write target file: %v", err)
		}

		stderrR, stderrW, err := os.Pipe()
		if err != nil {
			t.Fatalf("failed to create pipe: %v", err)
		}

		neverTerminal := func(*os.File) bool { return false }
		openTTY := func() (*os.File, error) { return nil, errors.New("no such device") }
		getenv := func(key string) string {
			if key == TmuxEnvVar {
				return "/tmp/tmux-1000/default,1234,0"
			}
			return ""
		}
		tmuxAvailable := func() bool { return true }

		var gotCommand []string
		var gotPath string
		runTmuxPopup := func(command []string, path string, stdout, stderr *os.File) error {
			gotCommand = command
			gotPath = path
			// 実際に対象ファイルへ書き込むことで、popup 経由でも
			// エディタの実行結果が反映されることを検証する。
			f, openErr := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
			if openErr != nil {
				return openErr
			}
			defer f.Close()
			if _, writeErr := f.WriteString("popup-edited"); writeErr != nil {
				return writeErr
			}
			return nil
		}

		opts := InteractiveOptions{
			IsTerminal:    neverTerminal,
			OpenTTY:       openTTY,
			Getenv:        getenv,
			TmuxAvailable: tmuxAvailable,
			RunTmuxPopup:  runTmuxPopup,
		}

		runErr := OpenInteractive([]string{"zed", "--wait"}, target, nil, nil, stderrW, opts)
		stderrW.Close()

		if runErr != nil {
			t.Fatalf("unexpected error: %v", runErr)
		}

		if !reflect.DeepEqual(gotCommand, []string{"zed", "--wait"}) {
			t.Errorf("RunTmuxPopup command = %v, want %v (引数付きエディタコマンドがそのまま渡される)", gotCommand, []string{"zed", "--wait"})
		}
		if gotPath != target {
			t.Errorf("RunTmuxPopup path = %q, want %q", gotPath, target)
		}

		warned, readErr := io.ReadAll(stderrR)
		if readErr != nil {
			t.Fatalf("failed to read stderr pipe: %v", readErr)
		}
		if !strings.Contains(string(warned), "tmux") {
			t.Errorf("stderr = %q, want it to mention tmux popup", string(warned))
		}

		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("failed to read target file: %v", readErr)
		}
		if string(got) != "popup-edited" {
			t.Errorf("target file content = %q, want %q", string(got), "popup-edited")
		}
	})

	t.Run("tmux_popup起動自体が失敗すればそのエラーを返す", func(t *testing.T) {
		neverTerminal := func(*os.File) bool { return false }
		openTTY := func() (*os.File, error) { return nil, errors.New("no such device") }
		getenv := func(key string) string {
			if key == TmuxEnvVar {
				return "/tmp/tmux-1000/default,1234,0"
			}
			return ""
		}
		tmuxAvailable := func() bool { return true }
		runTmuxPopup := func([]string, string, *os.File, *os.File) error {
			return errors.New("tmux popup failed")
		}

		opts := InteractiveOptions{
			IsTerminal:    neverTerminal,
			OpenTTY:       openTTY,
			Getenv:        getenv,
			TmuxAvailable: tmuxAvailable,
			RunTmuxPopup:  runTmuxPopup,
		}

		err := OpenInteractive([]string{"irrelevant"}, "irrelevant", nil, nil, nil, opts)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestBuildTmuxPopupArgs は tmux display-popup へ渡す引数列を検証する。
func TestBuildTmuxPopupArgs(t *testing.T) {
	t.Run("引数なしエディタコマンドの場合", func(t *testing.T) {
		got := BuildTmuxPopupArgs([]string{"vi"}, "/tmp/target.md")
		want := []string{"display-popup", "-E", "-w", "90%", "-h", "90%", "--", "vi", "/tmp/target.md"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("BuildTmuxPopupArgs() = %v, want %v", got, want)
		}
	})

	t.Run("引数付きエディタコマンド(zed --wait)の場合", func(t *testing.T) {
		got := BuildTmuxPopupArgs([]string{"zed", "--wait"}, "/tmp/target.md")
		want := []string{"display-popup", "-E", "-w", "90%", "-h", "90%", "--", "zed", "--wait", "/tmp/target.md"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("BuildTmuxPopupArgs() = %v, want %v", got, want)
		}
	})
}

// TestDefaultTmuxAvailable は DefaultTmuxAvailable が実行時エラーなく
// bool を返すことのみを確認する（実環境のtmux有無に依存するため、
// 具体的な真偽値までは固定しない）。
func TestDefaultTmuxAvailable(t *testing.T) {
	_ = DefaultTmuxAvailable()
}

// TestDefaultRunTmuxPopup は実 tmux の代わりに PATH 上へ配置した
// 偽 tmux シェルスクリプトを使い、DefaultRunTmuxPopup が
// BuildTmuxPopupArgs 通りの引数で tmux を起動し、その終了を待って
// から結果（エラーの有無）を返すことを検証する。
func TestDefaultRunTmuxPopup(t *testing.T) {
	t.Run("BuildTmuxPopupArgs通りの引数でtmuxを起動しその終了を待つ", func(t *testing.T) {
		dir := t.TempDir()
		fakeTmux := filepath.Join(dir, "tmux")
		argsFile := filepath.Join(dir, "args.txt")
		script := "#!/bin/sh\necho \"$@\" > \"" + argsFile + "\"\n"
		if err := os.WriteFile(fakeTmux, []byte(script), 0o755); err != nil {
			t.Fatalf("failed to write fake tmux: %v", err)
		}

		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		target := filepath.Join(dir, "target.md")
		if err := DefaultRunTmuxPopup([]string{"zed", "--wait"}, target, os.Stdout, os.Stderr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, readErr := os.ReadFile(argsFile)
		if readErr != nil {
			t.Fatalf("failed to read recorded args (tmux が起動・完了していない可能性): %v", readErr)
		}
		want := strings.Join(BuildTmuxPopupArgs([]string{"zed", "--wait"}, target), " ") + "\n"
		if string(got) != want {
			t.Errorf("tmux was invoked with args = %q, want %q", string(got), want)
		}
	})

	t.Run("tmuxコマンドが失敗した場合はそのエラーを返す", func(t *testing.T) {
		dir := t.TempDir()
		fakeTmux := filepath.Join(dir, "tmux")
		if err := os.WriteFile(fakeTmux, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatalf("failed to write fake tmux: %v", err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		err := DefaultRunTmuxPopup([]string{"vi"}, filepath.Join(dir, "target.md"), os.Stdout, os.Stderr)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestDefaultIsTerminal は DefaultIsTerminal が nil や通常ファイルに対して
// false を返すことを検証する（実端末を用いた true 判定はCI上で検証できないため対象外）。
func TestDefaultIsTerminal(t *testing.T) {
	t.Run("nilはfalseになる", func(t *testing.T) {
		if DefaultIsTerminal(nil) {
			t.Error("DefaultIsTerminal(nil) = true, want false")
		}
	})

	t.Run("通常ファイルはchar deviceではないのでfalseになる", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "not-a-tty")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer f.Close()

		if DefaultIsTerminal(f) {
			t.Error("DefaultIsTerminal(regular file) = true, want false")
		}
	})
}
