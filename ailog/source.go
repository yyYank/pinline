package ailog

// Source は「直前の AI 回答テキストを取得する」処理の抽象。
//
// Claude Code（ClaudeProvider）と Codex（CodexProvider）を実装として持ち、
// Detect 関数が環境に応じて適切な Provider を返す。
// 呼び出し側（cmd パッケージ等）は Provider または Source interface に依存する。
type Source interface {
	// LastAnswer は直前の AI 回答本文を返す。取得できない場合はエラーを返す。
	LastAnswer() (string, error)
}
