# pinline — Inline Prompt Protocol / CLI 仕様

## 1. 概要

`pinline` は、Claude Code や Codex などの対話型 AI CLI と、ユーザーの任意のテキストエディターを仲介するための軽量 CLI ツールである。

AI の長文回答をチャット UI 上で逐次処理するのではなく、回答を Markdown ファイルとしてエディターに展開し、ユーザーが回答中の任意箇所へインラインでコメント・修正指示を書き込めるようにする。

保存されたファイルを次のプロンプトとして AI CLI に戻すことで、テキストファイルそのものを AI との共有会話面として扱う。

正式コマンド名は `pinline`。短縮コマンドとして `pi` も同一機能で利用可能とする。

---

## 2. 背景・目的

対話型 AI CLI では、回答が長くなるほど以下の問題が発生する。

- 複数の論点に対して個別に返信するのが難しい
- 対象箇所を毎回引用してプロンプトへ貼り直す必要がある
- 長い会話では「どの文章に対する指示か」が離れ、文脈がずれやすい
- ターミナル上の入力欄は長文編集に適していない
- Vim / Neovim / Zed など、既存エディターの編集機能を活用できない

`pinline` は AI の能力やチャットクライアントを置き換えるものではない。

目的は、AI CLI と `$EDITOR` の間に薄いプロトコルを設け、AI の回答とユーザーの次プロンプトを「編集可能なファイル」に変換することである。

---

## 3. 基本コンセプト

```text
Claude Code / Codex
        ↓
      pinline
        ↓
   Markdown file
        ↓
 $EDITOR (Vim / Zed / etc.)
        ↓
 inline prompt / edit
        ↓
      pinline
        ↓
Claude Code / Codex
```

AI の回答はファイルへ書き出す。

ユーザーはそのファイルを `$EDITOR` で開き、回答本文の途中や直下にインラインでプロンプトを書く。

保存後、`pinline` は編集済みファイルを次の AI 入力として利用できる状態にする。

---

## 4. 想定利用環境

主な利用対象:

- Claude Code の対話モード
- Codex CLI の対話モード
- その他、標準入出力またはファイル参照によってプロンプトを渡せる AI CLI

ターミナル・シェル・エディターには依存しない。

想定例:

- WezTerm + zsh + tmux + Zed
- WezTerm + zsh + tmux + Vim / Neovim
- SSH 環境 + tmux + Vim
- 任意の terminal emulator + `$EDITOR`

`pinline` 自体は tmux、Zed、Vim 等の代替ではなく、それらを接続する仲介層として動作する。

---

## 5. コマンド名

正式名称:

```sh
pinline
```

短縮名称:

```sh
pi
```

`pinline` と `pi` は同一の実装を呼び出し、基本的に挙動を変えない。

配布バイナリまたはインストール時のリンクとして両方を利用可能にする。

---

## 6. 基本 UX

### 6.1 最小フロー

AI CLI で回答を受け取る。

```text
AI > 長い回答...
```

`pinline` を呼び出す。

```sh
pinline
```

または:

```sh
pi
```

直前の AI 回答を Markdown として生成し、`$EDITOR` で開く。

例:

```md
> AI の回答1行目
>
> AI の回答2行目
>
> AI の回答3行目

ここは前提が違う。AではなくBとして考えてください。

> AI の次の段落
> ...
```

ユーザーが保存してエディターを閉じる。

その内容を次の AI プロンプトとして利用する。

---

## 7. インラインプロンプト形式

v1 では特殊な独自構文を極力持たない。

AI の元回答を Markdown blockquote (`>`) として格納する。

ユーザーが追加した通常テキストをインラインプロンプトとして扱う。

例:

```md
> API は REST を採用するのが適切です。
> 理由はクライアントとの互換性が高いためです。

ここは GraphQL 前提でもう一度比較してください。

> キャッシュには Redis を利用します。

Redis を入れない構成も検討してください。
```

これにより以下を実現する。

