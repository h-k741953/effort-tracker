package port

import "github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"

// 本ファイルは契約一覧（E-8）・稼働の推移（E-9）の入出力 DTO と出力ポートを置く
// （実装設計 AC-7-21）。

// ListContractsInput は契約一覧（E-8）の入力 DTO。EngineerID は空文字列を取らない。
type ListContractsInput struct {
	Actor      Actor
	EngineerID string
	YearMonth  workmonth.YearMonth
}

// ListWorkTrendsInput は稼働の推移（E-9）の入力 DTO。EngineerID は空文字列を取らない。
type ListWorkTrendsInput struct {
	Actor      Actor
	EngineerID string
}

// ContractPeriodOutput は契約期間の出力 DTO（YYYY-MM）。End の nil は終了未定。
type ContractPeriodOutput struct {
	Start string
	End   *string
}

// ContractWorkMonthOutput は契約一覧の行が持つ、指定した年月の勤務月の出力 DTO。
type ContractWorkMonthOutput struct {
	YearMonth string
	State     string
	Generated bool
}

// ContractOutputRow は契約一覧の出力 DTO の1行。
type ContractOutputRow struct {
	ContractID          string
	ContractDisplayName string
	SettlementRange     SettlementRangeOutput
	ContractPeriod      ContractPeriodOutput
	WorkMonth           ContractWorkMonthOutput
}

// ListContractsOutput は契約一覧の出力 DTO。Items は0件でも空のスライス。
type ListContractsOutput struct {
	Items []ContractOutputRow
}

// ListContractsOutputPort は契約一覧の出力ポート。
type ListContractsOutputPort interface {
	Present(output ListContractsOutput)
	PresentError(err error)
}

// WorkTrendOutputRow は稼働の推移の出力 DTO の1行（勤務月1件）。
type WorkTrendOutputRow struct {
	ContractID      string
	YearMonth       string
	State           string
	SettlementRange SettlementRangeOutput
	TotalHours      Hours
	Excess          *Hours
	Shortfall       *Hours
}

// ListWorkTrendsOutput は稼働の推移の出力 DTO。Items は0件でも空のスライス。
type ListWorkTrendsOutput struct {
	Items []WorkTrendOutputRow
}

// ListWorkTrendsOutputPort は稼働の推移の出力ポート。
type ListWorkTrendsOutputPort interface {
	Present(output ListWorkTrendsOutput)
	PresentError(err error)
}
