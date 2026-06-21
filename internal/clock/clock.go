// Package clock は現在時刻の取得を抽象化し、テストで固定時刻を注入可能にする。
package clock

import "time"

// Clock は現在時刻を返す。
type Clock interface {
	Now() time.Time
}

// Real は実際の現在時刻を返す。
type Real struct{}

// Now は time.Now() を返す。
func (Real) Now() time.Time { return time.Now() }

// Fake は固定時刻を返す（テスト用）。
type Fake struct {
	T time.Time
}

// Now は設定された時刻を返す。
func (f Fake) Now() time.Time { return f.T }
