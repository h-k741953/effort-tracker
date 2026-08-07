// Package lambda は driver/lambda 層（AC-10）を実装する。
package lambda

import (
	"fmt"
	"time"

	// time/tzdata は標準ライブラリであり domain の許容 import には影響しない。
	// Lambda 実行環境には $GOROOT も /usr/share/zoneinfo も無く、
	// time.LoadLocation はシステムの zoneinfo を参照できないため、
	// バイナリへ tzdata を埋め込んで確実に解決させる（AC-10-4）。
	_ "time/tzdata"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// SystemClock が port.Clock（AC-6-5）を満たすことをコンパイル時に固定する
// （AC-10-4「port.Clock を実装し」）。
var _ port.Clock = SystemClock{}

// jstLocationName は「当日」の基準タイムゾーン（daily-record-entry.md D-8）。
// タイムゾーンの解決を担うのは driver/lambda 層だけであり、domain / usecase /
// adapter は基準タイムゾーンを知らない（AC-10-4）。
const jstLocationName = "Asia/Tokyo"

// jst はコールドスタート時（パッケージ初期化時）に一度だけ解決する。
// time/tzdata を import しているため通常は失敗しないが、万一 tzdata の
// 埋め込みに失敗した場合はここで panic し、誤って UTC 等へフォールバック
// したまま「当日」を返さない（AC-10-4、daily-record-entry.md D-4・D-8）。
var jst = mustLoadLocation(jstLocationName)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Errorf("基準タイムゾーン %q の解決に失敗した: %w", name, err))
	}
	return loc
}

// SystemClock は port.Clock（AC-6-5）の実装で、JST（Asia/Tokyo）の当日を返す
// （AC-10-4）。ゼロ値のまま使用できる。
type SystemClock struct{}

// Today は現在時刻を JST に変換した「当日」を返す。
func (SystemClock) Today() workmonth.Date {
	return todayIn(time.Now(), jst)
}

// todayIn は now を loc のタイムゾーンへ変換した「当日」を返す純関数。
// SystemClock.Today() から壁時計時刻とロケーションの解決を切り離すことで、
// 固定時刻での決定的なテストを可能にする。
func todayIn(now time.Time, loc *time.Location) workmonth.Date {
	local := now.In(loc)

	d, err := workmonth.NewDate(local.Year(), int(local.Month()), local.Day())
	if err != nil {
		// time.Time が返す年月日は常に妥当な暦日であるため到達しない。
		panic(err)
	}

	return d
}
