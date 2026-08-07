package lambda_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-10-4（時計の実装。
//     port.Clock を実装し「当日」を返す。基準タイムゾーンの解決を担うのは
//     この層だけ）・AC-6-5（Clock は Today() workmonth.Date を返す）。
//
// 基準タイムゾーンは daily-record-entry.md D-8（JST・Asia/Tokyo）が持ち、
// 本テストはその値を書き写すのではなく、実装が JST を基準にしていることを
// 実行時に time.Now().In(Asia/Tokyo) と突き合わせて検証する。

// dateView は Date の非公開フィールドを go-cmp で比較するための公開アクセサ経由
// のビュー（AC-12-4）。
type dateView struct {
	Year, Month, Day int
}

func viewOfDate(d workmonth.Date) dateView {
	return dateView{Year: d.Year(), Month: d.Month(), Day: d.Day()}
}

// TestSystemClock_Today_ReturnsJSTToday は driver/lambda が実装する port.Clock
// （lambda.SystemClock）の Today() が、JST（Asia/Tokyo）の当日を返すことを
// 検証する（AC-10-4・AC-6-5）。
func TestSystemClock_Today_ReturnsJSTToday(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("Asia/Tokyo のロードに失敗した: %v", err)
	}

	var c lambda.SystemClock

	got := c.Today()

	now := time.Now().In(loc)
	want, err := workmonth.NewDate(now.Year(), int(now.Month()), now.Day())
	if err != nil {
		t.Fatalf("期待値の Date 構築に失敗した: %v", err)
	}

	if diff := cmp.Diff(viewOfDate(want), viewOfDate(got)); diff != "" {
		t.Errorf("Today() が JST の当日と不一致 (-want +got):\n%s（AC-10-4・AC-6-5）", diff)
	}
}

// TestSystemClock_ImplementsPortClock は lambda.SystemClock が port.Clock を
// 満たすこと自体をコンパイル時に固定する（AC-10-4「port.Clock を実装し」）。
func TestSystemClock_ImplementsPortClock(t *testing.T) {
	var _ port.Clock = lambda.SystemClock{}
}
