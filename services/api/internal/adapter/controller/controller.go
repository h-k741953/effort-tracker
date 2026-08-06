// Package controller は HTTP リクエスト（パス・クエリ・ヘッダ・ボディ）を
// 入力 DTO へ変換し、ユースケースを呼ぶ（AC-9-1）。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-5 に従い、
// usecase/port・domain・標準ライブラリ・HTTP 関連のみを import する。
// **usecase/interactor は import しない**（AC-9-8-a・決定11）。呼び出し先は
// 本パッケージが自ら宣言する最小の interface（各ハンドラファイルの
// xxxInvoker）として受け取り、driver/lambda が実体を結線する。
//
// 各エンドポイントのハンドラは1ファイルずつに分かれており、本ファイルには
// それらに共通する最小の宣言（早期失敗の報告先と、一覧の件数の既定値・上限）
// のみを置く。要求の解釈に伴う判定順序は契約 AC-9 に従う（AC-9-6 の注記・
// 決定10）。
package controller

// errorPresenter は、ユースケースを呼ばずに早期に失敗を報告するために
// controller が要る最小の形（PresentError のみ）。usecase/port の各出力
// ポート（port.WorkMonthOutputPort 等）はいずれもこれを満たす。
//
// 一覧の出力ポート（AC-9-5 直後の blockquote が「まだ存在しない」と記す）の
// 具体的な形を本パッケージが知らずに済むよう、Present(...) を含む全体では
// なく PresentError だけを要求する（決定11と同じ「最小の interface を自ら
// 宣言する」形）。
type errorPresenter interface {
	PresentError(err error)
}

// DefaultListLimit は一覧（E-2）で limit が省略されたときに controller が
// 与える既定値（AC-9-6-k。適用値は入力 DTO に載せる）。**値そのものは契約・
// 実装設計のいずれも固定せず、実装 PR の選択に委ねられている**
// （domain-api-http-contract.md AC-3-5、実装設計 AC-13-16）。本定数はその選択
// として本パッケージが確定させた値であり、値を固定したくなった時点でまず
// 契約側へ書く（AC-13-16）。テストはこの定数を参照するのみで、リテラル値を
// 期待値に埋め込まない（AC-12-9）。
const DefaultListLimit = 20

// MaxListLimit は一覧（E-2）の limit に設ける上限（AC-9-6-k）。**値そのものは
// DefaultListLimit と同じく契約・実装設計のいずれも固定しない**（AC-3-5・
// AC-13-16）が、上限が存在すること自体はコストガードレール
// （docs/rules/cost-guardrails.md。Lambda の予約同時実行数と Neon への1クエリの
// 応答サイズ）の観点で必要である。既定値の数倍を切り上限として、単一
// リクエストが Neon への負荷と Lambda のメモリ／応答時間を膨らませないように
// 本パッケージが選定した（仕様が固定した値ではない）。
const MaxListLimit = 100
