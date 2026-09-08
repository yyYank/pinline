package cmd

import "errors"

// ErrCancelled はユーザーがTUI操作をキャンセルしたことを表す。
// main.go でこのエラーを判定し、正常終了（exit 0）として扱う。
var ErrCancelled = errors.New("cancelled")
