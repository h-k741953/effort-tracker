package workmonth

import "errors"

// ドメインのエラーは番兵値として公開し、errors.Is で判別できるようにする。
// 定義の根拠は docs/specs/workmonth-implementation-design.md AC-11。
var (
	// ErrNotEditable は Draft 以外の状態で稼働実績を入力・編集・削除しようとしたことを表す。
	ErrNotEditable = errors.New("workmonth: work month is not editable")

	// ErrDateOutOfMonth は対象日が当該勤務月の年月に属さないことを表す。
	ErrDateOutOfMonth = errors.New("workmonth: date is out of the work month")

	// ErrFutureDate は対象日が「当日」より後であることを表す。
	ErrFutureDate = errors.New("workmonth: date is in the future")

	// ErrTimeOfDayOutOfRange は時刻が値域外・粒度外であることを表す（AC-11-4）。
	ErrTimeOfDayOutOfRange = errors.New("workmonth: time of day out of range")

	// ErrInvalidTimeCombination は時刻の組み合わせが不正であることを表す（AC-11-4-a）。
	ErrInvalidTimeCombination = errors.New("workmonth: invalid time combination")

	// ErrInvalidValue は値オブジェクトの構築に失敗したことを表す。
	ErrInvalidValue = errors.New("workmonth: invalid value")

	// ErrNotClosable は Draft 以外の状態で締めようとしたことを表す
	// （二重締め・終端状態からの締め。monthly-closing.md AC-1-2・AC-1-3）。
	ErrNotClosable = errors.New("workmonth: work month is not closable")

	// ErrNotApprovable は PendingApproval 以外の状態で承認しようとしたことを表す
	// （二重承認・下書きの承認。approval.md AC-1-2・AC-1-3。実装設計 AC-4-4・AC-11-6）。
	ErrNotApprovable = errors.New("workmonth: work month is not approvable")

	// ErrNotRejectable は PendingApproval 以外の状態で差戻そうとしたことを表す
	// （下書き・終端状態からの差戻し。approval.md AC-2-2・AC-2-3。実装設計 AC-4-5・AC-11-6）。
	ErrNotRejectable = errors.New("workmonth: work month is not rejectable")
)
