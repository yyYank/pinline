package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/yyYank/pinline/ailog"
	"github.com/yyYank/pinline/document"
	"github.com/yyYank/pinline/editor"
	"github.com/yyYank/pinline/transport"
	"github.com/yyYank/pinline/tui"
)

var errHistoryCancelled = errors.New("history selection cancelled")

type historyDeps struct {
	LogRoot    string
	Cwd        string
	N          int
	OutputFile string
	Getenv     func(string) string
	OpenEditor openEditorFunc
	RunTUI     func(tui.SelectorModel) (tui.SelectorModel, error)

	HasTTY        func() bool
	TmuxAvailable func() bool
	SelfPath      func() (string, error)
}

func defaultHasTTY() bool {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func defaultRunTUI(m tui.SelectorModel) (tui.SelectorModel, error) {
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return m, err
	}
	return final.(tui.SelectorModel), nil
}

func runHistory(deps historyDeps, stdout, stderr io.Writer) error {
	logPath, err := ailog.LatestSessionLogPath(deps.LogRoot, deps.Cwd)
	if err != nil {
		fmt.Fprintln(stderr, "Error: セッションログが見つかりません。")
		return err
	}

	entries, err := ailog.LastNAssistantTexts(logPath, deps.N)
	if err != nil {
		fmt.Fprintln(stderr, "Error: assistant応答が見つかりません。")
		return err
	}

	hasTTY := deps.HasTTY
	if hasTTY == nil {
		hasTTY = defaultHasTTY
	}
	if !hasTTY() && deps.OutputFile == "" {
		if deps.inTmux() {
			return deps.runViaTmuxPopup(stdout, stderr)
		}
		return fmt.Errorf("TUI error: TTYが無く、tmuxも利用できません。tmux内で実行してください")
	}

	m := tui.NewSelectorModel(entries)
	result, err := deps.RunTUI(m)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if result.Cancelled() || result.Selected() == nil {
		return errHistoryCancelled
	}

	selected := result.Selected()
	quoted := document.ToBlockquote(selected.Text)

	tmpFile, err := os.CreateTemp("", "pinline-history-*.md")
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

	editorCmd := editor.ResolveCommand(deps.Getenv)
	fmt.Fprintf(stderr, "Opening %s with %v...\n", tmpPath, editorCmd)

	if err := deps.OpenEditor(editorCmd, tmpPath); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	if deps.OutputFile != "" {
		return os.WriteFile(deps.OutputFile, edited, 0o644)
	}

	return transport.WriteStdout(stdout, string(edited))
}

func (d historyDeps) inTmux() bool {
	getenv := d.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	tmuxAvail := d.TmuxAvailable
	if tmuxAvail == nil {
		tmuxAvail = editor.DefaultTmuxAvailable
	}
	return getenv("TMUX") != "" && tmuxAvail()
}

func (d historyDeps) runViaTmuxPopup(stdout, stderr io.Writer) error {
	selfPath := d.SelfPath
	if selfPath == nil {
		selfPath = os.Executable
	}
	exe, err := selfPath()
	if err != nil {
		return fmt.Errorf("failed to resolve self path: %w", err)
	}

	outputFile, err := os.CreateTemp("", "pinline-history-output-*.md")
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	fmt.Fprintln(stderr, "tmux popupで履歴選択UIを起動します。")

	args := []string{
		"display-popup", "-E", "-w", "90%", "-h", "90%", "-d", d.Cwd, "--",
		exe, "history", "-n", strconv.Itoa(d.N), "--_output", outputPath,
	}
	cmd := exec.Command("tmux", args...)
	cmd.Stdin = nil
	if f, ok := stderr.(*os.File); ok {
		cmd.Stdout = f
		cmd.Stderr = f
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux popup failed: %w", err)
	}

	result, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to read popup output: %w", err)
	}
	if len(result) == 0 {
		return errHistoryCancelled
	}

	return transport.WriteStdout(stdout, string(result))
}

func newHistoryCmd() *cobra.Command {
	var n int
	var outputFile string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "過去のAI応答履歴を選択してエディタで開く",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			logRoot := ""
			if home, err := os.UserHomeDir(); err == nil {
				logRoot = home + "/.claude/projects"
			}

			deps := historyDeps{
				LogRoot:       logRoot,
				Cwd:           cwd,
				N:             n,
				OutputFile:    outputFile,
				Getenv:        os.Getenv,
				OpenEditor:    defaultOpenEditor,
				RunTUI:        defaultRunTUI,
				HasTTY:        defaultHasTTY,
				TmuxAvailable: editor.DefaultTmuxAvailable,
			}

			return runHistory(deps, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().IntVarP(&n, "number", "n", 10, "表示する履歴の件数")
	cmd.Flags().StringVar(&outputFile, "_output", "", "")
	cmd.Flags().MarkHidden("_output")

	return cmd
}
