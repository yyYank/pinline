// Package cmd は pinline / pi CLI のエントリポイントを提供する。
// ロジックは document / editor / transport / ailog / clipboard パッケージへ
// 委譲し、ここでは MVP のフローを薄く配線するだけに留める。
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yyYank/pinline/ailog"
	"github.com/yyYank/pinline/clipboard"
	"github.com/yyYank/pinline/document"
	"github.com/yyYank/pinline/editor"
	"github.com/yyYank/pinline/transport"
)

const usageMessage = `使い方:
  cat answer.txt | pinline   # stdin から AI 回答を読み込む
  pinline --clipboard        # クリップボードから読み込む
  pinline                    # Claude Code の直前セッションログから読み込む`

// errNoAnswerSource はどの入力元からも AI 回答を取得できなかったことを表す。
var errNoAnswerSource = errors.New("no AI answer source available")

// inputSource は AI 回答の取得元を解決するために必要な依存をまとめたもの。
// テストでは各フィールドへ偽の実装を注入できる。
type inputSource struct {
	// StdinIsPipe は os.Stdin が pipe（非端末）かどうか。
	StdinIsPipe bool
	// ClipboardFlag は --clipboard が指定されたかどうか。
	ClipboardFlag bool

	// Stdin は StdinIsPipe が true の場合に読み込む入力。
	Stdin io.Reader

	// ReadClipboard はクリップボード読み取りの実装（既定は clipboard.ReadClipboard）。
	ReadClipboard func([]string) (string, error)
	// ClipboardCommand は ReadClipboard に渡すコマンド。
	ClipboardCommand []string

	// AILog は stdin・clipboard のどちらも使えない場合に使う
	// 直前 AI 回答の取得元（ailog.Source の実装）。
	AILog ailog.Source
}

// resolveAnswer は SPEC.md §21 の優先順位に従って AI 回答本文を取得する。
//  1. stdin が pipe ならそれを使う（既存挙動を変えない）
//  2. --clipboard 指定ならクリップボードを使う
//  3. それ以外は AILog（既定では Claude Code のセッションログ）から取得する
//  4. いずれも得られない場合は errNoAnswerSource を返す
func resolveAnswer(src inputSource) (string, error) {
	switch {
	case src.StdinIsPipe:
		return transport.ReadStdin(src.Stdin)

	case src.ClipboardFlag:
		text, err := src.ReadClipboard(src.ClipboardCommand)
		if err != nil {
			return "", fmt.Errorf("%w: %v", errNoAnswerSource, err)
		}
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("%w: clipboard is empty", errNoAnswerSource)
		}
		return text, nil

	default:
		if src.AILog == nil {
			return "", errNoAnswerSource
		}
		text, err := src.AILog.LastAnswer()
		if err != nil {
			return "", fmt.Errorf("%w: %v", errNoAnswerSource, err)
		}
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("%w: AI log answer is empty", errNoAnswerSource)
		}
		return text, nil
	}
}

// isPipe は f が端末（tty）ではないことを os.ModeCharDevice の有無で判定する。
// pipe された stdin だけでなく、通常ファイルもここでは true（非端末）になる。
func isPipe(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

// NewRootCmd は pinline のルートコマンドを構築する。
//
// Use は "pinline"、Aliases に "pi" を含めることで、同一実装を
// pinline / pi のどちらの名前で起動しても動作するようにする
// （配布時にバイナリを pi という名前でも配置する運用と組み合わせる）。
func NewRootCmd() *cobra.Command {
	var clipboardFlag bool

	cmd := &cobra.Command{
		Use:          "pinline",
		Aliases:      []string{"pi"},
		Short:        "AI CLI とエディタをつなぐファイルベースの会話プロトコル CLI",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			logRoot := ""
			if home, err := os.UserHomeDir(); err == nil {
				logRoot = home + "/.claude/projects"
			}

			src := inputSource{
				StdinIsPipe:      isPipe(os.Stdin),
				ClipboardFlag:    clipboardFlag,
				Stdin:            cmd.InOrStdin(),
				ReadClipboard:    clipboard.ReadClipboard,
				ClipboardCommand: clipboard.DefaultClipboardCommand,
				AILog:            ailog.ClaudeLog{LogRoot: logRoot, Cwd: cwd},
			}

			return run(src, cmd.OutOrStdout(), cmd.ErrOrStderr(), os.Getenv, defaultOpenEditor)
		},
	}

	cmd.Flags().BoolVar(&clipboardFlag, "clipboard", false, "AI 回答をクリップボードから取得する")

	return cmd
}

// openEditorFunc は解決済みのエディタコマンドで path を開き、終了（保存完了）
// まで待機する関数の型。本番では defaultOpenEditor（tty フォールバック付き
// の editor.OpenInteractive）を使うが、テストでは tty を必要としない実装へ
// 差し替えられるようにする。
type openEditorFunc func(command []string, path string) error

// defaultOpenEditor は pinline プロセス自身の実際の標準入出力
// (os.Stdin/os.Stdout/os.Stderr) を使って editor.OpenInteractive を呼び出す。
// stdin/stdout がパイプの場合（`answer | pinline` 等）でも、
// git が $EDITOR を起動する際と同様に /dev/tty へ接続し直してから
// エディタを起動する。
func defaultOpenEditor(command []string, path string) error {
	return editor.OpenInteractive(command, path, os.Stdin, os.Stdout, os.Stderr, editor.DefaultInteractiveOptions())
}

// run は MVP の中心フローを実行する。
//
//  1. 入力元の優先順位に従って AI 回答を取得する（取得できなければ使い方を
//     表示して非0終了し、エディタは起動しない）
//  2. Markdown blockquote 化する
//  3. 一時ファイル（.md）へ保存する
//  4. $EDITOR で開く（stdin/stdout がパイプでも /dev/tty へフォールバック）
//  5. エディタ終了（保存完了）を待つ
//  6. 編集済み Markdown を stdout へ返す（UI メッセージは stderr）
func run(src inputSource, stdout, stderr io.Writer, getenv func(string) string, openEditor openEditorFunc) error {
	input, err := resolveAnswer(src)
	if err != nil {
		fmt.Fprintln(stderr, "Error: could not obtain an AI answer to edit.")
		fmt.Fprintln(stderr, usageMessage)
		return err
	}

	quoted := document.ToBlockquote(input)

	tmpFile, err := os.CreateTemp("", "pinline-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(quoted); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	editorCmd := editor.ResolveCommand(getenv)
	fmt.Fprintf(stderr, "Opening %s with %v...\n", tmpPath, editorCmd)

	// エディタ自体の対話的な入出力は、pinline プロセス自身の stdin/stdout は
	// AI 回答の読み込みと編集済み Markdown の返却専用に使うため使わない。
	// openEditor（本番では defaultOpenEditor）が必要に応じて /dev/tty 等へ
	// 接続してエディタを起動する。
	if err := openEditor(editorCmd, tmpPath); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if err := transport.WriteStdout(stdout, string(edited)); err != nil {
		return fmt.Errorf("failed to write stdout: %w", err)
	}

	return nil
}