- コメント対象と指示が物理的に近い
- コピー＆ペーストによる引用が不要
- 複数箇所への指示を1ターンでまとめて送れる
- Markdown として普通に読める
- Vim / Zed 等の既存編集機能だけで操作できる

---

## 8. ファイルを介したプロンプト

保存後の入力方式は複数サポートできる設計とする。

### 8.1 Direct mode

編集済み Markdown 全体を、そのまま次のプロンプト本文として利用する。

概念例:

```sh
pi --direct
```

AI へ渡す内容:

```text
以下は前回の回答と、それに対する私のインラインコメントです。
引用部分は前回のAI回答、非引用部分は私の指示として扱ってください。
各コメントの対象となる周辺文脈を考慮して回答してください。

<edited markdown>
```

### 8.2 File mode

編集済みファイルを保持し、AI にはファイルパスを参照させる。

概念例:

```sh
pi --file
```

AI へ渡す内容:

```text
/path/to/pinline/turn-00042.md を読んでください。
引用部分は前回の回答、非引用部分は私のインライン指示です。
各指示に対応してください。
```

Claude Code / Codex がワークスペース上のファイルを直接読める場合はこちらを優先できる。

---

## 9. `$EDITOR` 連携

エディターは固定しない。

優先順位:

1. `PLINE_EDITOR`
2. `VISUAL`
3. `EDITOR`
4. OS ごとの安全なフォールバック

例:

```sh
export EDITOR=nvim
pi
```

```sh
export EDITOR="zed --wait"
pi
```

重要なのは、`pinline` が特定エディターの UI 機能へ依存しないことである。

ファイルを開いて編集し、保存完了を検知できればよい。

GUI エディターの場合は `--wait` 相当のオプションを設定できる必要がある。

---

## 10. AI との境界

`pinline` は AI モデルを内蔵しない。

原則として以下を担当しない。

- LLM API 呼び出し
- モデル選択
- Claude / OpenAI の認証
- AI セッション管理
- AI の回答生成

これらは Claude Code / Codex 等の既存 CLI に任せる。

`pinline` の責務は以下のみ。

```text
AI output
   ↓
capture / normalize
   ↓
editable markdown
   ↓
$EDITOR
   ↓
edited prompt
   ↓
AI input
```

---

## 11. 対話モードとの統合

理想的な利用形態は、Claude Code / Codex の対話セッションを維持したまま `pinline` を呼び出すことである。

例:

```text
Claude Code session
────────────────────────────

User > この設計について考えて

Claude > 長文回答...

User > !pinline
```

あるいは AI CLI が bang command / shell escape を提供している場合:

```text
!pi
```

`pinline` は可能であれば現在の対話セッションの直前回答を取得して編集ファイルへ変換する。

AI CLI から直前回答を直接取得する安定 API が存在しない場合は、stdin・clipboard・ログ・明示的な pipe など複数の入力経路を用意する。

---

## 12. stdin / stdout の設計

`icb` と同様、Unix CLI としてパイプ可能な設計を優先する。

例:

```sh
some-ai-output | pinline
```

stdin が pipe の場合:

1. stdin を AI 回答として読み込む
2. Markdown blockquote へ変換
3. 一時または永続ファイルを作成
4. `$EDITOR` で開く
5. 保存後、編集済み内容を stdout へ出力

これにより:

```sh
some-ai-output | pi | some-ai-input
```

のような組み合わせも可能になる。

stdout は可能な限り機械利用可能な純粋テキストとし、UI メッセージは stderr に出す。

---

## 13. 保存方式

v1 では、まずファイル単体で成立することを優先する。

候補ディレクトリ:

```text
~/.local/share/pinline/
```

または OS 標準の user data directory。

例:

```text
~/.local/share/pinline/
├── sessions/
│   └── <session-id>/
│       ├── turn-00001.md
│       ├── turn-00002.md
│       └── turn-00003.md
└── state.json
```

ただし v1 の最小実装では、一時ファイルのみでもよい。

---

## 14. ファイルバウンダリー

