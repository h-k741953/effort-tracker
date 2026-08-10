package lambda

import (
	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/interactor"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは AC-10-8③「具体型の結線」の境界を実装する（AC-12-15④）。
// gateway（repository の実装）と interactor（ユースケースの実装）を
// 具体型として組み立て、②が要求する形（buildInvoker 関数）にする。
//
// gateway.NewWorkMonthRepository・gateway.NewContractRepository は SQL 実行
// インターフェース（gateway.DB）だけを要求し、DB 接続そのもの（pgx・Neon）は
// 知らない（AC-9-14-a）。DB 接続の具体的な実装（driver/persistence）を組み立てて
// 渡すのは本パッケージの外（Lambda ハンドラのエントリポイント）の責務であり、
// 本ファイルの射程外（AC-13-19。driver/persistence は今回の実装対象外）。
//
// repository は DB 接続を共有してよい（状態を持たない）ため、buildInvoker を
// 返す関数の中で一度だけ組み立てる。出力ポート（presenter）だけがリクエスト
// ごとに異なり（AC-9-13-a）、interactor はその出力ポートを束ねて
// リクエストごとに新しく組み立てる。

// BuildGetWorkMonthInvoker は gateway.DB から GetWorkMonth（E-1）の
// interactor を組み立てる関数を返す（AC-10-8③）。
func BuildGetWorkMonthInvoker(db gateway.DB) func(port.WorkMonthOutputPort) GetWorkMonthInvoker {
	workMonths := gateway.NewWorkMonthRepository(db)
	contracts := gateway.NewContractRepository(db)

	return func(output port.WorkMonthOutputPort) GetWorkMonthInvoker {
		return interactor.NewGetWorkMonth(workMonths, contracts, output)
	}
}

// BuildEnterDailyRecordInvoker は gateway.DB と port.Clock から
// EnterDailyRecord（E-3）の interactor を組み立てる関数を返す（AC-10-8③）。
func BuildEnterDailyRecordInvoker(db gateway.DB, clock port.Clock) func(port.WorkMonthOutputPort) EnterDailyRecordInvoker {
	workMonths := gateway.NewWorkMonthRepository(db)
	contracts := gateway.NewContractRepository(db)

	return func(output port.WorkMonthOutputPort) EnterDailyRecordInvoker {
		return interactor.NewEnterDailyRecord(workMonths, contracts, clock, output)
	}
}
