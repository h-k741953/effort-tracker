package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
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
func HandleListWorkMonths(r *http.Request, invoker listWorkMonthsInvoker, output errorPresenter) {
	query := r.URL.Query()
	engineerID := query.Get("engineerId")
	state := query.Get("state")

	var actor port.Actor
	if engineerID == "" {
		// 承認待ち一覧（契約 AC-9 順1 の対象）。ヘッダの有無を他の構文検査より
		// 前に判定する（決定10）。
		a, err := requireActorHeader(r)
		if err != nil {
			output.PresentError(err)
			return
		}
		actor = a

		if state != string(workmonth.StatePendingApproval) {
			output.PresentError(fmt.Errorf(
				"%w: engineerId omitted requires state=PendingApproval, got %q", port.ErrInvalidRequest, state,
			))
			return
		}
	} else {
		a, err := buildActorAllowGuest(r)
		if err != nil {
			output.PresentError(err)
			return
		}
		actor = a

		if state != "" && !isListState(state) {
			output.PresentError(fmt.Errorf("%w: invalid state value: %q", port.ErrInvalidRequest, state))
			return
		}
	}

	limit, err := parseListLimit(query.Get("limit"))
	if err != nil {
		output.PresentError(err)
		return
	}

	offset, err := parseListOffset(query.Get("offset"))
	if err != nil {
		output.PresentError(err)
		return
	}

	invoker.Execute(r.Context(), port.ListWorkMonthsInput{
		Actor:      actor,
		EngineerID: engineerID,
		State:      state,
		Limit:      limit,
		Offset:     offset,
	})
}

// isListState は state クエリが3値（ユビキタス言語の英語名）のいずれかであることを
// 確認する（AC-9-6-j）。
func isListState(value string) bool {
	switch workmonth.State(value) {
	case workmonth.StateDraft, workmonth.StatePendingApproval, workmonth.StateApproved:
		return true
	default:
		return false
	}
}

// parseListLimit は limit クエリを解釈する（契約 AC-3-6）。省略時は
// controller が既定値を与える（AC-9-6-k）。上限（MaxListLimit）を超える値も
// 契約 AC-3-6 に従い INVALID_REQUEST とする（コストガードレールの観点。
// docs/rules/cost-guardrails.md）。
func parseListLimit(raw string) (int, error) {
	if raw == "" {
		return DefaultListLimit, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 || v > MaxListLimit {
		return 0, fmt.Errorf("%w: invalid limit: %q", port.ErrInvalidRequest, raw)
	}
	return v, nil
}

// parseListOffset は offset クエリを解釈する（契約 AC-3-6）。省略時は 0。
func parseListOffset(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%w: invalid offset: %q", port.ErrInvalidRequest, raw)
	}
	return v, nil
}
