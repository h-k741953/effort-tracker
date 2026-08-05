// Package controller は HTTP リクエスト（パス・クエリ・ヘッダ・ボディ）を
// 入力 DTO へ変換し、ユースケースを呼ぶ（AC-9-1）。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-5 に従い、
// usecase/port・domain・標準ライブラリ・HTTP 関連のみを import する。
// **usecase/interactor は import しない**（AC-9-8-a・決定11）。呼び出し先は
// 本パッケージが自ら宣言する最小の interface（各ハンドラファイルの
// xxxInvoker）として受け取り、driver/lambda が実体を結線する。
//
// 本ファイル時点の実装はテスト工程（tester）が置いた**スタブ**であり、
// 業務ロジックを持たない（docs/rules/development-process.md の TDD。
// Red を確認してから実装工程が中身を書く）。
package controller

import "errors"

// ErrInvalidRequest はリクエストの構文・型・書式が不正であることを表す
// 要求側の識別子（AC-9-9-a）。domain の workmonth.ErrInvalidValue とは
// 別の識別子であり、両者を1つに兼ねない（AC-9-9-b・AC-11-13）。
//
// 置き場所と名前は固定しない（AC-9-9-d・AC-13-17）。本パッケージに置くのは
// tester が選んだ一案であり、実装工程が変更してよい。
var ErrInvalidRequest = errors.New("controller: invalid request")

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
// 与える既定値（AC-9-6-k）。**値そのものは契約・本仕様のいずれも固定しない**
// （domain-api-http-contract.md AC-3-5、実装設計 AC-13-16）ため、ここでの値は
// tester が置いた仮の値であり、実装工程が変更してよい。テストはこの定数を
// 参照するのみで、リテラル値を期待値に埋め込まない。
const DefaultListLimit = 20
