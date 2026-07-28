// Package port はユースケースの境界（repository / 参照 / 時計 / 入出力）と
// その DTO・操作者・エラーを定義する。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-3 に従い、
// domain と Go 標準ライブラリのみを import する。
package port

import "errors"

// usecase のエラーは番兵値として公開し、errors.Is で判別できるようにする（AC-11）。
var (
	// ErrWorkMonthNotFound は対象の勤務月が存在しないことを表す。
	ErrWorkMonthNotFound = errors.New("port: work month not found")

	// ErrContractNotFound は対象の契約が存在しないことを表す。
	ErrContractNotFound = errors.New("port: contract not found")

	// ErrUnauthenticated は操作者が未認証（ゲスト）であることを表す。
	ErrUnauthenticated = errors.New("port: unauthenticated")

	// ErrNotOwner は操作者がその勤務月の技術者本人でないことを表す。
	ErrNotOwner = errors.New("port: actor is not the owner")

	// ErrNotApprover は操作者が承認者ロールを持たないことを表す。
	ErrNotApprover = errors.New("port: actor is not an approver")

	// ErrSelfApproval は自己承認・自己差戻しであることを表す。
	ErrSelfApproval = errors.New("port: self approval is not allowed")
)
