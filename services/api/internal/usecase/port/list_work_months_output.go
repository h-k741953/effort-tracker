package port

// 本ファイルは一覧（`ListWorkMonths`）の出力ポートと出力 DTO を置く（AC-7-17）。
// `GetWorkMonthInput` / `ListWorkMonthsInput`（入力 DTO）は reference_input.go に
// 置かれており、本ファイルはその出力側にあたる。

// ListWorkMonthsOutputRow は一覧の出力 DTO の1行（AC-7-17）。
//
// AC-6-7-d の4項目（契約識別子・契約表示名・年月・状態）を**素の値**で持つ
// （年月・状態は文字列＝AC-7-4）。WorkMonthQueryRow とは用途が異なる別の型
// とする（一方は永続化境界の読み取り専用モデル、他方はユースケースの出力）。
type ListWorkMonthsOutputRow struct {
	ContractID          string
	ContractDisplayName string
	YearMonth           string
	State               string
}

// ListWorkMonthsOutput は一覧の出力 DTO（AC-7-17）。
//
// Limit・Offset は入力 DTO（ListWorkMonthsInput）の値をそのまま載せる
// （controller が既定値・上限を適用済み）。Total は WorkMonthQuery が返した
// 値をそのまま載せる（interactor が数え直さない）。Items は該当0件でも
// 空スライス（nil ではない）。
type ListWorkMonthsOutput struct {
	Items  []ListWorkMonthsOutputRow
	Total  int
	Limit  int
	Offset int
}

// ListWorkMonthsOutputPort は一覧を返す出力ポート（AC-7-5・AC-7-17）。
// WorkMonthOutputPort（勤務月1件用）とは別の型とする（AC-9-13-d）。
type ListWorkMonthsOutputPort interface {
	Present(output ListWorkMonthsOutput)
	PresentError(err error)
}
