package main

import "errors"

// extract.go は AC-3-16 のテスト（main_test.go）をコンパイルさせるための
// 最小限の型・関数だけを置く。AC-3（抽出規則）そのものは実装しない。
// AC-3 の実装は implementer 工程の担当である。
//
// ここに置く名前・署名は docs/specs/public-api-diff-check.md AC-3-16-1 が
// 委任する「実装手段」であり、**tester が暫定的に固定したもの**である
// （AC-3-16-1 ②③）。implementer はこの形に合わせて AC-3 を実装するか、
// 都合が悪ければテスト（main_test.go）ごと見直す（AC-3-16-1 ⑤）。

// record は抽出結果の1レコードを表す。フィールドは
// docs/specs/public-api-diff-check.md AC-2（エクスポート一覧のレコード書式）
// の4フィールドに対応する。
type record struct {
	Pkg       string
	Kind      string
	Name      string
	Signature string
}

// errExtractRecordsNotImplemented は、AC-3 の抽出規則がまだ実装されていない
// ことを明示するための番人である。
//
// extractRecords がこのエラーを返す限り、main_test.go の
// TestExtractRecords_Self_HasNoRecords は「レコードが0件だったから」ではなく
// 「err != nil で Fatal したから」失敗する。空スライスを黙って返す実装だと
// 「未実装なので0件」という空振りが「実装済みで0件」に見えてしまうため、
// 成功として扱えるゼロ値ではなく明示的なエラーを返す。
var errExtractRecordsNotImplemented = errors.New("extractRecords: AC-3 の抽出規則は未実装（AC-3-16 のテストが要求する形だけを暫定的に固定している）")

// extractRecords は AC-3-16-1 ① が委任する「抽出規則の適用を、対象
// ディレクトリを与えてテストから呼べる形にする」ための入口である。
//
// dir には抽出対象のディレクトリのパスを渡す。戻り値は AC-3 の抽出規則を
// dir 配下へ適用した結果のレコード一覧である。
//
// 現時点では AC-3 を実装しておらず、常に errExtractRecordsNotImplemented を
// 返す。
func extractRecords(dir string) ([]record, error) {
	return nil, errExtractRecordsNotImplemented
}
