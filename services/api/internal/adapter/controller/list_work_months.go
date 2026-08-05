package controller

import (
	"context"
	"net/http"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// listWorkMonthsInvoker は ListWorkMonths（E-2）の呼び出し先が満たす最小の
// interface（AC-9-8-a）。
type listWorkMonthsInvoker interface {
	Execute(ctx context.Context, input port.ListWorkMonthsInput)
}

// HandleListWorkMonths は E-2（GET /work-months）を入力 DTO へ変換し
// invoker を呼ぶ（AC-9-5-b）。
//
// engineerId を省略した要求（承認待ち一覧）は契約 AC-9 順1 の対象であり、
// 両ヘッダ不在なら入力 DTO を組み立てずに port.ErrUnauthenticated を
// errorPresenter へ渡す（決定10・AC-9-7-a①）。engineerId を指定した要求は
// 両ヘッダ不在でも弾かず、未認証の Actor を渡す（AC-9-7-a②・AC-9-7-d）。
// 承認待ち一覧のロール要求（Approver）は判定しない（AC-9-6-j・AC-8-10）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない）。
func HandleListWorkMonths(_ *http.Request, _ listWorkMonthsInvoker, _ errorPresenter) {
	// TODO(implementer): AC-9-5-b・AC-9-6-j・AC-9-6-k・AC-9-7・決定10 を実装する。
}
