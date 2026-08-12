package lambda

import (
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-16「④ 全体の組み立て」の境界を実装する（AC-12-17 ①）。
//
// SQL 実行インターフェース（gateway.DB＝AC-9-14-e）と port.Clock（AC-6-5）を
// 引数として受け取り、契約 E-1〜E-7 の7エンドポイントすべてを結線した1つの
// http.Handler を返す。内側で AC-10-8 の①（NewRouter）②（各 NewXxxHandler）
// ③（各 BuildXxxInvoker）を使うだけで、ルーティングの規則も DI の中身も
// 本ファイルは持たない。
//
// DB 接続の確立も Lambda ランタイムの初期化も行わない（AC-10-9 と同じ分界。
// 確立済みの実装を受け取るだけ）。接続を確立する側はエントリポイント
// （AC-10-15・AC-10-18）である。
//
// 取り違えの型による防御の限界（AC-10-16）: Endpoints の各フィールドはいずれも
// http.Handler であり、位置への代入の取り違えはコンパイルで落ちない。これを
// Red にする手段は組み立てた http.Handler を通した振る舞いの検査だけである
// （AC-12-17 ①。そこで区別できない組は残りうる＝AC-13-24 ②）。

// NewHandler は7エンドポイントを結線した1つの http.Handler を返す（AC-10-16）。
func NewHandler(db gateway.DB, clock port.Clock) http.Handler {
	return NewRouter(Endpoints{
		GetWorkMonth:      NewGetWorkMonthHandler(BuildGetWorkMonthInvoker(db)),
		ListWorkMonths:    NewListWorkMonthsHandler(BuildListWorkMonthsInvoker(db)),
		EnterDailyRecord:  NewEnterDailyRecordHandler(BuildEnterDailyRecordInvoker(db, clock)),
		DeleteDailyRecord: NewDeleteDailyRecordHandler(BuildDeleteDailyRecordInvoker(db)),
		CloseWorkMonth:    NewCloseWorkMonthHandler(BuildCloseWorkMonthInvoker(db)),
		ApproveWorkMonth:  NewApproveWorkMonthHandler(BuildApproveWorkMonthInvoker(db)),
		RejectWorkMonth:   NewRejectWorkMonthHandler(BuildRejectWorkMonthInvoker(db)),
	})
}
