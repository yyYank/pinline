// Package editor は $EDITOR の解決と起動・終了待ちを担当する。
package editor

import (
	"errors"
	"io"
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
