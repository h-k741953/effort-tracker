package lambda

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-10-4（時計の実装。
//     port.Clock を実装し「当日」を返す。基準タイムゾーンの解決を担うのは
//     この層だけ）・AC-6-5（Clock は Today() workmonth.Date を返す）。
//
// 基準タイムゾーンは daily-record-entry.md D-8（JST・Asia/Tokyo）が持つ。
// 本テストは package lambda 内部（外部テストパッケージではない）に置き、
// パッケージ変数 jst（jstLocationName から解決された実体）へ直接アクセスする。
// これにより jstLocationName が "Asia/Tokyo" 以外へ変異した場合に、
// UTC/JST の日付が一致する時間帯でも確実に Red になる（壁時計時刻には
// 依存しない固定時刻のテーブル駆動）。

// dateView は Date の非公開フィールドを go-cmp で比較するための公開アクセサ経由
// のビュー（AC-12-4）。
type dateView struct {
	Year, Month, Day int
}

func viewOfDate(d workmonth.Date) dateView {
	return dateView{Year: d.Year(), Month: d.Month(), Day: d.Day()}
}

// TestTodayIn_JSTBoundary は todayIn が JST（Asia/Tokyo, UTC+9）の日付境界で
// 正しく「当日」を返すことを、固定時刻のテーブル駆動で検証する（AC-10-4）。
//
// UTC 2026-08-07T14:59:00Z は JST 2026-08-07T23:59:00+09:00（JST でまだ8/7）。
// UTC 2026-08-07T15:00:00Z は JST 2026-08-08T00:00:00+09:00（JST で8/8に切替）。
// loc に jstLocationName から解決した本物の jst を渡すため、jstLocationName が
// "Asia/Tokyo" 以外（例 "UTC"）に変異すると、この2件は必ず Red になる。
func TestTodayIn_JSTBoundary(t *testing.T) {
	tests := []struct {
		name string
		utc  time.Time
		want dateView
	}{
		{
			name: "JST切替の1分前はJSTでまだ前日",
			utc:  time.Date(2026, 8, 7, 14, 59, 0, 0, time.UTC),
			want: dateView{Year: 2026, Month: 8, Day: 7},
		},
		{
			name: "JST0時ちょうどはJSTで当日へ切り替わる",
			utc:  time.Date(2026, 8, 7, 15, 0, 0, 0, time.UTC),
			want: dateView{Year: 2026, Month: 8, Day: 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := viewOfDate(todayIn(tt.utc, jst))

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("todayIn() が期待するJSTの当日と不一致 (-want +got):\n%s（AC-10-4）", diff)
			}
		})
	}
}

// TestSystemClock_Today_DelegatesToTodayInWithNow は SystemClock.Today() が
// todayIn(time.Now(), jst) へ委譲していることを検証する（AC-10-4・AC-6-5）。
// 呼び出しの前後で time.Now() を挟むことで、Today() が同じ結果を返すことを
// （日付境界を跨がない限り）決定的に確認する。日付境界を跨いだ場合は before /
// after のいずれかと一致するはずであり、それ以外は不整合として扱う。
func TestSystemClock_Today_DelegatesToTodayInWithNow(t *testing.T) {
	before := viewOfDate(todayIn(time.Now(), jst))

	var c SystemClock
	got := viewOfDate(c.Today())

	after := viewOfDate(todayIn(time.Now(), jst))

	if got != before && got != after {
		t.Errorf("SystemClock.Today() = %+v, want %+v または %+v の範囲内（AC-10-4）", got, before, after)
	}
}
