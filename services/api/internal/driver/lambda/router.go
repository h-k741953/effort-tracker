package lambda

import (
	"fmt"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
)

// 本ファイルは AC-10-8①「ルーティング」の境界を実装する（AC-12-15①②）。
// パス・メソッドの文字列は docs/specs/domain-api-http-contract.md の
// E-1〜E-7（79〜87行）をそのまま用いる。
//
// 標準 net/http.ServeMux（Go 1.22+ のパターン構文）を使う。パターンに
// メソッドを含めない（パスだけで登録する）のは、未定義メソッドへの応答を
// ServeMux 標準の 405 Method Not Allowed ではなく、契約が定める
// 404 / NOT_FOUND（AC-1-11・AC-9・AC-11-13）に揃えるため。メソッドの判定は
// 本ファイルの methodDispatch が担う。

// Endpoints は7エンドポイント分の http.Handler をまとめる（AC-10-8①）。
// 各フィールドの実体（②リクエストごとの結線・③具体型の結線）は本パッケージの
// 他のファイルが提供する。
type Endpoints struct {
	GetWorkMonth      http.Handler
	ListWorkMonths    http.Handler
	EnterDailyRecord  http.Handler
	DeleteDailyRecord http.Handler
	CloseWorkMonth    http.Handler
	ApproveWorkMonth  http.Handler
	RejectWorkMonth   http.Handler
}

// NewRouter は Endpoints をパス・メソッドへ振り分ける http.Handler を返す
// （AC-10-8①）。未定義のパス・メソッドは 404 / NOT_FOUND を返す
// （AC-1-11・AC-9・AC-11-13）。`code`／ステータスの対応表は持たず、
// presenter.ErrRouteNotFound を presenter へ渡すことで対応表を presenter に
// 一元化したまま保つ（AC-11-10）。
//
// Endpoints のいずれかのフィールドが nil（＝ driver 内の配線漏れ）の場合は
// panic する（コールドスタート時に失敗させる。clock.go の mustLoadLocation と
// 同じ形）。nil のまま受理すると methodDispatch がそのパス・メソッドを
// 「未定義」と同一視し、配線漏れが契約 AC-1-11 の 404 / NOT_FOUND と
// 見分けが付かなくなる（C-1(b)）。
func NewRouter(endpoints Endpoints) http.Handler {
	requireEndpoint("GetWorkMonth", endpoints.GetWorkMonth)
	requireEndpoint("ListWorkMonths", endpoints.ListWorkMonths)
	requireEndpoint("EnterDailyRecord", endpoints.EnterDailyRecord)
	requireEndpoint("DeleteDailyRecord", endpoints.DeleteDailyRecord)
	requireEndpoint("CloseWorkMonth", endpoints.CloseWorkMonth)
	requireEndpoint("ApproveWorkMonth", endpoints.ApproveWorkMonth)
	requireEndpoint("RejectWorkMonth", endpoints.RejectWorkMonth)

	mux := http.NewServeMux()

	mux.Handle("/work-months", methodDispatch(map[string]http.Handler{
		http.MethodGet: endpoints.ListWorkMonths,
	}))
	mux.Handle("/work-months/{contractId}/{yearMonth}", methodDispatch(map[string]http.Handler{
		http.MethodGet: endpoints.GetWorkMonth,
	}))
	mux.Handle("/work-months/{contractId}/{yearMonth}/daily-records/{date}", methodDispatch(map[string]http.Handler{
		http.MethodPut:    endpoints.EnterDailyRecord,
		http.MethodDelete: endpoints.DeleteDailyRecord,
	}))
	mux.Handle("/work-months/{contractId}/{yearMonth}/close", methodDispatch(map[string]http.Handler{
		http.MethodPost: endpoints.CloseWorkMonth,
	}))
	mux.Handle("/work-months/{contractId}/{yearMonth}/approve", methodDispatch(map[string]http.Handler{
		http.MethodPost: endpoints.ApproveWorkMonth,
	}))
	mux.Handle("/work-months/{contractId}/{yearMonth}/reject", methodDispatch(map[string]http.Handler{
		http.MethodPost: endpoints.RejectWorkMonth,
	}))
	// 上記以外のすべてのパスの受け皿（"/" は末尾スラッシュのため部分木として
	// 全パスにマッチするが、より具体的な登録済みパターンが優先されるため
	// 未定義のパスにのみ到達する。ServeMux の優先順位規則に依る）。
	mux.HandleFunc("/", notFoundHandler)

	return mux
}

// methodDispatch はパスが一致したリクエストのメソッドで振り分ける
// （AC-12-15①）。一致しないメソッドは 404 / NOT_FOUND とする（AC-12-15②）。
func methodDispatch(handlers map[string]http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers[r.Method]; ok && h != nil {
			h.ServeHTTP(w, r)
			return
		}
		notFoundHandler(w, r)
	}
}

// notFoundHandler は未定義のパス・メソッドへの応答（AC-1-11・AC-9・AC-11-13）。
// presenter.ErrRouteNotFound を presenter へ渡すだけで、`code`／ステータスの
// 対応表は自前で持たない（AC-11-10）。PresentError は必ず result を設定する
// ため、result の有無を判定する分岐は持たない（到達不能な分岐を作らない）。
func notFoundHandler(w http.ResponseWriter, _ *http.Request) {
	out := presenter.NewWorkMonthPresenter()
	out.PresentError(presenter.ErrRouteNotFound)
	result, _ := out.Result()
	writeResult(w, result)
}

// requireEndpoint は endpoints の1フィールドが nil でないことを確認する
// （C-1(b)）。nil なら配線漏れとして panic する。
func requireEndpoint(name string, h http.Handler) {
	if h == nil {
		panic(fmt.Sprintf("driver/lambda: NewRouter に Endpoints.%s が結線されていない（nil handler）", name))
	}
}
