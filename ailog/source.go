package ailog

// Source は「直前の AI 回答テキストを取得する」処理の抽象。
//
// Claude Code のセッションログ（ClaudeLog）を最初の実装とするが、
// 将来的に Codex 等の別ログ形式を実装として追加できるよう、
// 呼び出し側（cmd パッケージ等）はこの最小限の interface にのみ依存する
// （duck typing）。
type Source interface {
	// LastAnswer は直前の AI 回答本文を返す。取得できない場合はエラーを返す。
	LastAnswer() (string, error)
}
