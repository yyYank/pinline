// Command pinline / pi は AI CLI とテキストエディタをつなぐ
// ファイルベースの会話プロトコル CLI のエントリポイント。
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/yyYank/pinline/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		if errors.Is(err, cmd.ErrCancelled) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
