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
// （extract.go に AC-3 の抽出規則そのものとして実装済み。詳細は
// extract.go と extract_ac3_test.go を見る）。
//
// dir には抽出対象のディレクトリのパスを渡す。本テストは自身のパッケージ
// ディレクトリ（`go test` 実行時のカレントディレクトリ＝
// services/api/cmd/exportlist）を "." で渡す。
//
// **空振りの排除（このテストが担保する設計）**: extractRecords(".") は
// このパッケージディレクトリの実ファイル（main.go・extract.go・本テスト
// 自身を含む *.go）を実際に読む。フィクスチャではなく実ソースを対象に
// するため、「0件だったから通った」を次の2点で偽陽性から区別する。
//
//  1. err != nil を Fatal で扱う。AC-3-13 により「対象の Go パッケージが
//     1つも見つからない」場合は非ゼロ（エラー）になる。もし "." の解決に
//     失敗しファイルが1つも走査されなければ、この分岐で本テストは失敗する
//     ため、「何も見なかったから0件」を「見た上で0件」と取り違えない。
//  2. extractRecords は本パッケージ内の他のテスト
//     （extract_ac3_test.go の TestExtractRecords_AC3 等）でも共有される
//     同一実装であり、それらは多数のフィクスチャに対して非空の具体的な
//     レコードを要求する。したがって extractRecords が「常に0件を返す」
//     ような no-op 実装であれば、本テストではなくそれらのテストが Red に
//     なる。本テストが local に担保するのは AC-3-15
//     （抽出器自身が公開シンボルを1つも持たないこと）そのものであり、
//     「extractRecords が動いていること」の担保は上記1点と、同一パッケージ
//     内の他テストとの組み合わせで成立する。
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
