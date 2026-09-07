package main

// 検証対象: docs/specs/public-api-diff-check.md AC-3-16
// （AC-3-15「抽出器自身はトップレベルの公開シンボルを1つも持たない」の
// 機械的な固定）。
//
// AC-3-16 が要求する検査内容はこの1点だけである（AC-3-16-1 ④
// 「検査の中身は緩めない」）:
//
//	抽出器のパッケージディレクトリ自身へ AC-3 の抽出規則を適用し、
//	得られるレコードが0件であること。
//
// **tester が決めた形（implementer が合わせる対象、AC-3-16-1 ②③。
// 名前・署名・分割の粒度は本仕様が固定するものではない）**:
//
//	type record struct {
//	    Pkg, Kind, Name, Signature string
//	}
//	func extractRecords(dir string) ([]record, error)
//
// （extract.go に置く。実装が入っていない現時点では常にエラーを返す
// スタブである。）
//
// dir には抽出対象のディレクトリのパスを渡す。本テストは自身のパッケージ
// ディレクトリ（`go test` 実行時のカレントディレクトリ＝
// services/api/cmd/exportlist）を "." で渡す。
//
// **空振りの排除（このテストが担保する設計）**: extractRecords は AC-3 の
// 抽出規則を実装していない現時点では、常にエラー
// （errExtractRecordsNotImplemented、extract.go）を返すスタブである。
// したがって本テストは「レコードが0件だったから通った」のではなく、
// 「err != nil で Fatal したから」Red になる。空スライスを黙って返す
// スタブだと「未実装だから0件」が「実装済みで0件」と区別できず空振りに
// なるため、スタブの戻り値を成功として扱えるゼロ値にしない
// （docs/harness/verification-loop.md「検査対象を読めなかったことは、
// 違反が無いことを意味しない」と同じ思想）。
// AC-3（抽出規則）が実装されエラーが解消されたあとに初めて、
// len(records) == 0 の判定が本来の検査として機能する。
//
// 依存: 標準 testing のみ。go-cmp は本テストの比較対象（長さ・エラーの
// 有無）に対して不要なため使わない。テストダブル・モックライブラリは
// 使わない（ADR 0007）。

import "testing"

// TestExtractRecords_Self_HasNoRecords は AC-3-16 本体を固定する。
func TestExtractRecords_Self_HasNoRecords(t *testing.T) {
	records, err := extractRecords(".")
	if err != nil {
		t.Fatalf(
			"extractRecords(\".\") がエラーを返した: %v"+
				"（AC-3 の抽出規則が未実装。実装後はエラー無く"+
				"0件になることを要求する＝AC-3-16）",
			err,
		)
	}

	if len(records) != 0 {
		t.Errorf(
			"extractRecords(\".\") が返したレコード数 = %d, want 0"+
				"（抽出器自身はトップレベルの公開シンボルを持たない"+
				"＝AC-3-15。詳細: %+v）",
			len(records), records,
		)
	}
}
