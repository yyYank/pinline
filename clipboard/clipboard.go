// Package clipboard はクリップボードから AI 回答テキストを取得する処理を提供する。
package clipboard

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultClipboardCommand は macOS でクリップボードの内容を読み取る
// デフォルトコマンド。
var DefaultClipboardCommand = []string{"pbpaste"}

var DefaultWriteCommand = []string{"pbcopy"}

// ReadClipboard は command を実行し、その標準出力をクリップボードの内容
// として返す。テストでは command を偽のスクリプトへ差し替えられる。
func ReadClipboard(command []string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("transport: clipboard command is empty")
	}

	cmd := exec.Command(command[0], command[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read clipboard via %v: %w", command, err)
	}
	return string(out), nil
}

func WriteClipboard(command []string, text string) error {
	if len(command) == 0 {
		return errors.New("clipboard: write command is empty")
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write clipboard via %v: %w", command, err)
	}
	return nil
}
