# pinline

[English](#english) / [日本語](#日本語)

---

<a name="english"></a>

## English

`pinline` is a **file-based conversation protocol** that connects AI CLIs with your text editor.

It takes responses from interactive AI CLIs such as Claude Code or Codex, writes them to Markdown, and opens them in any `$EDITOR` such as Vim, Neovim, or Zed so you can write prompts directly inline with the response.

> **pinline is a protocol, not an AI client.**

It does not replace your AI client or editor. Its purpose is to define how conversations move between them.

## Concept

```text
AI output
   ↓
editable Markdown
   ↓
inline prompt in $EDITOR
   ↓
AI input
```

Instead of quoting and replying to parts of a long AI response one by one in a terminal, `pinline` treats the response itself as an editable file.

```md
> REST should be used for this API.

Also compare this with GraphQL.

> Redis should be used for caching.

Consider an architecture without Redis as well.
```

Because instructions can be written next to the text they refer to, `pinline` reduces copy-pasting and makes it easier to address multiple points in a single turn.

## Command

Full command:

```sh
pinline
```

Short command:

```sh
pi
```

Both provide the same functionality.

## Basic usage

```sh
cat answer.txt | pi
```

`pinline` converts the AI response into Markdown blockquotes and opens it in `$EDITOR`.

After editing and saving, the resulting document can be used as the next AI prompt through stdout or as a referenced file.

Typical flow:

```text
Claude Code / Codex
        ↓
       pi
        ↓
 conversation.md
        ↓
 Vim / Neovim / Zed
        ↓
 inline prompt
        ↓
 Claude Code / Codex
```

## Philosophy

The core of `pinline` is the protocol, not the UI.

- Editor-first
- File-first
- AI-agnostic
- Editor-agnostic
- Unix composable

Claude Code / Codex can continue handling AI sessions, tmux can continue handling persistence, and Zed / Vim can continue handling editing.

`pinline` only defines the minimal bridge between them:

```text
answer → quote → edit → save → prompt
```

## Status

The initial MVP is intentionally small:

```text
stdin
  → Markdown blockquote
  → temporary file
  → $EDITOR
  → save
  → stdout
```

The goal is to establish the file protocol first, without depending on AI-specific APIs or custom UI.

## Tech

- Go
- Cobra
- Bubble Tea / Bubbles, only if needed
- Markdown / JSON / JSONL
- `$EDITOR`
- stdin / stdout

The technical stack and repository structure are intended to follow the same general approach as `yyYank/icb`.

## License

Apache License 2.0

---

<a name="日本語"></a>

# 日本語

`pinline` は、AI CLI とテキストエディターの間をつなぐ **ファイルベースの会話プロトコル** です。

Claude Code や Codex のような対話型 AI CLI の回答を Markdown に落とし、Vim / Neovim / Zed など任意の `$EDITOR` で開いて、回答の途中へそのままインラインで指示を書き込めるようにします。

> **pinline は AI client ではなく protocol です。**

AI クライアントやエディターを置き換えるのではなく、両者の間にある「会話の受け渡し方」を定義することが目的です。

## Concept

```text
AI output
   ↓
editable Markdown
   ↓
inline prompt in $EDITOR
   ↓
AI input
```

長い AI 回答をターミナル上で一つずつ引用して返信する代わりに、回答そのものを編集可能なファイルとして扱います。

```md
> APIにはRESTを使うべきです。

GraphQLの場合も比較してください。

> キャッシュにはRedisを利用します。

Redisを使わない構成も検討してください。
```

対象となる文章の近くに指示を書けるため、引用のコピペを減らし、複数の論点を一度に扱いやすくします。

## Command

正式コマンド:

```sh
pinline
```

短縮コマンド:

```sh
pi
```

どちらも同じ機能を提供します。

## Basic usage

```sh
cat answer.txt | pi
```

`pinline` は入力された AI 回答を Markdown の引用形式に変換し、`$EDITOR` で開きます。

編集・保存した内容は、次の AI プロンプトとして stdout またはファイル経由で利用できます。

想定フロー:

```text
Claude Code / Codex
        ↓
       pi
        ↓
 conversation.md
        ↓
 Vim / Neovim / Zed
        ↓
 inline prompt
        ↓
 Claude Code / Codex
```

## Philosophy

`pinline` の中心は UI ではなく protocol です。

- Editor-first
- File-first
- AI-agnostic
- Editor-agnostic
- Unix composable

Claude Code / Codex のセッション管理、tmux の永続化、Zed や Vim の編集能力はそのまま利用します。

`pinline` はその間で、次の最小プロトコルだけを担当します。

```text
answer → quote → edit → save → prompt
```

## Status

初期実装では以下を MVP とします。

```text
stdin
  → Markdown blockquote
  → temporary file
  → $EDITOR
  → save
  → stdout
```

AI 固有の API や専用 UI に依存せず、まずは単純なファイルプロトコルとして成立させます。

## Tech

- Go
- Cobra
- Bubble Tea / Bubbles（必要になった場合のみ）
- Markdown / JSON / JSONL
- `$EDITOR`
- stdin / stdout

技術スタックとリポジトリ構成は `yyYank/icb` と同系統を想定しています。

## License

Apache License 2.0
