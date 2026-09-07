package main

// 検証対象: docs/specs/public-api-diff-check.md AC-3（抽出規則）本体。
//
// main_test.go（AC-3-16 / AC-3-15 の自己言及テスト）とは別に、
// 「対象パッケージが1つも無ければ0行を出力して非ゼロ終了」以外の
// AC-3 の各条文が、それぞれ独立に落ちることを固定する。1項目を
// 実装から削れば、対応するケースが最低1件 Red になる粒度にしている。
//
// 依存: 標準 testing + google/go-cmp（cmp / cmp/cmpopts）のみ。
// google/go-cmp は既に services/api/go.mod の require にあり、本ファイルの
// 追加によって require は増えない（AC-3-12 と同じ制約を守る）。
// cmpopts.SortSlices は「構造体・スライスの比較」の一部として使う
// （ADR 0007。抽出結果のスライス順序は AC-3 のどの条文も固定していない
// ため、順序に依存しない比較にする。最終的な出力の並び順は AC-2-7 が
// 比較器の外側＝抽出器の出力全体に対して定めるものであり、
// extractRecords 単体の戻り値の順序ではない）。
//
// 入力は t.TempDir() へ組み立てる。リポジトリ内の実ファイルには依存しない
// （main_test.go の "." ケースとは別物として並立させる）。
//
// extractRecords は現時点でスタブ（常にエラーを返す。extract.go）のため、
// 本ファイルの全ケースは Red になる。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// writeFixture は files（tmpDir からの相対パス → 内容）を一時ディレクトリへ
// 書き出し、そのディレクトリの絶対パスを返す。
func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", full, err)
		}
	}
	return dir
}

