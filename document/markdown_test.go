package document

import "testing"

// TestToBlockquote は ToBlockquote が SPEC.md §26 の変換ルールを満たすことを検証する。
func TestToBlockquote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "空入力は空文字を返す",
			input: "",
			want:  "",
		},
		{
			name:  "単一行に > を付与する",
			input: "hello",
			want:  "> hello",
		},
		{
			name:  "複数行それぞれに > を付与する",
			input: "hello\nworld",
			want:  "> hello\n> world",
		},
		{
			name:  "末尾改行がある入力は余分な引用行を作らない",
			input: "hello\nworld\n",
			want:  "> hello\n> world",
		},
		{
			name:  "空行は > のみ（末尾スペースなし）になる",
			input: "hello\n\nworld",
			want:  "> hello\n>\n> world",
		},
		{
			name:  "改行のみの入力は1つの空引用行になる",
			input: "\n",
			want:  ">",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToBlockquote(tt.input)
			if got != tt.want {
				t.Errorf("ToBlockquote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
