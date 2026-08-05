// Package presenter は usecase の出力ポートを実装し、出力 DTO を
// ViewModel（HTTP ステータス + JSON ボディ）へ変換する（AC-9-2）。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-5 に従い、
// usecase/port・domain・標準ライブラリ・HTTP 関連のみを import する。
// **driver は import しない**（AC-1-5）。**pgx も import しない**（AC-1-6）。
// 集約・repository を直接触らない（AC-9-2「やらないこと」）。
//
// 本ファイル時点の実装はテスト工程（tester）が置いた**スタブ**であり、
// 業務ロジック（ViewModel への変換、エラー → code の対応表）を持たない
// （docs/rules/development-process.md の TDD。Red を確認してから実装工程が
// 中身を書く）。
package presenter

// ErrorBody は契約 AC-9-1 の `error` フィールドの中身（`code` / `message`）。
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse は契約 AC-9-1 のエラー応答全体（`{ "error": { ... } }`）。
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// Result は1回の Present / PresentError 呼び出しの結果（ステータス + ボディ。
// AC-9-13-b）。`driver/lambda` がこれを取り出して直列化する。presenter 自身は
// HTTP の応答へ書き込まない（AC-9-13-b）。
//
// Body の具体的な型は成功時（AC-9-11 の ViewModel）と失敗時（ErrorResponse）で
// 異なる。型は本仕様も本パッケージも固定しない値の置き場所であり、
// `driver/lambda` 側で json.Marshal できればよい。
type Result struct {
	StatusCode int
	Body       any
}
