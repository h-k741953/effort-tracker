package port

import "github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"

// 本ファイルは参照系（E-1・E-2）の入力 DTO を置く。
//
// docs/specs/workmonth-implementation-design.md AC-9-5 の直後の blockquote が
// 記すとおり、参照系の入力 DTO と一覧の出力ポートは本仕様の記述時点では
// 「まだ存在しない」ものとして扱われ、AC-6-4・AC-7-2・AC-7-5 の形で追加される
// ことを前提としていた。GetWorkMonth / ListWorkMonths の interactor 自体は
// 本 Issue（#51）の範囲外（controller のテストのみ）だが、controller が
// 「入力 DTO を1つ受け取り、戻り値を返さない」抽象（AC-9-8-a・決定11）を
// 宣言するには具体的な入力 DTO の型が要る。フィールドの内容は AC-9-5-a・
// AC-9-5-b・AC-6-4 が直接列挙する範囲に限り、それ以外は決めない
// （一覧の出力ポート・行のリードモデルは本ファイルに含めない）。

// GetWorkMonthInput は勤務月1件の取得（E-1）の入力 DTO（AC-9-5-a）。
// CloseWorkMonthInput 等と同じ形（操作者・契約識別子・年月のみ）。
type GetWorkMonthInput struct {
	Actor      Actor
	ContractID workmonth.ContractID
	YearMonth  workmonth.YearMonth
}

// ListWorkMonthsInput は勤務月一覧（E-2）の入力 DTO（AC-9-5-b）。
// 条件は参照ポート WorkMonthQuery が受け取るもの（AC-6-4）と対応させる。
//
// EngineerID・State は「省略可」（AC-9-5-b）であり、空文字列で「省略」を表す
// （技術者識別子・状態のいずれも空文字列を正当な値として持たないため）。
// Limit は省略時に controller が既定値を与え、適用値をここへ載せる（AC-9-6-k）。
type ListWorkMonthsInput struct {
	Actor      Actor
	EngineerID string
	State      string
	Limit      int
	Offset     int
}