// byRecord は record スライスを Pkg/Kind/Name/Signature の順で安定に
// 並べ替えるための比較関数。extractRecords の戻り値の順序は AC-3 のどの
// 条文も固定していないため、テストの比較を順序に依存させない。
func byRecord(a, b record) bool {
	if a.Pkg != b.Pkg {
		return a.Pkg < b.Pkg
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Signature < b.Signature
}

func TestExtractRecords_AC3(t *testing.T) {
	tests := []struct {
		name  string // AC 番号を含める
		files map[string]string
		want  []record
	}{
		{
			// AC-3-1: _test.go / testdata / vendor を除外する。
			// pkgA/keep.go だけが対象で、他の3ファイルの公開シンボルは
			// 一切現れないことを固定する。
			name: "AC-3-1_excludes_test_go_testdata_vendor",
			files: map[string]string{
				"pkgA/keep.go": "package pkgA\n\nfunc Keep() {}\n",
				"pkgA/foo_test.go": "package pkgA\n\n" +
					"func TestFoo() {}\n\nfunc Extra() {}\n",
				"pkgA/testdata/sub.go": "package sub\n\nfunc FromTestdata() {}\n",
				"vendor/vpkg/thing.go": "package vpkg\n\nfunc FromVendor() {}\n",
			},
			want: []record{
				{Pkg: "pkgA", Kind: "func", Name: "Keep", Signature: "() ()"},
			},
		},
		{
			// AC-3-2: package main を除外しない（P-1 の帰結1）。
			name: "AC-3-2_package_main_not_excluded",
			files: map[string]string{
				"cmdpkg/main.go": "package main\n\nfunc Run() {}\n\nfunc main() {}\n",
			},
			want: []record{
				{Pkg: "cmdpkg", Kind: "func", Name: "Run", Signature: "() ()"},
			},
		},
		{
			// AC-3-4: トップレベルの公開宣言のみ。関数内ローカル宣言
			// （大文字で書いても対象外）・非公開の宣言は対象外。
			name: "AC-3-4_toplevel_exported_only",
			files: map[string]string{
				"toplevel/a.go": "package toplevel\n\n" +
					"func Exported() {\n" +
					"\ttype LocalType int\n" +
					"\tvar LocalVar int\n" +
					"\t_ = LocalVar\n" +
					"}\n\n" +
					"func unexported() {}\n\n" +
					"type hidden struct{}\n",
			},
			want: []record{
				{Pkg: "toplevel", Kind: "func", Name: "Exported", Signature: "() ()"},
			},
		},
		{
			// AC-3-5: メソッドは受信者の基底型名とメソッド名の両方が
			// 公開のときのみ対象。
			name: "AC-3-5_method_requires_both_exported",
			files: map[string]string{
				"methods/a.go": "package methods\n\n" +
					"type Pub struct{}\n\n" +
					"func (p Pub) PubMethod() {}\n" +
					"func (p Pub) privMethod() {}\n" +
					"func (p *Pub) PtrMethod() {}\n\n" +
					"type priv struct{}\n\n" +
					"func (p priv) PubMethod() {}\n",
			},
			want: []record{
				{Pkg: "methods", Kind: "type", Name: "Pub", Signature: "struct{}"},
				{Pkg: "methods", Kind: "method", Name: "Pub.PubMethod", Signature: "(Pub) () ()"},
				{Pkg: "methods", Kind: "method", Name: "Pub.PtrMethod", Signature: "(*Pub) () ()"},
			},
		},
		{
			// AC-3-6 + kind 別テーブル: func の型パラメータ、type の
			// 型パラメータ、型エイリアスの `= ` 前置、var/const の
			// 型省略時の `-`。
			name: "AC-3-6_kind_table_type_params_alias_missing_type",
			files: map[string]string{
				"kinds/a.go": "package kinds\n\n" +
					"import \"io\"\n\n" +
					"func Generic[T any, U comparable](x T, y int) (U, error) { panic(\"x\") }\n\n" +
					"type Box[T any] struct {\n\tVal T\n}\n\n" +
					"type Alias = io.Reader\n\n" +
					"var TypedVar int\n" +
					"var UntypedVar = 42\n\n" +
					"const TypedConst string = \"x\"\n" +
					"const UntypedConst = \"x\"\n",
			},
			want: []record{
				{Pkg: "kinds", Kind: "func", Name: "Generic", Signature: "[T any, U comparable] (T, int) (U, error)"},
				{Pkg: "kinds", Kind: "type", Name: "Box", Signature: "[T any] struct { Val T }"},
				{Pkg: "kinds", Kind: "type", Name: "Alias", Signature: "= io.Reader"},
				{Pkg: "kinds", Kind: "var", Name: "TypedVar", Signature: "int"},
				{Pkg: "kinds", Kind: "var", Name: "UntypedVar", Signature: "-"},
				{Pkg: "kinds", Kind: "const", Name: "TypedConst", Signature: "string"},
				{Pkg: "kinds", Kind: "const", Name: "UntypedConst", Signature: "-"},
			},
		},
		{
			// AC-3-7: go/printer 出力の正規化。改行・タブ・連続空白・
			// コメントが signature に現れない。
			name: "AC-3-7_normalizes_whitespace_and_drops_comments",
			files: map[string]string{
				"normalize/a.go": "package normalize\n\n" +
					"func WithComment(\n" +
					"\t// a comment about x\n" +
					"\tx int,\n" +
					"\ty string, // trailing comment\n" +
					") (\n" +
					"\t// result comment\n" +
					"\tok bool,\n" +
					") {\n" +
					"\treturn true\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "normalize", Kind: "func", Name: "WithComment", Signature: "(int, string) (bool)"},
			},
		},
		{
			// AC-3-8: 引数・結果の括弧付けと区切り、可変長引数。
			name: "AC-3-8_parens_commas_variadic",
			files: map[string]string{
				"parens/a.go": "package parens\n\n" +
					"func Variadic(nums ...int) []int { return nums }\n\n" +
					"func NoArgsNoResults() {}\n\n" +
					"func OneResult() int { return 0 }\n",
			},
			want: []record{
				{Pkg: "parens", Kind: "func", Name: "Variadic", Signature: "(...int) ([]int)"},
				{Pkg: "parens", Kind: "func", Name: "NoArgsNoResults", Signature: "() ()"},
				{Pkg: "parens", Kind: "func", Name: "OneResult", Signature: "() (int)"},
			},
		},
		{
			// AC-3-9: 構造体の非公開フィールド／インターフェースの
			// 非公開メソッドを signature から除去する。
			// （全メンバーが非公開になり0件へ落ちる境界での正規化後の
			// 空白表現は仕様が一意に定めないため、ここでは1件が残る
			// ケースだけを固定する。詳細は報告参照）
			name: "AC-3-9_removes_unexported_fields_and_methods",
			files: map[string]string{
				"hide/a.go": "package hide\n\n" +
					"type Box struct {\n" +
					"\tVal    int\n" +
					"\thidden string\n" +
					"}\n\n" +
					"type Reader interface {\n" +
					"\tRead(p []byte) (n int, err error)\n" +
					"\tprivateMethod()\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "hide", Kind: "type", Name: "Box", Signature: "struct { Val int }"},
				{Pkg: "hide", Kind: "type", Name: "Reader", Signature: "interface { Read(p []byte) (n int, err error) }"},
			},
		},
		{
			// AC-3-10: 埋め込みフィールドは、埋め込まれた型名（最終要素）
			// が公開なら残し、非公開なら除去する。
			name: "AC-3-10_embedded_field_visibility",
			files: map[string]string{
				"embed/a.go": "package embed\n\n" +
					"import \"io\"\n\n" +
					"type hiddenEmbed struct{}\n" +
					"type Pub struct{}\n\n" +
					"type Container struct {\n" +
					"\tio.Reader\n" +
					"\thiddenEmbed\n" +
					"\tPub\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "embed", Kind: "type", Name: "Pub", Signature: "struct{}"},
				{Pkg: "embed", Kind: "type", Name: "Container", Signature: "struct { io.Reader Pub }"},
			},
		},
		{
			// AC-3-3: ビルドタグを解釈しない。相反する //go:build を持つ
			// 2ファイルの両方が読まれ、同じキー（pkg+kind+name）でも
			// signature が違う2レコードとして両方現れる（一方を黙って
			// 採らない）。この結果を INDETERMINATE / DUPLICATE へ倒すのは
			// 比較器（.github/scripts/、shell）の仕事であり、
			// extractRecords 自身の仕事ではない（AC-1-3）。
			name: "AC-3-3_build_tags_not_interpreted",
			files: map[string]string{
				"tags/a.go": "//go:build linux\n\npackage tags\n\nfunc Dup(x int) {}\n",
				"tags/b.go": "//go:build windows\n\npackage tags\n\nfunc Dup() {}\n",
			},
			want: []record{
				{Pkg: "tags", Kind: "func", Name: "Dup", Signature: "(int) ()"},
				{Pkg: "tags", Kind: "func", Name: "Dup", Signature: "() ()"},
			},
		},
		{
			// AC-3-11: 依存解決・型解決を行わない。解決できないインポート
			// パスを型として参照していても、構文だけで抽出が成功する
			// （go/build や packages.Load を使っていれば、解決できない
			// インポートで失敗するはずの入力）。
			name: "AC-3-11_no_dependency_resolution",
			files: map[string]string{
				"unresolved/a.go": "package unresolved\n\n" +
					"import \"github.com/does-not-exist/whatever\"\n\n" +
					"var X whatever.Foo\n",
			},
			want: []record{
				{Pkg: "unresolved", Kind: "var", Name: "X", Signature: "whatever.Foo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeFixture(t, tt.files)

			got, err := extractRecords(dir)
			if err != nil {
				t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
			}

			if diff := cmp.Diff(
				tt.want, got,
				cmpopts.SortSlices(byRecord),
			); diff != "" {
				t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
			}
		})
	}
}

// TestExtractRecords_AC3_13_NoPackagesFound は AC-3-13 を固定する:
// 抽出対象の Go パッケージが1つも見つからない場合、0行（0件のレコード）を
// 返し、非ゼロ（extractRecords の戻り値としてはエラー）を返す。
// 「対象が無いので通った」を作らない。
//
// ここでの「対象が見つからない」は「ディレクトリ配下に *.go が1つも無い」
// という、AC-3-1 の除外境界に関わらない最も単純なケースに絞る（*.go は
// あるが全て除外規則に該当する場合の扱いは仕様の文言からは一意に定まらない
// ため、このテストには含めない。詳細は報告参照）。
//
// サブテスト packages_found_succeeds は対照ケースである。現在の
// extractRecords は常にエラーを返すスタブであり、no_packages_found_errors
// 単体は「エラーを返す」という絶対値の判定であっても、常時エラーの
// スタブと区別が付かず Red にならない（要求そのものが「エラーを返す
// こと」であるため）。対照ケースを同居させることで、本テスト全体は
// スタブに対して確実に Red になる。対照ケースは AC-3-13 の判定基準
// （0行・非ゼロ終了）を一切緩めない。
func TestExtractRecords_AC3_13_NoPackagesFound(t *testing.T) {
	t.Run("packages_found_succeeds", func(t *testing.T) {
		dir := writeFixture(t, map[string]string{
			"pkg/a.go": "package pkg\n\nfunc Marker() {}\n",
		})

		got, err := extractRecords(dir)
		if err != nil {
			t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
		}
		want := []record{
			{Pkg: "pkg", Kind: "func", Name: "Marker", Signature: "() ()"},
		}
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
			t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
		}
	})

	t.Run("no_packages_found_errors", func(t *testing.T) {
		dir := t.TempDir() // .go ファイルを1つも置かない

		got, err := extractRecords(dir)
		if err == nil {
			t.Fatalf(
				"extractRecords(%q) returned nil error, want non-nil"+
					"（AC-3-13: 抽出対象の Go パッケージが1つも見つからない場合は非ゼロで終了する）",
				dir,
			)
		}
		if len(got) != 0 {
			t.Errorf(
				"extractRecords(%q) returned %d records, want 0"+
					"（AC-3-13: 0行を出力する。詳細: %+v）",
				dir, len(got), got,
			)
		}
	})
}

// TestExtractRecords_AC3_14_ParseFailure は AC-3-14 を固定する:
// 構文解析に失敗したファイルがある場合、読めなかったファイルを黙って
// 飛ばさずエラーを返す。
//
// サブテスト parseable_succeeds は対照ケースである（理由は
// TestExtractRecords_AC3_13_NoPackagesFound の解説と同じ）。
func TestExtractRecords_AC3_14_ParseFailure(t *testing.T) {
	t.Run("parseable_succeeds", func(t *testing.T) {
		dir := writeFixture(t, map[string]string{
			"pkg/a.go": "package pkg\n\nfunc Marker() {}\n",
		})

		got, err := extractRecords(dir)
		if err != nil {
			t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
		}
		want := []record{
			{Pkg: "pkg", Kind: "func", Name: "Marker", Signature: "() ()"},
		}
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
			t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
		}
	})

	t.Run("broken_file_errors", func(t *testing.T) {
		dir := writeFixture(t, map[string]string{
			"broken/a.go": "package broken\n\nfunc Bad( {\n",
		})

		_, err := extractRecords(dir)
		if err == nil {
			t.Fatalf(
				"extractRecords(%q) returned nil error, want non-nil"+
					"（AC-3-14: 構文解析に失敗したファイルがあれば非ゼロで終了する）",
				dir,
			)
		}
	})
}
