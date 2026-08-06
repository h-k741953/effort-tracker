package port

import "context"

// 本ファイルは AC-6-7（`WorkMonthQuery` の具体形）を置く。AC-6-4 が挙げる
// 参照ポートの1行をテストに落とせる粒度へ具体化したものであり、新しい
// ポートを足すものではない（AC-6-6 は不変。ポートは4つ + 出力ポートのまま）。

// WorkMonthQueryCondition は一覧の絞り込み条件（AC-6-7-b・AC-6-7-c）。
//
// 技術者識別子・状態・件数・開始位置の4つだけを持つ。並び順は条件に含めない
// （並び順は契約 AC-3-4 に固定されており、実現は SQL の ORDER BY。AC-9-18-c）。
// Actor は持たない（ポートは認可を判定しない。AC-9-18-f）ため、
// 入力 DTO（ListWorkMonthsInput）とは別の型とする。
// 省略は空文字列で表す（EngineerID・State のいずれも空文字列を正当な値として
// 持たない。入力 DTO の既存の表し方に揃える）。
type WorkMonthQueryCondition struct {
	EngineerID string
	State      string
	Limit      int
	Offset     int
}

// WorkMonthQueryRow は一覧の行のリードモデル（AC-6-7-d）。
//
// 契約識別子・契約表示名・年月・状態の4つだけを持ち、稼働実績・総稼働時間・
// 精算幅・超過／不足を持たない。AC-6-3 の Contract とは別の型（用途が異なる）。
//
// 各項目の型（domain の値オブジェクトか素の値か）は本仕様が固定していない
// （AC-6-7-d・AC-13-17 と同じ扱い）。本ファイルは素の値（文字列）を選んだ
// （tester の選択。年月は YYYY-MM）。実装工程が別の型を選ぶ場合は
// 本ファイルと参照側（gateway・interactor のテスト）を合わせて調整する。
type WorkMonthQueryRow struct {
	ContractID          string
	ContractDisplayName string
	YearMonth           string
	State               string
}

// WorkMonthQuery は一覧の参照ポート（AC-6-4・AC-6-7）。**`ListWorkMonths`
// だけ**が使う（`GetWorkMonth` は使わない。AC-6-7-a）。
type WorkMonthQuery interface {
	// List は条件に一致する行と総件数を返す。総件数は絞り込み後・
	// ページング適用前の件数であり、返した行の数ではない（AC-6-7-e）。
	List(ctx context.Context, condition WorkMonthQueryCondition) ([]WorkMonthQueryRow, int, error)
}
