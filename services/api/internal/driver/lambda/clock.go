// Package lambda は driver/lambda 層（AC-10）を実装する。
package lambda

import (
	"time"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// jstLocationName は「当日」の基準タイムゾーン（daily-record-entry.md D-8）。
// タイムゾーンの解決を担うのは driver/lambda 層だけであり、domain / usecase /
// adapter は基準タイムゾーンを知らない（AC-10-4）。
const jstLocationName = "Asia/Tokyo"

// SystemClock は port.Clock（AC-6-5）の実装で、JST（Asia/Tokyo）の当日を返す
// （AC-10-4）。
type SystemClock struct{}

// Today は現在時刻を JST に変換した「当日」を返す。
func (SystemClock) Today() workmonth.Date {
	loc, err := time.LoadLocation(jstLocationName)
	if err != nil {
		// tzdata は Go 標準ライブラリに同梱されるため、通常は到達しない
		// （AC-10-4 のタイムゾーン名は固定値）。フォールバックとして UTC を使う。
		loc = time.UTC
	}

	now := time.Now().In(loc)

	d, err := workmonth.NewDate(now.Year(), int(now.Month()), now.Day())
	if err != nil {
		// time.Now() が返す年月日は常に妥当な暦日であるため到達しない。
		panic(err)
	}

	return d
}
