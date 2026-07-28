package port

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 【スタブ】本ファイルは tester 工程が置いた境界の定義であり、
// 実装（gateway / presenter / driver）は implementer 工程が用意する。

// Role は操作者のロール（AC-7-2・D-9）。値はユビキタス言語の英語名と一致させる。
// ゲストは未認証の操作者として表し、Guest というロール値は設けない。
type Role string

const (
	// RoleEngineer は技術者。
	RoleEngineer Role = "Engineer"
	// RoleApprover は承認者。
	RoleApprover Role = "Approver"
)

// Actor は操作者を表す（D-9）。
type Actor struct {
	ID            string
	Role          Role
	Authenticated bool
}

// Contract は契約の参照専用リードモデル（AC-6-3）。
type Contract struct {
	ID              workmonth.ContractID
	DisplayName     string
	EngineerID      string
	SettlementRange workmonth.SettlementRange
}

// WorkMonthRepository は勤務月の永続化の境界（AC-6-1）。
type WorkMonthRepository interface {
	Find(ctx context.Context, contractID workmonth.ContractID, yearMonth workmonth.YearMonth) (*workmonth.WorkMonth, error)
	Save(ctx context.Context, target *workmonth.WorkMonth) error
}

// ContractRepository は契約の読み取り専用の境界（AC-6-2）。
type ContractRepository interface {
	Find(ctx context.Context, contractID workmonth.ContractID) (Contract, error)
}

// Clock は「当日」を与える境界（AC-6-5）。実装は driver が JST で返す。
type Clock interface {
	Today() workmonth.Date
}

// EnterDailyRecordInput は稼働実績の入力・編集の入力 DTO（AC-7-2）。
//
// 稼働時間を値オブジェクトではなく素の整数で受け取るのは、値域の検査を
// 認可・状態の判定より後に行うためである（判定順序は
// docs/specs/domain-api-http-contract.md AC-9）。
type EnterDailyRecordInput struct {
	Actor      Actor
	ContractID workmonth.ContractID
	YearMonth  workmonth.YearMonth
	Date       workmonth.Date
	Hours      int
	Minutes    int
}

// DeleteDailyRecordInput は稼働実績の削除の入力 DTO（AC-7-2）。
type DeleteDailyRecordInput struct {
	Actor      Actor
	ContractID workmonth.ContractID
	YearMonth  workmonth.YearMonth
	Date       workmonth.Date
}

// Hours は時分の素の値（AC-7-4）。
type Hours struct {
	Hours   int
	Minutes int
}

// SettlementRangeOutput は精算幅の出力 DTO。
type SettlementRangeOutput struct {
	LowerBound Hours
	UpperBound Hours
}

// DailyRecordOutput は稼働実績1件の出力 DTO。
type DailyRecordOutput struct {
	Date                string // YYYY-MM-DD
	WorkingHours        Hours
	RoundedWorkingHours Hours
}

// WorkMonthOutput は勤務月1件の出力 DTO（AC-7-4・AC-7-5）。
// 未確定の超過／不足は nil で表す。
type WorkMonthOutput struct {
	ContractID          string
	ContractDisplayName string
	YearMonth           string // YYYY-MM
	State               string
	Generated           bool
	SettlementRange     SettlementRangeOutput
	TotalHours          Hours
	Excess              *Hours
	Shortfall           *Hours
	DailyRecords        []DailyRecordOutput
}

// WorkMonthOutputPort は勤務月1件を返す出力ポート（AC-7-3・AC-7-5）。
// interactor は戻り値を返さず、このポートを呼ぶ。
type WorkMonthOutputPort interface {
	Present(output WorkMonthOutput)
	PresentError(err error)
}
