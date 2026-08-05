package presenter

import (
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// WorkMonthPresenter は port.WorkMonthOutputPort を実装する（AC-9-2）。
// リクエストごとに生成し、プロセス内で共有しない（AC-9-13-a）。
var _ port.WorkMonthOutputPort = (*WorkMonthPresenter)(nil)

// WorkMonthPresenter は成功・失敗いずれか1回の呼び出しの結果を保持する
// （AC-9-13-b）。
type WorkMonthPresenter struct {
	result *Result
}

// NewWorkMonthPresenter は WorkMonthPresenter を生成する（AC-9-13-a。
// リクエストごとに生成する）。
func NewWorkMonthPresenter() *WorkMonthPresenter {
	return &WorkMonthPresenter{}
}

// Present は勤務月1件の出力 DTO を ViewModel へ変換し保持する（AC-9-10・AC-9-11）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない）。
func (p *WorkMonthPresenter) Present(_ port.WorkMonthOutput) {
	// TODO(implementer): AC-9-10・AC-9-11（出力 DTO → ViewModel）を実装する。
}

// PresentError はエラーをステータス・code へ写して保持する（AC-9-12・AC-11-12・
// AC-11-13）。対応表に無いエラーは INTERNAL_ERROR とする（AC-11-11）。
//
// スタブ（tester が置いた最小実装。ビルドを通すためだけのもので業務ロジックを
// 持たない）。
func (p *WorkMonthPresenter) PresentError(_ error) {
	// TODO(implementer): AC-9-12・AC-11-12・AC-11-13（エラー → code の対応表）を実装する。
}

// Result は直近の Present / PresentError の呼び出し結果を返す。一度も
// 呼ばれていなければ ok は false（AC-9-13-c。`driver/lambda` はこれを
// INTERNAL_ERROR として応答する）。
func (p *WorkMonthPresenter) Result() (Result, bool) {
	if p.result == nil {
		return Result{}, false
	}
	return *p.result, true
}
