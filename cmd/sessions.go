package cmd

import (
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

var errSessionsCancelled = fmt.Errorf("sessions selection: %w", ErrCancelled)

type sessionsDeps struct {
	LogRoot    string
	Cwd        string
	N          int
	OutputFile string
	Getenv     func(string) string
	OpenEditor openEditorFunc
	RunSessionTUI func(tui.SessionSelectorModel) (tui.SessionSelectorModel, error)
	RunTUI        func(tui.SelectorModel) (tui.SelectorModel, error)

	HasTTY        func() bool
	TmuxAvailable func() bool
	SelfPath      func() (string, error)
}

func defaultRunSessionTUI(m tui.SessionSelectorModel) (tui.SessionSelectorModel, error) {
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInputTTY())
	final, err := p.Run()
	if err != nil {
		return m, err
	}
	return final.(tui.SessionSelectorModel), nil
}

func runSessions(deps sessionsDeps, stdout, stderr io.Writer) error {
	sessions, err := ailog.ListSessions(deps.LogRoot, deps.Cwd, deps.N)
	if err != nil {
		fmt.Fprintln(stderr, "Error: セッション一覧を取得できません。")
		return err
	}

	if len(sessions) == 0 {
		fmt.Fprintln(stderr, "Error: セッションが見つかりません。")
		return fmt.Errorf("no sessions found")
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

	sm := tui.NewSessionSelectorModel(sessions)
	sessionResult, err := deps.RunSessionTUI(sm)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if sessionResult.Cancelled() || sessionResult.Selected() == nil {
		return errSessionsCancelled
	}

	selected := sessionResult.Selected()

	entryLimit := deps.N
	if sessionResult.Filter() != "" {
		entryLimit = 0
	}
	entries, err := ailog.LastNEntries(selected.LogPath, entryLimit)
	if err != nil {
		fmt.Fprintln(stderr, "Error: 選択されたセッションにエントリがありません。")
		return err
	}

	em := tui.NewSelectorModel(entries)
	runTUI := deps.RunTUI
	if runTUI == nil {
		runTUI = defaultRunTUI
	}
	entryResult, err := runTUI(em)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if entryResult.Cancelled() || entryResult.Selected() == nil {
		return errSessionsCancelled
	}

	entry := entryResult.Selected()
	quoted := document.ToBlockquote(entry.Text)

	tmpFile, err := os.CreateTemp("", "pinline-sessions-*.md")
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

func (d sessionsDeps) inTmux() bool {
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

func (d sessionsDeps) runViaTmuxPopup(stdout, stderr io.Writer) error {
	selfPath := d.SelfPath
	if selfPath == nil {
		selfPath = os.Executable
	}
	exe, err := selfPath()
	if err != nil {
		return fmt.Errorf("failed to resolve self path: %w", err)
	}

	outputFile, err := os.CreateTemp("", "pinline-sessions-output-*.md")
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	fmt.Fprintln(stderr, "tmux popupでセッション選択UIを起動します。")

	args := []string{
		"display-popup", "-E", "-w", "90%", "-h", "90%", "-d", d.Cwd, "--",
		exe, "sessions", "-n", strconv.Itoa(d.N), "--_output", outputPath,
	}
	cmd := exec.Command("tmux", args...)
	cmd.Stdin = nil
	if f, ok := stderr.(*os.File); ok {
		cmd.Stdout = f
		cmd.Stderr = f
	}
	runErr := cmd.Run()

	result, err := os.ReadFile(outputPath)
	if err != nil {
		if runErr != nil {
			return fmt.Errorf("tmux popup failed: %w", runErr)
		}
		return fmt.Errorf("failed to read popup output: %w", err)
	}
	if len(result) == 0 {
		return errSessionsCancelled
	}

	return transport.WriteStdout(stdout, string(result))
}

func newSessionsCmd() *cobra.Command {
	var n int
	var outputFile string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "セッション一覧から選択してAI応答履歴を引用する",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			logRoot := ""
			if home, err := os.UserHomeDir(); err == nil {
				logRoot = home + "/.claude/projects"
			}

			deps := sessionsDeps{
				LogRoot:       logRoot,
				Cwd:           cwd,
				N:             n,
				OutputFile:    outputFile,
				Getenv:        os.Getenv,
				OpenEditor:    defaultOpenEditor,
				RunSessionTUI: defaultRunSessionTUI,
				RunTUI:        defaultRunTUI,
				HasTTY:        defaultHasTTY,
				TmuxAvailable: editor.DefaultTmuxAvailable,
			}

			return runSessions(deps, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().IntVarP(&n, "number", "n", 10, "表示するセッションの件数")
	cmd.Flags().StringVar(&outputFile, "_output", "", "")
	cmd.Flags().MarkHidden("_output")

	return cmd
}
