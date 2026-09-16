package ailog

import (
	"os"
	"path/filepath"
)

// Provider は AI セッションログの操作を抽象化する。
// Claude Code と Codex の両方に対応する。
type Provider interface {
	Source
	ListSessions(n int) ([]SessionInfo, error)
	CurrentLogPath() (string, error)
	ReadAssistantTexts(logPath string, n int) ([]AssistantEntry, error)
	ReadEntries(logPath string, n int) ([]AssistantEntry, error)
}

// ClaudeProvider は Claude Code のセッションログに対する Provider 実装。
type ClaudeProvider struct {
	LogRoot   string
	Cwd       string
	SessionID string
}

func (p *ClaudeProvider) LastAnswer() (string, error) {
	if p.SessionID != "" {
		path, err := SessionLogPathByID(p.LogRoot, p.Cwd, p.SessionID)
		if err != nil {
			return "", err
		}
		return LastAssistantText(path)
	}
	return ReadLastClaudeAnswer(p.LogRoot, p.Cwd)
}

func (p *ClaudeProvider) ListSessions(n int) ([]SessionInfo, error) {
	return ListSessions(p.LogRoot, p.Cwd, n)
}

func (p *ClaudeProvider) CurrentLogPath() (string, error) {
	if p.SessionID != "" {
		return SessionLogPathByID(p.LogRoot, p.Cwd, p.SessionID)
	}
	return LatestSessionLogPath(p.LogRoot, p.Cwd)
}

func (p *ClaudeProvider) ReadAssistantTexts(logPath string, n int) ([]AssistantEntry, error) {
	return LastNAssistantTexts(logPath, n)
}

func (p *ClaudeProvider) ReadEntries(logPath string, n int) ([]AssistantEntry, error) {
	return LastNEntries(logPath, n)
}

var _ Provider = (*ClaudeProvider)(nil)

// Detect は環境に応じて適切な Provider を返す。
//
// 検出順序:
//  1. PINLINE_PROVIDER 環境変数（"claude" / "codex"）
//  2. CLAUDE_CODE_SESSION_ID 環境変数 → Claude Code
//  3. ~/.claude/projects が存在 → Claude Code
//  4. ~/.codex/sessions が存在 → Codex
//  5. 両方存在する場合は、対象 cwd のログがある方を優先
//  6. どちらもなければ nil
func Detect(getenv func(string) string, cwd string) Provider {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	if p := getenv("PINLINE_PROVIDER"); p != "" {
		switch p {
		case "codex":
			return &CodexProvider{
				LogRoot: filepath.Join(home, ".codex", "sessions"),
				Cwd:     cwd,
			}
		case "claude":
			return &ClaudeProvider{
				LogRoot:   filepath.Join(home, ".claude", "projects"),
				Cwd:       cwd,
				SessionID: getenv("CLAUDE_CODE_SESSION_ID"),
			}
		}
	}

	if sid := getenv("CLAUDE_CODE_SESSION_ID"); sid != "" {
		return &ClaudeProvider{
			LogRoot:   filepath.Join(home, ".claude", "projects"),
			Cwd:       cwd,
			SessionID: sid,
		}
	}

	claudeRoot := filepath.Join(home, ".claude", "projects")
	codexRoot := filepath.Join(home, ".codex", "sessions")

	claudeExists := dirExists(claudeRoot)
	codexExists := dirExists(codexRoot)

	switch {
	case claudeExists && !codexExists:
		return &ClaudeProvider{LogRoot: claudeRoot, Cwd: cwd}
	case codexExists && !claudeExists:
		return &CodexProvider{LogRoot: codexRoot, Cwd: cwd}
	case claudeExists && codexExists:
		cp := &ClaudeProvider{LogRoot: claudeRoot, Cwd: cwd}
		if _, err := cp.CurrentLogPath(); err == nil {
			return cp
		}
		xp := &CodexProvider{LogRoot: codexRoot, Cwd: cwd}
		if _, err := xp.CurrentLogPath(); err == nil {
			return xp
		}
		return cp
	default:
		return nil
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
