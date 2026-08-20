// Package transport は stdin からの読み込みと stdout への書き出しを担当する。
package transport

import (
	"io"
)

// ReadStdin は r の内容をすべて読み込み、文字列として返す。
func ReadStdin(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