長期的には AI 会話全体を1ファイルへ無制限に蓄積しない。

基本単位は「編集対象となる1回答 + それに対するインライン指示」とする。

```text
turn-N.md
```

1ファイルに含めるもの:

- 対象となる AI 回答
- その回答へユーザーが追記したインライン指示
- 必要最低限のメタ情報

過去の全会話履歴は AI CLI 本体のセッション管理に任せる。

`pinline` は「現在レビュー・編集している回答」に集中する。

これによりファイルサイズと文脈境界を明確に保つ。

---

## 15. コメントの寿命

v1 ではコメント専用 DB や JSONL を必須にしない。

ユーザーコメントは編集済み Markdown に直接存在する。

AI へ送った後の扱いは以下のいずれかを選べる設計にする。

### Replace

次の AI 回答を新しいファイルとして生成する。

過去のインラインコメントは前ターンファイルに残る。

### Append

同一ファイルへ新しい AI 回答を追記する。

### Archive

送信済みファイルを turn ファイルとして保存し、新しい回答用ファイルを生成する。

初期実装では `Archive + new turn` を基本とする。

コメントの resolved/unresolved 状態や JSONL による構造化管理は、実際に必要性が確認されてから追加する。

---

## 16. 将来的な構造化コメント

将来的には以下のような sidecar JSONL を追加可能とする。

```text
turn-00042.md
turn-00042.comments.jsonl
```

例:

```json
{"id":"c1","anchor":"API は REST","comment":"GraphQL 前提でも比較して","status":"open"}
{"id":"c2","anchor":"Redis","comment":"Redis なしも検討して","status":"resolved"}
```

ただしこれは v1 の必須仕様ではない。

本文内コメントだけで十分に運用できる間は導入しない。

---

## 17. tmux との関係

`pinline` は tmux と競合しない。

役割を明確に分離する。

```text
tmux
  = AI CLI セッションの永続化 / pane管理

Zed / Vim / Neovim
  = 長文回答とプロンプトの編集UI

pinline
  = AI CLI とエディター間の変換・受け渡し
```

この構成により、Zed のターミナルスレッドを利用する場合も tmux を捨てる必要はない。

---

## 18. 技術スタック

実装言語は Go。

基本方針・依存ライブラリ・リポジトリ構成は `yyYank/icb` と同系統とする。

`icb` は Go 1.21、Cobra を CLI フレームワーク、Bubble Tea を TUI フレームワークとして利用している。

`pinline` でも以下を基本候補とする。

| 役割 | 技術 |
|---|---|
| Language | Go |
| CLI framework | Cobra |
| TUI framework | Bubble Tea |
| TUI components | Bubbles（必要な場合） |
| 永続化 | Plain Markdown / JSON / JSON Lines |
| OS abstraction | Go standard library |
| Process execution | `os/exec` |

TUI は必須ではない。

`pinline` の中心は `$EDITOR` 連携であり、Bubble Tea は履歴選択やモード選択など、明確な用途が生まれた場合のみ利用する。

---

## 19. 想定ディレクトリ構成

`icb` と同様に責務ごとに小さく分割する。

```text
pinline/
├── main.go
├── cmd/
│   └── root.go
├── editor/
│   └── editor.go
├── document/
│   └── markdown.go
├── session/
│   └── session.go
├── transport/
│   ├── stdin.go
│   └── stdout.go
└── store/
    └── store.go
```

将来的に TUI が必要になった場合:

```text
├── tui/
│   └── tui.go
```

---

## 20. v1 コマンド仕様案

### `pinline` / `pi`

基本操作。

```sh
pi
```

入力元を判定し、AI 回答を編集可能 Markdown として `$EDITOR` で開く。

### Pipe input

```sh
cat answer.txt | pi
```

stdin を AI 回答として扱う。

### Explicit file

```sh
pi answer.md
```

指定ファイルを編集対象として開く。

### Direct output

```sh
pi --direct
```

保存後、AI へ直接投入できるプロンプト本文を stdout に出力する。

### File output

