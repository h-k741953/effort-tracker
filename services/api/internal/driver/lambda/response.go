package lambda

import (
	"encoding/json"
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

// writeInternalErrorResult は presenter が一度も呼ばれなかった場合の応答
// （AC-9-13-c）。`code`／ステータスの対応表を持たず、固定の INTERNAL_ERROR を
// 直接組み立てる（対応表自体は presenter.PresentError の中にしかない。
// ここは「結果が無かった」という driver/lambda 自身の異常であり、presenter に
// 委譲できるエラー値が無いため対応表を経由しない）。
func writeInternalErrorResult(w http.ResponseWriter) {
	writeResult(w, presenter.Result{
		StatusCode: http.StatusInternalServerError,
		Body: presenter.ErrorResponse{Error: presenter.ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "internal error",
		}},
	})
}
