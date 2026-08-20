// Package editor は $EDITOR の解決と起動・終了待ちを担当する。
package editor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// envPriority はエディタコマンドを解決する際に参照する環境変数の優先順位。
var envPriority = []string{"PINLINE_EDITOR", "VISUAL", "EDITOR"}

// defaultCommand はどの環境変数も設定されていない場合のフォールバック。
var defaultCommand = []string{"vi"}

// ResolveCommand は lookup を使って優先順位
// (PINLINE_EDITOR > VISUAL > EDITOR > vi) でエディタコマンドを解決する。
//
// "zed --wait" のように引数を含むコマンドはスペースで分割して返す。
func ResolveCommand(lookup func(key string) string) []string {
	for _, key := range envPriority {
		v := lookup(key)
		if v == "" {
			continue
		}
		fields := strings.Fields(v)
		if len(fields) > 0 {
			return fields
		}
	}
	return append([]string{}, defaultCommand...)
}

// Open は command で指定されたエディタを path を引数の末尾に付けて起動し、
// 終了（保存完了）まで待機する。
//
// stdin/stdout/stderr はエディタプロセスへそのまま接続され、対話的なエディタ
// （Vim など）でもターミナルを介して操作できる。
func Open(command []string, path string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(command) == 0 {
		return errors.New("editor: command is empty")
	}

	args := make([]string, 0, len(command)-1+1)
	args = append(args, command[1:]...)
	args = append(args, path)

	cmd := exec.Command(command[0], args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

// DefaultTTYPath は DefaultOpenTTY が開く制御端末デバイスのパス。
var DefaultTTYPath = "/dev/tty"

// DefaultIsTerminal は f が端末（tty）に接続されているかどうかを判定する。
// f が nil、または Stat に失敗した場合は false を返す。
func DefaultIsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// DefaultOpenTTY は DefaultTTYPath（既定では /dev/tty）を読み書き両用で開く。
func DefaultOpenTTY() (*os.File, error) {
	return os.OpenFile(DefaultTTYPath, os.O_RDWR, 0)
}

// ttyUnavailableWarning は制御端末が開けず、tmux popup も使えない場合に
// stderr へ出す警告メッセージ。GUI エディタ（zed --wait 等）は TTY を必要と
// しないため、ここではエラーにせず警告のみ出してエディタの起動を続行する。
const ttyUnavailableWarning = `警告: 制御端末(TTY)が無いためターミナルエディタは動作しません。GUIエディタの利用を推奨します（例: EDITOR="zed --wait"）。`

// TmuxEnvVar は tmux セッション内で実行されているかどうかを判定するために
// 参照する環境変数名。
const TmuxEnvVar = "TMUX"

// tmuxPopupInfoMessage は tmux popup 経由でエディタを起動する際に
// stderr へ出す案内メッセージ。
const tmuxPopupInfoMessage = "tmux popupでエディタを起動します。"

// InteractiveOptions は OpenInteractive が端末の有無に応じて依存を切り替える
// ために使う注入可能な依存の集合。テストでは各フィールドへ偽の実装を渡す。
type InteractiveOptions struct {
	// IsTerminal は f が端末（tty）に接続されているかどうかを判定する。
	IsTerminal func(f *os.File) bool
	// OpenTTY は制御端末（既定では /dev/tty）を読み書き両用で開く。
	OpenTTY func() (*os.File, error)

	// Getenv は環境変数を参照する（TMUX の判定に使う）。
	Getenv func(key string) string
	// TmuxAvailable は tmux コマンドが利用可能かどうかを判定する。
	TmuxAvailable func() bool
	// RunTmuxPopup は command（path を末尾に付与）を tmux display-popup で
	// 起動し、popup が閉じるまで待機する。
	RunTmuxPopup func(command []string, path string, stdout, stderr *os.File) error
}

// DefaultInteractiveOptions は本番用の InteractiveOptions を返す。
func DefaultInteractiveOptions() InteractiveOptions {
	return InteractiveOptions{
		IsTerminal:    DefaultIsTerminal,
		OpenTTY:       DefaultOpenTTY,
		Getenv:        os.Getenv,
		TmuxAvailable: DefaultTmuxAvailable,
		RunTmuxPopup:  DefaultRunTmuxPopup,
	}
}

// DefaultTmuxAvailable は tmux コマンドが PATH 上で実行可能かどうかを返す。
func DefaultTmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// BuildTmuxPopupArgs は `tmux display-popup` に渡す引数列を組み立てる。
// -E により、command（エディタ）が終了すると popup も自動的に閉じ、
// tmux プロセス自体もそのタイミングで終了する（＝終了待ちができる）。
// command は "zed --wait" のように複数トークンでもよく、そのまま
// `--` の後に展開し、最後に path を付与する。
func BuildTmuxPopupArgs(command []string, path string) []string {
	args := []string{"display-popup", "-E", "-w", "90%", "-h", "90%", "--"}
	args = append(args, command...)
	args = append(args, path)
	return args
}

// DefaultRunTmuxPopup は tmux コマンドを実際に起動し、popup が閉じる
// （＝command が終了する）まで待機する。
func DefaultRunTmuxPopup(command []string, path string, stdout, stderr *os.File) error {
	cmd := exec.Command("tmux", BuildTmuxPopupArgs(command, path)...)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// OpenInteractive は Open と同様にエディタを起動するが、pinline 自身の
// stdin/stdout/stderr がパイプ等で端末に接続されていない場合でも、
// 対話的エディタが起動できるようフォールバックを行う。
//
// フォールバックの優先順位:
//  1. stdin/stdout/stderr がすべて端末なら、従来通りそれらをそのまま使う。
//  2. そうでない場合、git が $EDITOR を起動する際と同様に制御端末
//     （既定では opts.OpenTTY が返す /dev/tty）へ接続し直して起動する。
//  3. /dev/tty も開けない環境で、tmux セッション内
//     （opts.Getenv(TmuxEnvVar) が空でない）かつ tmux コマンドが利用可能
//     （opts.TmuxAvailable()）な場合は、`tmux display-popup` でエディタを
//     popup として起動し、popup が閉じる（＝コマンド終了）まで待つ。
//  4. どちらも使えない場合は、zed --wait のような TTY 不要の GUI エディタは
//     問題なく動作できるためエラーにはせず、stderr へ警告を1行出し、
//     Stdin は割り当てず（nil）、Stdout/Stderr は stderr へ接続して
//     エディタを起動する（pinline 自身の stdout は汚さない）。
//
// いずれの場合も、エディタ（またはtmuxコマンド）自体が失敗した場合は
// その失敗をそのままエラーとして返す。
func OpenInteractive(
	command []string,
	path string,
	stdin, stdout, stderr *os.File,
	opts InteractiveOptions,
) error {
	allTerminal := opts.IsTerminal(stdin) && opts.IsTerminal(stdout) && opts.IsTerminal(stderr)
	if allTerminal {
		return Open(command, path, stdin, stdout, stderr)
	}

	if tty, err := opts.OpenTTY(); err == nil {
		defer tty.Close()
		return Open(command, path, tty, tty, tty)
	}

	if opts.Getenv != nil && opts.TmuxAvailable != nil && opts.RunTmuxPopup != nil &&
		opts.Getenv(TmuxEnvVar) != "" && opts.TmuxAvailable() {
		fmt.Fprintln(stderr, tmuxPopupInfoMessage)
		return opts.RunTmuxPopup(command, path, stderr, stderr)
	}

	fmt.Fprintln(stderr, ttyUnavailableWarning)
	return Open(command, path, nil, stderr, stderr)
}
