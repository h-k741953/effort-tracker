package lambda

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
)

// writeResult は presenter.Result を HTTP 応答として書き込む（AC-9-13-b。
// presenter 自身は HTTP へ書き込まず、driver/lambda がこれを担う）。
func writeResult(w http.ResponseWriter, result presenter.Result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.StatusCode)
	// レスポンスボディの直列化失敗は Lambda 実行環境の異常であり、この時点で
	// ステータス行は既に書き込み済みのため、書き込み側にできることはない
	// （AC-13-19 の射程外）。
	_ = json.NewEncoder(w).Encode(result.Body)
}

// errInvokerDidNotRespond は driver/lambda 自身が定義する番兵で、「invoker が
// 一度も Present/PresentError を呼ばなかった」という driver/lambda 自身の異常
// （AC-9-13-c）を表す。presenter の errorMapping（AC-11-12・AC-11-13）に
// 対応する行を意図的に置かないため、PresentError に渡すと対応表に無いエラー
// として INTERNAL_ERROR に写る（AC-11-11）。`code`／ステータスの対応表は
// presenter 以外に持たせない（AC-11-10）。
var errInvokerDidNotRespond = errors.New("driver/lambda: invoker did not call Present or PresentError")

// resultPresenter は Present の入出力 DTO の型を問わず、driver/lambda が結果を
// 取り出すために要る最小の形（WorkMonthPresenter・ListWorkMonthsPresenter の
// いずれも構造的にこれを満たす）。
type resultPresenter interface {
	PresentError(err error)
	Result() (presenter.Result, bool)
}

// writeResultOrDelegate は output の結果を HTTP 応答として書き込む。invoker が
// 一度も Present/PresentError を呼ばなかった場合（AC-9-13-c）は、driver/lambda
// 自身の番兵（errInvokerDidNotRespond）を output.PresentError へ渡すことで
// INTERNAL_ERROR を得る。`code`／ステータスを driver/lambda が直書きしない
// （AC-11-10）。
func writeResultOrDelegate(w http.ResponseWriter, output resultPresenter) {
	result, ok := output.Result()
	if !ok {
		output.PresentError(errInvokerDidNotRespond)
		result, _ = output.Result()
	}
	writeResult(w, result)
}