```sh
pi --file
```

保存後、ファイルを保持し、そのファイルを参照させるためのプロンプトまたはパスを返す。

### Keep

```sh
pi --keep
```

一時ファイルではなく履歴として保存する。

---

## 21. 入力元の優先順位

v1 の候補:

1. 明示されたファイル引数
2. pipe された stdin
3. AI CLI から取得可能な直前応答
4. 保存済みの直前 `pinline` turn
5. エラー

AI CLI ごとの内部ログ形式へ強く依存する実装は避ける。

可能な限り、stdin とファイルを共通プロトコルとして使う。

---

## 22. 出力 Markdown の例

AI の回答:

```text
APIにはRESTを使うべきです。

キャッシュにはRedisを利用します。
```

`pinline` が生成:

```md
# AI Response

> APIにはRESTを使うべきです.
>
> キャッシュにはRedisを利用します.
```

ユーザー編集後:

```md
# AI Response

> APIにはRESTを使うべきです。

GraphQLの場合との比較も追加してください。

> キャッシュにはRedisを利用します。

ここはRedisを導入しない案も検討してください。

最後に両方を踏まえた推奨構成を出してください。
```

次ターンではこのファイル全体を AI に渡す。

---

## 23. AI へ付加する標準指示

Direct mode で必要な場合、`pinline` が以下と同等の短いメタ指示を先頭へ付与する。

```text
以下のMarkdownは、あなたの前回の回答を引用形式で含み、
その周辺にユーザーがインラインで指示を書き加えたものです。

引用されていないユーザーの文章を指示として扱い、
位置関係から対象となる引用部分を判断してください。
複数の指示がある場合はすべて処理してください。
```

AI 固有の Skill や特殊プロンプトがなくても動作することを目標とする。

---

## 24. 非目標

v1 では以下を行わない。

- 独自チャット UI の構築
- AI モデル/API の実装
- Claude Code / Codex の置き換え
- エディターの再実装
- PR レビュー UI
- LSP のような複雑なインライン表示
- コメントアンカーの完全な追跡
- 全 AI 会話履歴の独自管理
- tmux / Zed / Vim 固有依存

---

## 25. 設計原則

### Editor-first

長文の読み書きはターミナル入力欄ではなく、既存のテキストエディターへ任せる。

### File-first

AI とユーザーの間で受け渡す共通形式はプレーンテキスト / Markdown とする。

### AI-agnostic

Claude Code / Codex を中心に想定するが、特定 AI CLI の内部実装へ依存しない。

### Editor-agnostic

Vim / Neovim / Zed その他、ユーザーの `$EDITOR` をそのまま利用する。

### Unix composability

stdin / stdout / file path を使い、他の CLI と組み合わせられる設計を優先する。

### Minimal protocol

最初から構造化データや専用 UI を増やさず、

```text
answer → quote → edit → save → prompt
```

という最小プロトコルを成立させる。

---

## 26. MVP

最初の実装で必要なのは以下のみ。

```text
1. stdin から AI 回答を受け取る
2. Markdown blockquote 化する
3. 一時ファイルへ保存する
4. $EDITOR で開く
5. 保存完了を待つ
6. 編集済み Markdown を stdout へ返す
7. pinline / pi の両方で起動できる
```

利用イメージ:

```sh
cat answer.txt | pi
```

または AI CLI とのラッパーから:

```sh
last_answer | pi | next_prompt
```

この段階で「AI の長文回答を任意エディターへ持ち込み、インラインで返信する」という中心価値は成立する。

---

## 27. 将来拡張

MVP の利用後、必要性が確認できたものから追加する。

- Claude Code 専用 integration
- Codex 専用 integration
- tmux integration
- Zed integration
- shell integration
- turn history
- session picker
- JSONL metadata
- resolved / unresolved comments
- diff 表示
- AI answer / user prompt の syntax highlight
- `pinline init`
- config file
- `pinline resume`
- `pinline history`

専用 integration を追加しても、中心のファイルプロトコルは変更しない。
