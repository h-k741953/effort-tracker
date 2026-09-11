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
// extractRecords は extract.go に AC-3 の抽出規則そのものとして実装済み。
// 実装の詳細（AST を書き換えないコピー戦略を含む）は extract.go のコメント
// を見る。

import (
	"os"
	"path/filepath"
	"strings"
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
				{Pkg: "methods", Kind: "type", Name: "Pub", Signature: "struct { }"},
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
			// AC-3-6（Issue #93 reviewer C-1）: 「引数名・結果名・受信者
			// 変数名を出力しない」は、トップレベルの引数だけでなく
			// **関数型引数の内部**（`func(ctx int) error` のような、型式
			// として現れる別の *ast.FuncType）にも掛かる。トップレベルの
			// 引数名剥がしは AC-3-2 系のケースで既に固定済みだが、入れ子の
			// 位置は別経路（printer にそのまま渡す fieldListTypesString）
			// を通るため、独立したケースとして固定する。
			name: "AC-3-6_nested_func_type_arg_names_stripped",
			files: map[string]string{
				"nestedfunc/a.go": "package nestedfunc\n\n" +
					"func Run(f func(ctx int) error) {}\n",
			},
			want: []record{
				{Pkg: "nestedfunc", Kind: "func", Name: "Run", Signature: "(func(int) error) ()"},
			},
		},
		{
			// AC-3-6（Issue #93 reviewer C-1）: 引数名・結果名だけを変えた
			// 2つのソース（namesA / namesB）が同一の signature を出すことを
			// 固定する。相対比較（A の結果 == B の結果）だけにせず、期待値
			// そのもの（絶対値）も literal で固定する。相対一致だけを見る
			// テストは、実装が不在で両方とも同じ（誤った）値を返している
			// 場合にも緑になり得るため。
			//
			// 対象は2箇所: (1) 関数型引数の内部 `func(ctx int) error`、
			// (2) インターフェースのメソッド署名
			// `Read(p []byte) (n int, err error)`。
			name: "AC-3-6_arg_result_names_only_diff_yield_identical_signature",
			files: map[string]string{
				"namesA/a.go": "package namesA\n\n" +
					"func Run(f func(ctx int) error) {}\n\n" +
					"type R interface {\n" +
					"\tRead(p []byte) (n int, err error)\n" +
					"}\n",
				"namesB/a.go": "package namesB\n\n" +
					"func Run(f func(c int) error) {}\n\n" +
					"type R interface {\n" +
					"\tRead(buf []byte) (count int, e error)\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "namesA", Kind: "func", Name: "Run", Signature: "(func(int) error) ()"},
				{Pkg: "namesA", Kind: "type", Name: "R", Signature: "interface { Read([]byte) (int, error) }"},
				{Pkg: "namesB", Kind: "func", Name: "Run", Signature: "(func(int) error) ()"},
				{Pkg: "namesB", Kind: "type", Name: "R", Signature: "interface { Read([]byte) (int, error) }"},
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
			//
			// Grouped は「複数名をまとめた引数・結果宣言（`x, y int`）は
			// 名前の数だけ型を繰り返す」を固定する（Issue #93 reviewer
			// W-2）。引数が `f(a string)` → `f(a, b string)` のように
			// 増えたとき、型の重複展開が壊れていると引数の増加を
			// 検出できない偽陰性になるため、絶対値で固定する。
			name: "AC-3-8_parens_commas_variadic",
			files: map[string]string{
				"parens/a.go": "package parens\n\n" +
					"func Variadic(nums ...int) []int { return nums }\n\n" +
					"func NoArgsNoResults() {}\n\n" +
					"func OneResult() int { return 0 }\n\n" +
					"func Grouped(x, y int) (a, b error) { return nil, nil }\n",
			},
			want: []record{
				{Pkg: "parens", Kind: "func", Name: "Variadic", Signature: "(...int) ([]int)"},
				{Pkg: "parens", Kind: "func", Name: "NoArgsNoResults", Signature: "() ()"},
				{Pkg: "parens", Kind: "func", Name: "OneResult", Signature: "() (int)"},
				{Pkg: "parens", Kind: "func", Name: "Grouped", Signature: "(int, int) (error, error)"},
			},
		},
		{
			// AC-3-9: 構造体の非公開フィールド／インターフェースの
			// 非公開メソッドを signature から除去する。
			// （全メンバーが非公開になり0件へ落ちる境界は、別ケース
			// AC-3-9_empty_after_removing_unexported_members が covering
			// する。ここは1件が残るケースを固定する）
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
				// AC-3-6:「引数名・結果名・受信者変数名を出力しない」は
				// <signature> 全体に掛かる要求であり、型の右辺に現れる
				// インターフェースのメソッド署名も対象に含む
				// （Issue #93 reviewer C-1。名前だけを変えた2入力が
				// 同一の signature を出すことを担保するための固定値）。
				{Pkg: "hide", Kind: "type", Name: "Reader", Signature: "interface { Read([]byte) (int, error) }"},
			},
		},
		{
			// AC-3-9（境界）: 非公開メンバの除去によって全メンバーが
			// 無くなり0件へ落ちる型式は、ソースが元から空だった型と
			// 同じ表記（`struct { }` / `interface { }`）になる。
			//
			// AC-3-9 は「非公開メンバの増減は公開 API の変化ではない」
			// ことを担保する条文である。「元から空」と「除去後に空」を
			// 同一ケースに並置し、両者が同一の絶対値表記に揃うことを
			// 固定する。片方だけの表記変化（例: 除去後だけ
			// `struct{}` のように `struct` と `{` のあいだの空白が
			// 落ちる）を許すと、非公開フィールドの増減だけで
			// SIGNATURE_CHANGED が生じる偽陽性を許すことになり、
			// AC-3-9 の趣旨に反する。
			//
			// 綴りを `struct { }` 側（`struct` と `{` のあいだに空白が
			// 入る形）に採る根拠は
			// TestExtractRecords_AC3_9_SpellingIsIndependentOfRemovalCountAndLayout
			// の解説にある（メンバー0個には AC-3-9-2 (v) により区切りが
			// 現れないため、`struct { }` / `interface { }` が唯一の綴り
			// として定まる）。
			//
			// AC-3-10 の帰結（埋め込みフィールドが全部非公開で
			// 除去され空になる場合）も同じ趣旨で含める。
			name: "AC-3-9_empty_after_removing_unexported_members",
			files: map[string]string{
				"hideempty/a.go": "package hideempty\n\n" +
					"type OrigEmptyStruct struct{}\n" +
					"type OrigEmptyIface interface{}\n\n" +
					"type EmptiedStruct struct {\n" +
					"\thidden int\n" +
					"\tsecret string\n" +
					"}\n\n" +
					"type EmptiedIface interface {\n" +
					"\thiddenMethod() int\n" +
					"}\n\n" +
					"type hiddenEmbed2 struct{}\n\n" +
					"type EmptiedByEmbed struct {\n" +
					"\thiddenEmbed2\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "hideempty", Kind: "type", Name: "OrigEmptyStruct", Signature: "struct { }"},
				{Pkg: "hideempty", Kind: "type", Name: "OrigEmptyIface", Signature: "interface { }"},
				{Pkg: "hideempty", Kind: "type", Name: "EmptiedStruct", Signature: "struct { }"},
				{Pkg: "hideempty", Kind: "type", Name: "EmptiedIface", Signature: "interface { }"},
				{Pkg: "hideempty", Kind: "type", Name: "EmptiedByEmbed", Signature: "struct { }"},
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
				{Pkg: "embed", Kind: "type", Name: "Pub", Signature: "struct { }"},
				{Pkg: "embed", Kind: "type", Name: "Container", Signature: "struct { io.Reader; Pub }"},
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
		{
			// AC-3-6（Issue #93 reviewer C-1r-(a)）: var/const の型式は
			// extractGenDecl が vs.Type を名前剥がし（現 typeExprSignature）を通さず
			// そのまま printNode に渡している（extract.go:218 付近）。
			// func 型の var/const は、型式の内部に *ast.FuncType の
			// 引数名を含みうるため、そこが剥がされないままだと
			// 「引数名だけを変えた2入力が同一 signature を出す」
			// （AC-3-6）が var/const では成立しない。
			//
			// varfuncA / varfuncB は引数名（ctx / c）だけが異なる2入力。
			// 両方が同一の絶対値 "func(int) error" を出すことを固定する
			// （相対比較だけだと、両方とも剥がされていない誤った値
			// "func(ctx int) error" のままでも一致してしまい検出できない
			// ため、絶対値も併記する）。
			name: "AC-3-6_C1r_a_var_const_type_expr_strips_nested_func_arg_names",
			files: map[string]string{
				"varfuncA/a.go": "package varfuncA\n\n" +
					"var V func(ctx int) error\n" +
					"const C func(ctx int) error = nil\n",
				"varfuncB/a.go": "package varfuncB\n\n" +
					"var V func(c int) error\n" +
					"const C func(c int) error = nil\n",
			},
			want: []record{
				{Pkg: "varfuncA", Kind: "var", Name: "V", Signature: "func(int) error"},
				{Pkg: "varfuncA", Kind: "const", Name: "C", Signature: "func(int) error"},
				{Pkg: "varfuncB", Kind: "var", Name: "V", Signature: "func(int) error"},
				{Pkg: "varfuncB", Kind: "const", Name: "C", Signature: "func(int) error"},
			},
		},
		{
			// AC-3-6（Issue #93 reviewer C-1r-(b)）: 型パラメータの制約
			// （typeParamsString）は field.Type を名前剥がし（現 typeExprSignature）を
			// 通さず printNode に直接渡している。制約が関数型
			// （`F func(ctx int) error`）のとき、制約の**内側**の引数名
			// （ctx）が残ってしまう。
			//
			// 型パラメータ名そのもの（F）は落とさない（AC-9-7:
			// 型パラメータ名の変更は差分に出る、という要求と矛盾しない
			// ため）。落とすのは制約の内側の引数名だけ。
			//
			// typeparamA / typeparamB は制約内の引数名（ctx / c）だけが
			// 異なる2入力。両方が同一の絶対値
			// "[F func(int) error] (F) ()" を出すことを固定する。
			name: "AC-3-6_C1r_b_type_param_constraint_strips_nested_func_arg_names",
			files: map[string]string{
				"typeparamA/a.go": "package typeparamA\n\n" +
					"func Do[F func(ctx int) error](f F) {}\n",
				"typeparamB/a.go": "package typeparamB\n\n" +
					"func Do[F func(c int) error](f F) {}\n",
			},
			want: []record{
				{Pkg: "typeparamA", Kind: "func", Name: "Do", Signature: "[F func(int) error] (F) ()"},
				{Pkg: "typeparamB", Kind: "func", Name: "Do", Signature: "[F func(int) error] (F) ()"},
			},
		},
		{
			// AC-3-6（Issue #93 reviewer C-1r-(c)）: 型集合の union
			// （*ast.BinaryExpr）と `~T`（*ast.UnaryExpr）は
			// 名前剥がし（現 typeExprSignature）の switch に case が無く default で
			// そのまま（無変換で）返る。union の要素に関数型が現れると
			// （`interface{ ~int | func(ctx int) error }`）、その内側の
			// 引数名が剥がされない。
			//
			// unionfuncA / unionfuncB は union 内の関数型の引数名
			// （ctx / c）だけが異なる2入力。両方が同一の絶対値
			// "(interface { ~int | func(int) error }) ()" を出すことを
			// 固定する。
			name: "AC-3-6_C1r_c_union_and_tilde_strip_nested_func_arg_names",
			files: map[string]string{
				"unionfuncA/a.go": "package unionfuncA\n\n" +
					"func FUnion(x interface{ ~int | func(ctx int) error }) {}\n",
				"unionfuncB/a.go": "package unionfuncB\n\n" +
					"func FUnion(x interface{ ~int | func(c int) error }) {}\n",
			},
			want: []record{
				{Pkg: "unionfuncA", Kind: "func", Name: "FUnion", Signature: "(interface { ~int | func(int) error }) ()"},
				{Pkg: "unionfuncB", Kind: "func", Name: "FUnion", Signature: "(interface { ~int | func(int) error }) ()"},
			},
		},
		{
			// AC-3-9（Issue #93 reviewer C-2、最重要）: 当時のメンバー除去
			// （現 filterFieldList）は無名（埋め込み）フィールドを一律に
			// 埋め込み名で判定していたが、その判定（現 embeddableTypeName）は
			// *ast.BinaryExpr（union `~int | ~float64`）と
			// *ast.UnaryExpr（`~T`）に対して "" を返す。isExported("") は
			// false になるため、型集合の要素が丸ごと除去される。
			//
			// AC-3-9 が除去を許すのは「構造体の非公開フィールドと、
			// インターフェースの非公開メソッド」だけであり、AC-3-10 が
			// 除去を許すのは「埋め込みフィールド（埋め込まれた型名の
			// 公開性）」だけである。union 項・`~T` 項はどちらでもなく、
			// この除去を許す条文は無い。
			//
			// UnionKeepA: interface{ ~int | ~float64 } の signature を
			// 絶対値で固定する（union 項が消えず、丸ごと残ること）。
			name: "AC-3-9_C2_union_type_set_element_not_removed",
			files: map[string]string{
				"unionkeep/a.go": "package unionkeep\n\n" +
					"type Number interface{ ~int | ~float64 }\n",
			},
			want: []record{
				{Pkg: "unionkeep", Kind: "type", Name: "Number", Signature: "interface { ~int | ~float64 }"},
			},
		},
		{
			// AC-3-9（Issue #93 reviewer C-2）: `~T` 単独（*ast.UnaryExpr）
			// も同じ経路で除去される。単独ケースを union とは別に固定する。
			name: "AC-3-9_C2_tilde_only_type_set_element_not_removed",
			files: map[string]string{
				"tildekeep/a.go": "package tildekeep\n\n" +
					"type Other interface{ ~string }\n",
			},
			want: []record{
				{Pkg: "tildekeep", Kind: "type", Name: "Other", Signature: "interface { ~string }"},
			},
		},
		{
			// AC-3-9（Issue #93 reviewer C-2、偽 Green の直接の再発防止）:
			// オーケストレーターが実測した偽 Green ―― 公開制約を
			// `~int | ~float64` → `~string | ~bool` へ変える（破壊的な
			// 公開 API 変更）と、除去バグにより出力が完全に同一
			// （型集合が丸ごと消えた空のインターフェース）になり
			// 差分が出ない。
			//
			// unionfalsegreenA / unionfalsegreenB は型集合の中身だけが
			// 異なる2入力。**異なる signature を出す**ことを固定する
			// （両方とも同一の誤った値＝空のインターフェースを返す
			// 偽 Green の再発を防ぐ）。
			name: "AC-3-9_C2_different_type_sets_yield_different_signatures",
			files: map[string]string{
				"unionfalsegreenA/a.go": "package unionfalsegreenA\n\n" +
					"type WithConstraint interface{ ~int | ~float64 }\n",
				"unionfalsegreenB/a.go": "package unionfalsegreenB\n\n" +
					"type WithConstraint interface{ ~string | ~bool }\n",
			},
			want: []record{
				{Pkg: "unionfalsegreenA", Kind: "type", Name: "WithConstraint", Signature: "interface { ~int | ~float64 }"},
				{Pkg: "unionfalsegreenB", Kind: "type", Name: "WithConstraint", Signature: "interface { ~string | ~bool }"},
			},
		},
		{
			// AC-3-9（Issue #93 reviewer C-2、回帰）: union 項の保持
			// （AC-3-9_C2_*）が、既存の AC-3-9（非公開メソッドの除去）・
			// AC-3-10 の趣旨を壊していないことを、同一インターフェース内で
			// 両方が混在する形で固定する。
			// interface { hiddenMethod() int; ~int | ~string; Do() error }
			// では、非公開メソッド hiddenMethod は除去され、union
			// （~int | ~string）と公開メソッド Do は残る。
			name: "AC-3-9_C2_union_coexists_with_unexported_method_removal",
			files: map[string]string{
				"mixedunion/a.go": "package mixedunion\n\n" +
					"type Mixed interface {\n" +
					"\thiddenMethod() int\n" +
					"\t~int | ~string\n" +
					"\tDo() error\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "mixedunion", Kind: "type", Name: "Mixed", Signature: "interface { ~int | ~string; Do() error }"},
			},
		},
		{
			// AC-3-9（Issue #93 reviewer C-2、回帰）: union 項とメソッドが
			// 混在するとき、メソッドの引数名・結果名（AC-3-6）は従来どおり
			// 剥がされる。union の保持を実装するときにメソッド側の名前剥がし
			// を壊さないことを固定する。
			name: "AC-3-9_C2_union_coexists_with_method_arg_name_stripping",
			files: map[string]string{
				"unionmethod/a.go": "package unionmethod\n\n" +
					"type ConstraintWithMethod interface {\n" +
					"\t~int | ~string\n" +
					"\tDo() error\n" +
					"}\n",
			},
			want: []record{
				{Pkg: "unionmethod", Kind: "type", Name: "ConstraintWithMethod", Signature: "interface { ~int | ~string; Do() error }"},
			},
		},
		{
			// AC-3-9 / AC-3-10（Issue #93 reviewer W-2r）: 非公開メンバー
			// 除去（現 filterFieldList / typeExprSignature に統合）は type 宣言の右辺の
			// トップレベルにしか適用されておらず、入れ子の構造体・
			// インターフェース（フィールドの型として現れる無名の
			// struct{...} / interface{...}）には適用されない。AC-3-9 は
			// `<signature>` 全体に掛かる要求であり、「型宣言の右辺
			// トップレベルに限る」とは書かれていない。
			//
			// NestedStruct: フィールド Pub の型（無名 struct）の中の
			// hidden、フィールド Inner の型（無名 interface）の中の
			// secret() が、どちらも除去されることを固定する。
			name: "AC-3-9_W2r_nested_type_expr_removes_unexported_members",
			files: map[string]string{
				"nestedhide/a.go": "package nestedhide\n\n" +
					"type NestedStruct struct {\n" +
					"\tPub struct {\n" +
					"\t\thidden int\n" +
					"\t\tShown  int\n" +
					"\t}\n" +
					"\tInner interface {\n" +
					"\t\tsecret() int\n" +
					"\t\tPublic() int\n" +
					"\t}\n" +
					"}\n",
			},
			want: []record{
				{
					Pkg:  "nestedhide",
					Kind: "type",
					Name: "NestedStruct",
					Signature: "struct { Pub struct { Shown int }; " +
						"Inner interface { Public() int } }",
				},
			},
		},
		{
			// AC-3-9（Issue #93 reviewer W-2r）: 関数の引数型に現れる無名
			// 構造体（`struct{ hidden int; Fn func(int) error }`）の中の
			// 非公開フィールド hidden も除去されることを固定する。
			// あわせて、除去対象ではない Fn の型（AC-3-6 が既に対象とする
			// 関数型の引数名 ctx）が剥がれていることも同じケースで固定する。
			name: "AC-3-9_W2r_nested_struct_in_func_param_removes_unexported_field",
			files: map[string]string{
				"nestedparam/a.go": "package nestedparam\n\n" +
					"func H(x struct {\n" +
					"\thidden int\n" +
					"\tFn     func(ctx int) error\n" +
					"}) {}\n",
			},
			want: []record{
				{Pkg: "nestedparam", Kind: "func", Name: "H", Signature: "(struct { Fn func(int) error }) ()"},
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
// という、AC-3-1 の除外境界に関わらない最も単純なケースに絞る。「*.go は
// あるが全て除外規則に該当する場合」（例: ディレクトリの中身が
// *_test.go だけ）に AC-3-13 が適用されるかどうかは、AC-3-1 と AC-3-13 の
// 文言からは一意に定まらない。業務ルールの推測で埋めず、このテストの
// 対象には含めない。
//
// サブテスト packages_found_succeeds は対照ケースである。
// no_packages_found_errors 単体（「エラーを返す」という絶対値の判定）
// だけでは、実装を丸ごと「常にエラーを返す」に差し替えても Green のままに
// なり得る。対照ケースを同居させることで、「対象が見つかるときは成功し、
// 見つからないときだけエラーになる」という条件付きの振る舞いを固定する。
// 対照ケースは AC-3-13 の判定基準（0行・非ゼロ終了）を一切緩めない。
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

// TestExtractRecords_AC3_10_1_PredeclaredEmbeds は AC-3-10-1 を固定する:
// Go の定義済み（predeclared）型名は、埋め込みフィールド／埋め込み要素
// として現れた場合、公開として扱い <signature> に保持する。
//
// docs/specs/public-api-diff-check.md
// 「定義済み型名を埋め込みとして保持する理由と、対象範囲の決め方
// （AC-3-10-1 の根拠）」節の小節「3-10-1 が要求する期待値
// （テストに落とす形）」の表 (i)〜(vii) を漏れなく落とす。
//
// (vi)（io.Reader = 公開の修飾埋め込みは 3-10 のまま保持される）と
// (vii)（一覧の外側の非公開ローカル型は 3-10 のまま除去される）は
// 対照ケースであり、「保持されること」だけでなく「除去され続けること」も
// 併せて検査する（通る側だけを検査しない。AC-6-5 と同じ趣旨）。
//
// 期待する <signature> は go/printer の実際の出力を確認したうえでの
// 絶対値である。相対比較（embed 有無で結果が違う、というだけの判定）は
// 実装が両側とも同じ誤った値を返す偽 Green を検出できない
// （progress.md の失敗ログ #4 と同じ失敗モード）ため、絶対値の cmp.Diff
// による突き合わせを主とし、(i)〜(v)・(vii) については記録同士の
// バイト一致／不一致も明示的に確認する。
func TestExtractRecords_AC3_10_1_PredeclaredEmbeds(t *testing.T) {
	files := map[string]string{
		// (i): インターフェース埋め込みの `error`。
		"i_with/a.go":    "package i_with\n\ntype T interface { error; Code() int }\n",
		"i_without/a.go": "package i_without\n\ntype T interface { Code() int }\n",

		// (ii): インターフェース埋め込みの `comparable`。
		"ii_with/a.go":    "package ii_with\n\ntype T interface { comparable }\n",
		"ii_without/a.go": "package ii_without\n\ntype T interface{}\n",

		// (iii): インターフェース埋め込みの `any`。
		"iii_with/a.go":    "package iii_with\n\ntype T interface { any }\n",
		"iii_without/a.go": "package iii_without\n\ntype T interface{}\n",

		// (iv): 構造体埋め込みの `error`。
		"iv_with/a.go":    "package iv_with\n\ntype T struct { error; N int }\n",
		"iv_without/a.go": "package iv_without\n\ntype T struct { N int }\n",

		// (v): 構造体埋め込みの `int`。
		"v_with/a.go":    "package v_with\n\ntype T struct { int; M string }\n",
		"v_without/a.go": "package v_without\n\ntype T struct { M string }\n",

		// (vi) 対照: 公開の修飾埋め込み（io.Reader）は 3-10 のまま保持
		// される。本項（3-10-1）で振る舞いが変わらないこと。
		"vi/a.go": "package vi\n\nimport \"io\"\n\ntype T struct { io.Reader }\n",

		// (vii) 対照: 一覧の外側の小文字識別子（同パッケージの非公開型
		// helper）は 3-10 のまま除去され、embed の無い宣言とバイト一致
		// すること。
		"vii_with/a.go": "package vii_with\n\n" +
			"type helper struct{}\n\n" +
			"type T struct { helper; N int }\n",
		"vii_without/a.go": "package vii_without\n\ntype T struct { N int }\n",

		// 3-10-1 の対象は22個に限る（一覧の全要素）。代表例だけでは
		// 実装が一部（例: error / any / comparable の3つだけ）を許可
		// リストに入れて他を見逃しても Green になりうるため、22個
		// すべてをインターフェース埋め込みとして固定する。
		"allpredeclared_iface/a.go": "package allpredeclared_iface\n\n" +
			"type All interface {\n" +
			"\tany\n\tbool\n\tbyte\n\tcomparable\n\tcomplex64\n\tcomplex128\n\terror\n" +
			"\tfloat32\n\tfloat64\n\tint\n\tint8\n\tint16\n\tint32\n\tint64\n\trune\n" +
			"\tstring\n\tuint\n\tuint8\n\tuint16\n\tuint32\n\tuint64\n\tuintptr\n" +
			"}\n",

		// (viii) 対照: 適用条件 (a) のパッケージ修飾子を持たないこと。
		// io.error は綴りが一覧の定義済み型名 error と一致していても
		// 修飾子付きは対象外であり、3-10 のまま除去され、embed の無い
		// 宣言とバイト一致すること。
		"viii_with/a.go": "package viii_with\n\nimport \"io\"\n\n" +
			"type T struct { io.error; N int }\n",
		"viii_without/a.go": "package viii_without\n\ntype T struct { N int }\n",

		// (ix): 適用条件 (a) の `*T` の形でも識別子部分で判定すること。
		// *int は保持され、embed の無い宣言とバイト一致しないこと。
		"ix_with/a.go":    "package ix_with\n\ntype T struct { *int; M string }\n",
		"ix_without/a.go": "package ix_without\n\ntype T struct { M string }\n",

		// 同じ22個のうち `comparable` を除いた21個を構造体埋め込みとして
		// 固定する（`comparable` は構造体の埋め込みフィールドとしては
		// 非合法で `type S struct{ comparable }` はコンパイルできないため、
		// struct 側の網羅からは除く。オーケストレーターの指示どおり）。
		"allpredeclared_struct/a.go": "package allpredeclared_struct\n\n" +
			"type AllStruct struct {\n" +
			"\tany\n\tbool\n\tbyte\n\tcomplex64\n\tcomplex128\n\terror\n" +
			"\tfloat32\n\tfloat64\n\tint\n\tint8\n\tint16\n\tint32\n\tint64\n\trune\n" +
			"\tstring\n\tuint\n\tuint8\n\tuint16\n\tuint32\n\tuint64\n\tuintptr\n" +
			"}\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	want := []record{
		{Pkg: "i_with", Kind: "type", Name: "T", Signature: "interface { error; Code() int }"},
		{Pkg: "i_without", Kind: "type", Name: "T", Signature: "interface { Code() int }"},

		{Pkg: "ii_with", Kind: "type", Name: "T", Signature: "interface { comparable }"},
		{Pkg: "ii_without", Kind: "type", Name: "T", Signature: "interface { }"},

		{Pkg: "iii_with", Kind: "type", Name: "T", Signature: "interface { any }"},
		{Pkg: "iii_without", Kind: "type", Name: "T", Signature: "interface { }"},

		{Pkg: "iv_with", Kind: "type", Name: "T", Signature: "struct { error; N int }"},
		{Pkg: "iv_without", Kind: "type", Name: "T", Signature: "struct { N int }"},

		{Pkg: "v_with", Kind: "type", Name: "T", Signature: "struct { int; M string }"},
		{Pkg: "v_without", Kind: "type", Name: "T", Signature: "struct { M string }"},

		{Pkg: "vi", Kind: "type", Name: "T", Signature: "struct { io.Reader }"},

		{Pkg: "vii_with", Kind: "type", Name: "T", Signature: "struct { N int }"},
		{Pkg: "vii_without", Kind: "type", Name: "T", Signature: "struct { N int }"},

		{Pkg: "viii_with", Kind: "type", Name: "T", Signature: "struct { N int }"},
		{Pkg: "viii_without", Kind: "type", Name: "T", Signature: "struct { N int }"},

		{Pkg: "ix_with", Kind: "type", Name: "T", Signature: "struct { *int; M string }"},
		{Pkg: "ix_without", Kind: "type", Name: "T", Signature: "struct { M string }"},

		{
			Pkg: "allpredeclared_iface", Kind: "type", Name: "All",
			Signature: "interface { any; bool; byte; comparable; complex64; complex128; error; " +
				"float32; float64; int; int8; int16; int32; int64; rune; string; uint; uint8; " +
				"uint16; uint32; uint64; uintptr }",
		},
		{
			Pkg: "allpredeclared_struct", Kind: "type", Name: "AllStruct",
			Signature: "struct { any; bool; byte; complex64; complex128; error; " +
				"float32; float64; int; int8; int16; int32; int64; rune; string; uint; uint8; " +
				"uint16; uint32; uint64; uintptr }",
		},
	}

	if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
		t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
	}

	// 期待値の絶対値一致（上の cmp.Diff）に加えて、AC-3-10-1 の根拠節の
	// 表が直接要求する「バイト一致しない／する」を record 同士の比較でも
	// 明示的に確認する。絶対値だけでは、たまたま両方の期待値リテラルを
	// 見間違えて同じ値にしてしまった場合の書き損じを拾えないため。
	sig := func(t *testing.T, pkg string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == "type" && r.Name == "T" {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q, kind=type, name=T in %+v", pkg, got)
		return ""
	}

	pairs := []struct {
		label       string
		with        string
		without     string
		wantEqualBy bool // true なら (vii) のようにバイト一致することを要求
	}{
		{"i_error", "i_with", "i_without", false},
		{"ii_comparable", "ii_with", "ii_without", false},
		{"iii_any", "iii_with", "iii_without", false},
		{"iv_error_struct", "iv_with", "iv_without", false},
		{"v_int_struct", "v_with", "v_without", false},
		{"vii_local_unexported_helper", "vii_with", "vii_without", true},
		{"viii_qualified_io_error", "viii_with", "viii_without", true},
		{"ix_star_int", "ix_with", "ix_without", false},
	}
	for _, p := range pairs {
		t.Run(p.label, func(t *testing.T) {
			with := sig(t, p.with)
			without := sig(t, p.without)
			equal := with == without
			if equal != p.wantEqualBy {
				t.Errorf(
					"pkg %q signature=%q, pkg %q signature=%q: byte-equal=%v, want byte-equal=%v",
					p.with, with, p.without, without, equal, p.wantEqualBy,
				)
			}
		})
	}
}

// TestExtractRecords_AC3_9_SpellingIsIndependentOfRemovalCountAndLayout は、
// AC-3-9 が担保する「非公開メンバの増減は公開 API の変化ではない」を
// <signature> の**綴り**のレベルで固定する（Issue #93 reviewer 往復5 の指摘
// C-5-1）。
//
// 【何が壊れているか】
//
//	AC-3-9 の除去そのものは効いているが、除去後に go/printer が選ぶ表記が
//	(α) 除去後に残ったメンバー数（0個 / 1個 / 2個以上）と
//	(β) 元ソースが1行で書かれているか複数行で書かれているか
//	の2つに依存する。AC-4-3 は <signature> を**バイト単位の完全一致**で
//	比較するため、この揺れがそのまま偽の SIGNATURE_CHANGED になる。
//	（比較器側で空白を正規化して吸収することは AC-4-3 が明示的に禁じている。
//	正規化は抽出器の責務＝AC-3-7。）
//
// 【固定する不変条件】
//
//	**同じ公開 API を表す型は、(α)(β) がどうであっても同一の <signature> を
//	出す。** 「除去後に空になったときだけ元から空の型に合わせる」ような
//	場合分け（往復2 で入れた collapseIfEmpty の対症療法）は、(α) の 0個/1個
//	の境界しか塞がず、1個/2個以上の境界と (β) をそのまま残すため、この
//	不変条件を満たさない。
//
// 【期待する綴りが AC から一意に定まる理由（実装の出力に合わせたのではない）】
//
//	AC-3-7 が許すのは「go/printer の出力を用い、改行・タブ・連続空白を空白
//	1つへ畳み、前後の空白を落とす」ことだけである。空白を1つへ畳む操作は
//	空白の**有無**を変えられない（削除も挿入もしない）。したがって残った
//	メンバーが0個・1個のときの綴りは go/printer が出す形に一意に定まる。
//
//	  (1) `struct{ N int }`  … `struct` と `{` のあいだに空白が無い形。
//	      go/printer はソースの `{` と `}` が同じ行にあり、かつ残った
//	      フィールドが**ちょうど1個**（かつ短い）ときにだけこの形を出す。
//	  (2) `struct { N int }` … 空白が入る形。上記以外のすべてで出る。
//
//	0個のときは (2) の空の形（`struct { }`、インターフェースなら
//	`interface { }`）になる —— AC-3-9-2 (v) は「メンバー0個には区切りが
//	現れない」ことを要求しており、(1)(2) の分岐に区切りは関与しない。
//
//	**メンバーが2個以上残る型は別の規則で決まる。** AC-3-7 の畳み込み
//	だけを字面どおり適用すると、フィールド境界の改行も空白1つへ潰れて
//	しまい `struct { N int M string }` のように区切りの無い形になる
//	（(1)(2) はどちらもこの区切りを持たない）。AC-3-9-2 はこれを偽 Green
//	として明示的に禁じ、「構造体のフィールド間・インターフェースの
//	メンバー間には `<signature>` 上に区切りを出す」ことを要求する
//	（メンバー1個以下には区切りが現れないことも同項が対照として固定する
//	ため、(1)(2) の一致条件を変えない）。区切り文字の綴りそのものは
//	AC-3-7 / AC-3-9-2 のどちらも固定しないが、
//	TestExtractRecords_AC3_9_2_MemberBoundarySeparator が `"; "`
//	（セミコロン + 空白1つ）を絶対値として先に固定しているため、本テストも
//	これに揃える。
//
//	この導出の帰結として、本ファイルの既存ケースが持っていた
//	`struct{ N int M string }` のような区切り無しの絶対値は、メンバーが
//	2個以上残るケースに限り `"; "` 区切りへ改めた（0個・1個の綴りは
//	変わらない）。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_9_SpellingIsIndependentOfRemovalCountAndLayout(t *testing.T) {
	type variant struct {
		name string // パッケージ名の後半に使う（Go の識別子として有効な綴り）
		decl string // `type T` の右辺。ソースの改行・空白をそのまま与える
	}
	groups := []struct {
		name     string // パッケージ名の前半
		want     string // AC-3-7 から導いた <signature> の絶対値
		variants []variant
	}{
		{
			// 公開 API: 公開フィールドを1つも持たない構造体。
			name: "struct_zero",
			want: "struct { }",
			variants: []variant{
				{"orig_oneline", "struct{}"},
				{"orig_multiline", "struct {\n}"},
				{"removed1_oneline", "struct{ hidden int }"},
				{"removed1_multiline", "struct {\n\thidden int\n}"},
				{"removed2_oneline", "struct{ hidden int; secret string }"},
				{"removed2_multiline", "struct {\n\thidden int\n\tsecret string\n}"},
			},
		},
		{
			// 公開 API: 公開フィールド N int を1つだけ持つ構造体。
			name: "struct_one",
			want: "struct { N int }",
			variants: []variant{
				{"removed0_oneline", "struct{ N int }"},
				{"removed0_multiline", "struct {\n\tN int\n}"},
				{"removed1_oneline", "struct{ N int; hidden string }"},
				{"removed1_multiline", "struct {\n\tN int\n\thidden string\n}"},
				{"removed2_oneline", "struct{ hidden string; N int; secret bool }"},
				{"removed2_multiline", "struct {\n\thidden string\n\tN int\n\tsecret bool\n}"},
			},
		},
		{
			// 公開 API: 公開フィールド N int / M string を持つ構造体。
			// メンバーが2個残るため、AC-3-9-2 によりフィールド境界へ
			// 区切り（"; "）が入る。
			name: "struct_two",
			want: "struct { N int; M string }",
			variants: []variant{
				{"removed0_oneline", "struct{ N int; M string }"},
				{"removed0_multiline", "struct {\n\tN int\n\tM string\n}"},
				{"removed1_oneline", "struct{ N int; hidden bool; M string }"},
				{"removed1_multiline", "struct {\n\tN int\n\thidden bool\n\tM string\n}"},
			},
		},
		{
			// 公開 API: 公開メソッドを1つも持たないインターフェース。
			name: "iface_zero",
			want: "interface { }",
			variants: []variant{
				{"orig_oneline", "interface{}"},
				{"orig_multiline", "interface {\n}"},
				{"removed1_oneline", "interface{ hidden() int }"},
				{"removed1_multiline", "interface {\n\thidden() int\n}"},
				{"removed2_oneline", "interface{ hidden() int; secret() string }"},
				{"removed2_multiline", "interface {\n\thidden() int\n\tsecret() string\n}"},
			},
		},
		{
			// 公開 API: 公開メソッド Do() error を1つだけ持つインターフェース。
			name: "iface_one",
			want: "interface { Do() error }",
			variants: []variant{
				{"removed0_oneline", "interface{ Do() error }"},
				{"removed0_multiline", "interface {\n\tDo() error\n}"},
				{"removed1_oneline", "interface{ Do() error; hidden() int }"},
				{"removed1_multiline", "interface {\n\tDo() error\n\thidden() int\n}"},
				{"removed2_oneline", "interface{ hidden() int; Do() error; secret() string }"},
				{"removed2_multiline", "interface {\n\thidden() int\n\tDo() error\n\tsecret() string\n}"},
			},
		},
		{
			// 公開 API: 公開メソッド Do() error / Get() int を持つ
			// インターフェース（struct_two と同じく、メンバーが2個
			// 残るため AC-3-9-2 により区切りが入る）。
			name: "iface_two",
			want: "interface { Do() error; Get() int }",
			variants: []variant{
				{"removed0_oneline", "interface{ Do() error; Get() int }"},
				{"removed0_multiline", "interface {\n\tDo() error\n\tGet() int\n}"},
				{"removed1_oneline", "interface{ Do() error; hidden() bool; Get() int }"},
				{"removed1_multiline", "interface {\n\tDo() error\n\thidden() bool\n\tGet() int\n}"},
			},
		},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			files := make(map[string]string, len(g.variants))
			want := make([]record, 0, len(g.variants))
			for _, v := range g.variants {
				pkg := g.name + "_" + v.name
				files[pkg+"/a.go"] = "package " + pkg + "\n\ntype T " + v.decl + "\n"
				want = append(want, record{
					Pkg: pkg, Kind: "type", Name: "T", Signature: g.want,
				})
			}

			dir := writeFixture(t, files)
			got, err := extractRecords(dir)
			if err != nil {
				t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
			}

			// (1) 絶対値。相対比較だけにすると、全 variant が同じ誤った
			// 綴りを返す偽 Green を検出できない。
			if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
				t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
			}

			// (2) 不変条件そのもの。どの variant 同士もバイト一致すること
			// （AC-4-3 の比較はバイト単位の完全一致であり、部分一致・
			// 空白無視・正規表現へ緩めない）。基準は先頭の variant。
			sigByPkg := make(map[string]string, len(got))
			for _, r := range got {
				sigByPkg[r.Pkg] = r.Signature
			}
			basePkg := g.name + "_" + g.variants[0].name
			base, ok := sigByPkg[basePkg]
			if !ok {
				t.Fatalf("record not found for pkg %q in %+v", basePkg, got)
			}
			for _, v := range g.variants[1:] {
				pkg := g.name + "_" + v.name
				sig, ok := sigByPkg[pkg]
				if !ok {
					t.Fatalf("record not found for pkg %q in %+v", pkg, got)
				}
				if sig != base {
					t.Errorf(
						"pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true"+
							"（AC-3-9: 非公開メンバの増減と元ソースの改行位置は"+
							"公開 API の変化ではない。AC-4-3 はバイト単位で比較する）",
						basePkg, base, pkg, sig,
					)
				}
			}
		})
	}
}

// TestExtractRecords_AC3_7_NestedFuncTypeFieldListSpellingIsLayoutIndependent
// は、TestExtractRecords_AC3_9_SpellingIsIndependentOfRemovalCountAndLayout が
// 固定していなかった経路 —— stripFieldListNames（関数型の引数リスト・結果
// リストから名前を剥がす関数。extract.go の typeExprSignature の
// *ast.FuncType ケースから呼ばれる）を通る綴りの層依存性 —— を固定する。
//
// 【上の既存テストが固定していなかったもの】
//
//	既存テストの6グループはすべてトップレベルの struct / interface であり、
//	どの variant も入れ子の関数型（引数リスト・結果リストを持つ func 型）を
//	一度も含まない。そのため filterFieldList（構造体・インターフェース
//	自身のフィールドリスト）の Opening/Closing 位置落としは固定されて
//	いても、stripFieldListNames（関数型の引数リスト・結果リストの
//	Opening/Closing 位置落とし。extract.go 574行〜）は一度も経由されず、
//	そこにある同型のバグ（元ソースが複数行かつ末尾カンマを持つ引数／結果
//	リストのとき go/printer が "func( int, string, ) error" のような
//	別の綴りを出す）は既存テストでは検出できない。
//
// 【固定する不変条件】
//
//	stripFieldListNames を通る経路（引数リスト・結果リストの双方）は、
//	同じ公開 API を表す型なら、元ソースが1行で書かれているか・複数行＋
//	末尾カンマで書かれているかによらず同一の <signature> を出す。
//
// 【固定する経路】
//
//   - struct_field: 構造体フィールドに入れ子の関数型（引数リスト側）
//   - toplevel_var: トップレベル var 宣言の型が関数型そのもの
//     （引数リスト・結果リストの双方が stripFieldListNames を通る）
//   - iface_method: インターフェースのメソッド（引数リスト側）
//   - typeparam_constraint: 型パラメータ制約に現れる関数型（引数リスト側）
//   - results_side: 引数リストは1行のまま、結果リストだけを複数行＋
//     末尾カンマにする（結果リスト側が独立して stripFieldListNames を
//     通ることを、引数リスト側の layout に依存させず固定する）
//
// 【期待する綴りが AC から一意に定まる理由（実装の出力に合わせたのではない）】
//
//	AC-3-7 が許すのは「go/printer の出力を用い、改行・タブ・連続空白を
//	空白1つへ畳み、前後の空白を落とす」ことだけであり、空白の有無を
//	変えられない。入れ子の関数型は AC-3-8 の "(<引数型…>) (<結果型…>)"
//	という独自の組み立て（func/method kind 専用。fieldListTypesString /
//	funcTypeSignature）を経由せず、go/printer が *ast.FuncType /
//	フィールドをそのまま印字した結果を正規化するだけである
//	（extractGenDecl の var/const 行、typeParamsString、および
//	struct/interface の各フィールドの型は、いずれも
//	normalizeWhitespace(printNode(...)) を直接呼ぶ。AC-3-6 の表で
//	func/method 以外の行に "(<結果型…>)" の括弧強制が書かれていないのは
//	このため）。したがって go/printer が単一の無名結果を丸括弧なしで
//	印字する通常の Go 構文どおりの綴りになり、引数名・結果名は
//	AC-3-6 により剥がされる。改行・タブを空白1つへ畳む操作は
//	カンマの直後に残る空白の有無を変えないため、末尾カンマの直後に
//	残った空白が消えず "int, string, )" のような綴りになるかどうかが、
//	layout 依存のバグの有無を分ける。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_7_NestedFuncTypeFieldListSpellingIsLayoutIndependent(t *testing.T) {
	type flVariant struct {
		name string // パッケージ名の後半に使う
		src  string // package 行に続けてそのまま書く宣言ソース（改行・タブを含む）
	}
	groups := []struct {
		name     string // パッケージ名の前半
		kind     string
		ident    string // record.Name
		want     string // AC-3-6/3-7/3-8 から導いた <signature> の絶対値
		variants []flVariant
	}{
		{
			// 経路: struct field の入れ子関数型（引数リスト側）。
			name:  "struct_field",
			kind:  "type",
			ident: "T",
			want:  "struct { Cb func(int, string) error }",
			variants: []flVariant{
				{
					"oneline",
					"type T struct { Cb func(a int, b string) error }",
				},
				{
					"multiline_trailing_comma",
					"type T struct {\n\tCb func(\n\t\ta int,\n\t\tb string,\n\t) error\n}",
				},
			},
		},
		{
			// 経路: トップレベル var 宣言の型が関数型そのもの
			// （引数リスト・結果リストの双方が stripFieldListNames を通る）。
			name:  "toplevel_var",
			kind:  "var",
			ident: "V",
			want:  "func(int, string) (int, error)",
			variants: []flVariant{
				{
					"oneline",
					"var V func(a int, b string) (int, error)",
				},
				{
					"multiline_trailing_comma",
					"var V func(\n\ta int,\n\tb string,\n) (\n\tint,\n\terror,\n)",
				},
			},
		},
		{
			// 経路: インターフェースのメソッド（引数リスト側）。
			name:  "iface_method",
			kind:  "type",
			ident: "T",
			want:  "interface { Do(int, string) error }",
			variants: []flVariant{
				{
					"oneline",
					"type T interface { Do(a int, b string) error }",
				},
				{
					"multiline_trailing_comma",
					"type T interface {\n\tDo(\n\t\ta int,\n\t\tb string,\n\t) error\n}",
				},
			},
		},
		{
			// 経路: 型パラメータ制約に現れる関数型（引数リスト側）。
			name:  "typeparam_constraint",
			kind:  "type",
			ident: "T",
			want:  "[F func(int, string) error] struct { }",
			variants: []flVariant{
				{
					"oneline",
					"type T[F func(a int, b string) error] struct{}",
				},
				{
					"multiline_trailing_comma",
					"type T[F func(\n\ta int,\n\tb string,\n) error] struct{}",
				},
			},
		},
		{
			// 経路: 結果リスト側の独立検査。引数リストは1行のまま固定し、
			// 結果リストだけを複数行＋末尾カンマにする。
			name:  "results_side",
			kind:  "type",
			ident: "T",
			want:  "struct { Cb func(int) (int, string) }",
			variants: []flVariant{
				{
					"oneline",
					"type T struct { Cb func(a int) (int, string) }",
				},
				{
					"multiline_trailing_comma",
					"type T struct {\n\tCb func(a int) (\n\t\tint,\n\t\tstring,\n\t)\n}",
				},
			},
		},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			files := make(map[string]string, len(g.variants))
			want := make([]record, 0, len(g.variants))
			for _, v := range g.variants {
				pkg := g.name + "_" + v.name
				files[pkg+"/a.go"] = "package " + pkg + "\n\n" + v.src + "\n"
				want = append(want, record{
					Pkg: pkg, Kind: g.kind, Name: g.ident, Signature: g.want,
				})
			}

			dir := writeFixture(t, files)
			got, err := extractRecords(dir)
			if err != nil {
				t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
			}

			// (1) 絶対値。相対比較だけにすると、全 variant が同じ誤った
			// 綴りを返す偽 Green を検出できない。
			if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
				t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
			}

			// (2) 不変条件そのもの。oneline と multiline_trailing_comma が
			// バイト一致すること（AC-4-3 の比較はバイト単位の完全一致で
			// あり、部分一致・空白無視・正規表現へ緩めない）。
			sigByPkg := make(map[string]string, len(got))
			for _, r := range got {
				sigByPkg[r.Pkg] = r.Signature
			}
			basePkg := g.name + "_" + g.variants[0].name
			base, ok := sigByPkg[basePkg]
			if !ok {
				t.Fatalf("record not found for pkg %q in %+v", basePkg, got)
			}
			for _, v := range g.variants[1:] {
				pkg := g.name + "_" + v.name
				sig, ok := sigByPkg[pkg]
				if !ok {
					t.Fatalf("record not found for pkg %q in %+v", pkg, got)
				}
				if sig != base {
					t.Errorf(
						"pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true"+
							"（stripFieldListNames を通る経路も、元ソースの改行位置・末尾カンマの有無は"+
							"公開 API の変化ではない。AC-4-3 はバイト単位で比較する）",
						basePkg, base, pkg, sig,
					)
				}
			}
		})
	}
}

// TestExtractRecords_AC3_6_AC3_8_GroupedArgsExpandThroughNestedFuncTypePaths は
// Issue #93 reviewer 往復7 の指摘 C-7-1 を固定する。
//
// 【指摘の要旨】
//
//	複数名をまとめた引数・結果宣言（`a, b int`）を持つ関数型が、
//	stripFieldListNames（extract.go 574行〜）を通る位置 —— インターフェース
//	のメソッド引数、構造体フィールドの関数型の引数・結果、関数の引数位置に
//	現れる関数型、型パラメータ制約に現れる関数型 —— では、名前を剥がす
//	ときに Field 自体を名前の数だけ展開しない。結果、`A(x, y int) error` の
//	ような宣言から2番目以降の引数がまるごと消える（偽陰性）。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	AC-3-6 が出力しないと定めるのは「引数名・結果名・受信者変数名」で
//	あって「引数そのもの」ではない。AC-3-8 は「引数型・結果型はカンマ +
//	空白1つで区切る」と定め、n 個の引数は n 個の型として現れることを
//	要求している。この2つを組み合わせると、`a, b int` という1つの
//	フィールド宣言（名前2つ）は、<signature> 上の型の並びとしては
//	`int, int`（2個）でなければならない。トップレベル関数の引数
//	（fieldListTypesString 経由。extract_ac3_test.go の
//	AC-3-8_parens_commas_variadic ケース Grouped 参照）はこの導出どおりに
//	既に実装されており、本テストはその対照（既に正しい経路）と、
//	stripFieldListNames を通る各経路（壊れている経路）を同居させる。
//
// 【固定する経路】
//
//	interface メソッド引数（2名・3名）、構造体フィールドの関数型の引数、
//	同・結果、関数の引数位置に現れる関数型、型パラメータ制約に現れる
//	関数型、トップレベル関数（対照）。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_6_AC3_8_GroupedArgsExpandThroughNestedFuncTypePaths(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []record
	}{
		{
			// 経路: interface メソッドの引数リスト（2名のまとめ宣言）。
			name: "C-7-1_iface_method_two_names",
			files: map[string]string{
				"ac761ifc2/a.go": "package ac761ifc2\n\ntype Ifc interface{ A(x, y int) error }\n",
			},
			want: []record{
				{Pkg: "ac761ifc2", Kind: "type", Name: "Ifc", Signature: "interface { A(int, int) error }"},
			},
		},
		{
			// 経路: 同上、3名（個数で場合分けしないことの確認）。
			name: "C-7-1_iface_method_three_names",
			files: map[string]string{
				"ac761ifc3/a.go": "package ac761ifc3\n\ntype Ifc3 interface{ A(x, y, z int) error }\n",
			},
			want: []record{
				{Pkg: "ac761ifc3", Kind: "type", Name: "Ifc3", Signature: "interface { A(int, int, int) error }"},
			},
		},
		{
			// 経路: 構造体フィールドの関数型の引数リスト。
			name: "C-7-1_struct_field_functype_args",
			files: map[string]string{
				"ac761cbargs/a.go": "package ac761cbargs\n\ntype Cbs struct{ Cb func(a, b int) error }\n",
			},
			want: []record{
				{Pkg: "ac761cbargs", Kind: "type", Name: "Cbs", Signature: "struct { Cb func(int, int) error }"},
			},
		},
		{
			// 経路: 構造体フィールドの関数型の結果リスト。
			name: "C-7-1_struct_field_functype_results",
			files: map[string]string{
				"ac761cbres/a.go": "package ac761cbres\n\ntype Cbs struct{ Res func() (a, b error) }\n",
			},
			want: []record{
				{Pkg: "ac761cbres", Kind: "type", Name: "Cbs", Signature: "struct { Res func() (error, error) }"},
			},
		},
		{
			// 経路: 関数の引数位置に現れる関数型。
			name: "C-7-1_func_arg_position_functype",
			files: map[string]string{
				"ac761funcarg/a.go": "package ac761funcarg\n\nfunc F(g func(a, b int) error) {}\n",
			},
			want: []record{
				{Pkg: "ac761funcarg", Kind: "func", Name: "F", Signature: "(func(int, int) error) ()"},
			},
		},
		{
			// 経路: 型パラメータ制約に現れる関数型。
			name: "C-7-1_typeparam_constraint_functype",
			files: map[string]string{
				"ac761tparam/a.go": "package ac761tparam\n\ntype T[F func(a, b int) error] struct{}\n",
			},
			want: []record{
				{Pkg: "ac761tparam", Kind: "type", Name: "T", Signature: "[F func(int, int) error] struct { }"},
			},
		},
		{
			// 対照: トップレベル関数の引数は fieldListTypesString 経由で
			// 既に正しく展開される（stripFieldListNames を通らない）。
			// 修正がこちらを逆方向へ壊さないことの守り。
			name: "C-7-1_toplevel_control",
			files: map[string]string{
				"ac761top/a.go": "package ac761top\n\nfunc Top(a, b int) error { return nil }\n",
			},
			want: []record{
				{Pkg: "ac761top", Kind: "func", Name: "Top", Signature: "(int, int) (error)"},
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

// TestExtractRecords_AC3_7_1_IndexListLayoutSpellingIsInvariant は Issue #93
// reviewer 往復7 の指摘 C-7-2 を固定する。AC-3-7-1 の「3-7-1 が要求する
// 期待値（テストに落とす形）」表 (i)〜(vi) をそのままテーブルへ落とす。
//
// 【指摘の要旨】
//
//	型引数2個以上の実体化（`G[int, string]`。*ast.IndexListExpr）は、
//	typeExprSignature の IndexListExpr ケースが Lbrack/Rbrack の位置を
//	token.NoPos へ落とさない。struct/interface/関数型の引数・結果リスト
//	（FieldList の Opening/Closing。filterFieldList / stripFieldListNames）
//	は既にこれを落としているのに対し、IndexListExpr だけがこの扱いから
//	漏れている。そのため元ソースの行取り（1行 vs 複数行 + 末尾カンマ）で
//	go/printer の出力が変わり、AC-4-3 のバイト単位比較のもとで偽の
//	SIGNATURE_CHANGED を生む。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	AC-3-7-1 は「同じ宣言を1行で書いたものと、複数行に分けて末尾カンマを
//	置いて書いたものは、<signature> がバイト一致する」ことを「型式が現れる
//	すべての位置へ一様に掛かる」要求として定め、型引数の実体化を明示的に
//	列挙している。要素の個数で場合分けしないため、型引数1個
//	（*ast.IndexExpr。表(i)の対象は型引数「2個以上」の実体化であり、1個の
//	IndexExpr は表(i)の対象外）はこの壊れ方の対象外であることを対照として
//	同居させる。一方、複数行 + 末尾カンマ無し（型引数2個以上）は表(i)の
//	対象そのものであり、対照ではなく壊れ方の別 layout 変種として同居させる
//	（実測では Lbrack/Rbrack の行番号が異なるだけで go/printer の出力が
//	変わるため、末尾カンマの有無を問わず複数行なら影響を受ける）。
//	表(i)の直後の解説「固定するのは『行取りの違う2つの綴りがバイト一致
//	すること』だけであり、綴りそのものは固定しない」を踏まえ、絶対値は
//	AC-3-6/AC-3-7/AC-3-8 から導出できる形（既存テストの綴りの基準点と
//	同じ規則）を用い、baseline と layout variant のバイト一致を別途検査
//	する。
//
// 【固定する表の行】
//
//	(i) 型引数の実体化（type alias 経由。末尾カンマ有り・無しの両方の
//	複数行、および実際の壊れ方が構造体フィールドの入れ子
//	`Inner G[...]` でも再現することを別グループで固定し、型引数1個を
//	対照として固定する）
//	(ii) struct フィールドの分割 (iii) interface メソッドの分割
//	(iv) 関数の引数リスト (v) struct フィールドの関数型（入れ子）
//	(vi) 型パラメータリスト。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_7_1_IndexListLayoutSpellingIsInvariant(t *testing.T) {
	type variant struct {
		name string
		src  string // package 宣言に続けてそのまま書くソース全体（改行・タブを含む）
	}
	groups := []struct {
		name     string
		kind     string
		ident    string
		want     string // AC-3-6/3-7/3-8 から導いた <signature> の絶対値
		variants []variant
	}{
		{
			// (i): 型引数2個の実体化（type alias 経由）。
			name:  "i_type_alias_two_typeargs",
			kind:  "type",
			ident: "T",
			want:  "= G[int, string]",
			variants: []variant{
				{
					"oneline",
					"type G[K any, V any] struct{}\n\ntype T = G[int, string]\n",
				},
				{
					"multiline_trailing_comma",
					"type G[K any, V any] struct{}\n\ntype T = G[\n\tint,\n\tstring,\n]\n",
				},
			},
		},
		{
			// (i) の対照: 型引数1個（*ast.IndexExpr）は影響を受けない。
			name:  "i_control_single_typearg",
			kind:  "type",
			ident: "T",
			want:  "= G1[int]",
			variants: []variant{
				{
					"oneline",
					"type G1[K any] struct{}\n\ntype T = G1[int]\n",
				},
				{
					"multiline_trailing_comma",
					"type G1[K any] struct{}\n\ntype T = G1[\n\tint,\n]\n",
				},
			},
		},
		{
			// (i) の別 layout 変種: 複数行だが末尾カンマが無い形（閉じ括弧を
			// 最後の型引数と同じ行に置く。閉じ括弧を独立した行へ置いて
			// 末尾カンマを省くと "missing ',' before newline in type
			// argument list" で構文解析に失敗するため、書ける形はこれに
			// 限られる）。AC-3-7-1 は「型引数の実体化」へ一様に掛かるため、
			// 末尾カンマの有無に関わらずバイト一致を要求する。
			name:  "i_no_trailing_comma_multiline",
			kind:  "type",
			ident: "T",
			want:  "= G[int, string]",
			variants: []variant{
				{
					"oneline",
					"type G[K any, V any] struct{}\n\ntype T = G[int, string]\n",
				},
				{
					"multiline_no_trailing_comma",
					"type G[K any, V any] struct{}\n\ntype T = G[\n\tint,\n\tstring]\n",
				},
			},
		},
		{
			// (i) が構造体フィールドの入れ子でも再現することの固定。
			name:  "i_struct_field_nested_indexlist",
			kind:  "type",
			ident: "T",
			want:  "struct { Inner G[int, string] }",
			variants: []variant{
				{
					"oneline",
					"type G[K any, V any] struct{}\n\ntype T struct { Inner G[int, string] }\n",
				},
				{
					"multiline_trailing_comma",
					"type G[K any, V any] struct{}\n\ntype T struct {\n\tInner G[\n\t\tint,\n\t\tstring,\n\t]\n}\n",
				},
			},
		},
		{
			// (ii): struct フィールドの分割。
			name:  "ii_struct_fields_split",
			kind:  "type",
			ident: "T",
			want:  "struct { A int; B int }",
			variants: []variant{
				{"oneline", "type T struct { A int; B int }\n"},
				{"multiline", "type T struct {\n\tA int\n\tB int\n}\n"},
			},
		},
		{
			// (iii): interface メソッドの分割。
			name:  "iii_iface_methods_split",
			kind:  "type",
			ident: "T",
			want:  "interface { A() error; B() error }",
			variants: []variant{
				{"oneline", "type T interface { A() error; B() error }\n"},
				{"multiline", "type T interface {\n\tA() error\n\tB() error\n}\n"},
			},
		},
		{
			// (iv): 関数の引数リスト。
			name:  "iv_func_args_split",
			kind:  "func",
			ident: "F",
			want:  "(int, string) (error)",
			variants: []variant{
				{"oneline", "func F(a int, b string) error { return nil }\n"},
				{
					"multiline_trailing_comma",
					"func F(\n\ta int,\n\tb string,\n) error {\n\treturn nil\n}\n",
				},
			},
		},
		{
			// (v): struct フィールドの関数型（入れ子の関数型引数リスト）。
			name:  "v_struct_field_functype_split",
			kind:  "type",
			ident: "T",
			want:  "struct { F func(int, string) error }",
			variants: []variant{
				{"oneline", "type T struct { F func(a int, b string) error }\n"},
				{
					"multiline_trailing_comma",
					"type T struct {\n\tF func(\n\t\ta int,\n\t\tb string,\n\t) error\n}\n",
				},
			},
		},
		{
			// (vi): 型パラメータリスト。
			name:  "vi_typeparam_list_split",
			kind:  "func",
			ident: "F",
			want:  "[T any, U any] (T) (error)",
			variants: []variant{
				{"oneline", "func F[T any, U any](t T) error { panic(\"x\") }\n"},
				{
					"multiline_trailing_comma",
					"func F[\n\tT any,\n\tU any,\n](t T) error {\n\tpanic(\"x\")\n}\n",
				},
			},
		},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			files := make(map[string]string, len(g.variants))
			want := make([]record, 0, len(g.variants))
			for _, v := range g.variants {
				pkg := g.name + "_" + v.name
				files[pkg+"/a.go"] = "package " + pkg + "\n\n" + v.src
				want = append(want, record{
					Pkg: pkg, Kind: g.kind, Name: g.ident, Signature: g.want,
				})
			}

			dir := writeFixture(t, files)
			got, err := extractRecords(dir)
			if err != nil {
				t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
			}

			// (1) 絶対値。相対比較だけにすると、全 variant が同じ誤った
			// 綴りを返す偽 Green を検出できない。
			//
			// (kind/ident でフィルタせず全レコードと比較すると、各 variant
			// のソースが含む型 G / G1 のレコードまで比較対象へ紛れ込む。
			// G / G1 は group ごとに固定の <signature> を持つが want には
			// 含めていないため、比較対象は g.kind/g.ident に一致する
			// レコードだけへ絞る。)
			var filtered []record
			for _, r := range got {
				if r.Kind == g.kind && r.Name == g.ident {
					filtered = append(filtered, r)
				}
			}
			if diff := cmp.Diff(want, filtered, cmpopts.SortSlices(byRecord)); diff != "" {
				t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
			}

			// (2) 不変条件そのもの。oneline と layout variant がバイト一致
			// すること（AC-4-3 の比較はバイト単位の完全一致であり、部分
			// 一致・空白無視・正規表現へ緩めない）。基準は先頭の variant。
			sigByPkg := make(map[string]string, len(filtered))
			for _, r := range filtered {
				sigByPkg[r.Pkg] = r.Signature
			}
			basePkg := g.name + "_" + g.variants[0].name
			base, ok := sigByPkg[basePkg]
			if !ok {
				t.Fatalf("record not found for pkg %q (kind=%q name=%q) in %+v", basePkg, g.kind, g.ident, got)
			}
			for _, v := range g.variants[1:] {
				pkg := g.name + "_" + v.name
				sig, ok := sigByPkg[pkg]
				if !ok {
					t.Fatalf("record not found for pkg %q (kind=%q name=%q) in %+v", pkg, g.kind, g.ident, got)
				}
				if sig != base {
					t.Errorf(
						"pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true"+
							"（AC-3-7-1: 元ソースの行取りと末尾カンマの有無は"+
							"公開 API の変化ではない。AC-4-3 はバイト単位で比較する）",
						basePkg, base, pkg, sig,
					)
				}
			}
		})
	}
}

// TestExtractRecords_AC3_9_1_GroupedFieldDeclsExpandAndPreserveNames は
// Issue #93 reviewer 往復7 の指摘 W-7-1 を固定する。AC-3-9-1 の「3-9-1 が
// 要求する期待値（テストに落とす形）」表 (i)〜(vi) をそのままテーブルへ
// 落とす。
//
// 【指摘の要旨】
//
//	構造体フィールドのまとめ宣言（`A, B int`）は、filterFieldList が
//	非公開名を除去した後もそのまま1つの Field として残す（フィールド単位
//	への展開をしない）。分割宣言（`A int` / `B int`）は最初から2つの Field
//	であるため、両者の <signature> はバイト一致しない —— AC-3-9-1 が要求
//	する「n 個の名前は n 個のフィールドとして現れる」に反する。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	AC-3-9-1 は「1つのフィールド宣言が複数の名前を持つ場合、<signature>
//	にはその型が名前の数だけ現れ、名前ごとに分けて宣言した形とバイト一致
//	する」と明示し、直後の期待値表 (i)〜(vi) がテストに落とす形をそのまま
//	定める。**フィールド名は落とさない**ことも同項が明示しており、(iii) は
//	これを「バイト一致しないこと」で固定する。絶対値の綴り
//	（`struct { A int; B int }` の形）は
//	TestExtractRecords_AC3_9_SpellingIsIndependentOfRemovalCountAndLayout /
//	TestExtractRecords_AC3_10_1_PredeclaredEmbeds /
//	TestExtractRecords_AC3_9_2_MemberBoundarySeparator が固定した
//	「メンバーが2個以上残るときは AC-3-9-2 によりフィールド境界へ `"; "`
//	区切りが入る」規則と同一のものを流用する（新しい綴りを発明しない）。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_9_1_GroupedFieldDeclsExpandAndPreserveNames(t *testing.T) {
	files := map[string]string{
		// (i): 2名のまとめ宣言と分割宣言はバイト一致する。
		"w71_i_grouped/a.go": "package w71_i_grouped\n\ntype T struct { A, B int }\n",
		"w71_i_split/a.go":   "package w71_i_split\n\ntype T struct { A int; B int }\n",

		// (ii): 3名でも同じ（個数で場合分けしない）。
		"w71_ii_grouped/a.go": "package w71_ii_grouped\n\ntype T struct { A, B, C int }\n",
		"w71_ii_split/a.go":   "package w71_ii_split\n\ntype T struct { A int; B int; C int }\n",

		// (iii): フィールド名を落とさない —— 名前が違えばバイト一致しない。
		"w71_iii_ab/a.go": "package w71_iii_ab\n\ntype T struct { A, B int }\n",
		"w71_iii_ac/a.go": "package w71_iii_ac\n\ntype T struct { A, C int }\n",

		// (iv): 3-9 を先に適用すること —— 非公開の b は除去され、単独宣言
		// とバイト一致する。
		"w71_iv_grouped/a.go": "package w71_iv_grouped\n\ntype T struct { A, b int }\n",
		"w71_iv_single/a.go":  "package w71_iv_single\n\ntype T struct { A int }\n",

		// (v): まとめ宣言の名前がすべて非公開なら、そのフィールド宣言ごと
		// 除去される。
		"w71_v_grouped/a.go": "package w71_v_grouped\n\ntype T struct { a, b int }\n",
		"w71_v_empty/a.go":   "package w71_v_empty\n\ntype T struct{}\n",

		// (vi) 対照: 埋め込みフィールドは名前を持たないため本項の対象外。
		// 3-10-1 のまま保持され、本項によって振る舞いが変わらないこと。
		"w71_vi_embed/a.go": "package w71_vi_embed\n\ntype T struct { error; N int }\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	want := []record{
		{Pkg: "w71_i_grouped", Kind: "type", Name: "T", Signature: "struct { A int; B int }"},
		{Pkg: "w71_i_split", Kind: "type", Name: "T", Signature: "struct { A int; B int }"},

		{Pkg: "w71_ii_grouped", Kind: "type", Name: "T", Signature: "struct { A int; B int; C int }"},
		{Pkg: "w71_ii_split", Kind: "type", Name: "T", Signature: "struct { A int; B int; C int }"},

		{Pkg: "w71_iii_ab", Kind: "type", Name: "T", Signature: "struct { A int; B int }"},
		{Pkg: "w71_iii_ac", Kind: "type", Name: "T", Signature: "struct { A int; C int }"},

		{Pkg: "w71_iv_grouped", Kind: "type", Name: "T", Signature: "struct { A int }"},
		{Pkg: "w71_iv_single", Kind: "type", Name: "T", Signature: "struct { A int }"},

		{Pkg: "w71_v_grouped", Kind: "type", Name: "T", Signature: "struct { }"},
		{Pkg: "w71_v_empty", Kind: "type", Name: "T", Signature: "struct { }"},

		{Pkg: "w71_vi_embed", Kind: "type", Name: "T", Signature: "struct { error; N int }"},
	}

	if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
		t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
	}

	// 絶対値の一致（上の cmp.Diff）に加えて、AC-3-9-1 の期待値表が直接
	// 要求する「バイト一致する／しない」を record 同士の比較でも明示的に
	// 確認する（期待値リテラルの書き損じを絶対値だけでは拾えないため。
	// TestExtractRecords_AC3_10_1_PredeclaredEmbeds と同じ作法）。
	sig := func(t *testing.T, pkg string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == "type" && r.Name == "T" {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q in %+v", pkg, got)
		return ""
	}

	equalPairs := []struct{ a, b, label string }{
		{"w71_i_grouped", "w71_i_split", "(i) grouped == split (2 names)"},
		{"w71_ii_grouped", "w71_ii_split", "(ii) grouped == split (3 names)"},
		{"w71_iv_grouped", "w71_iv_single", "(iv) 非公開除去後は単独宣言と一致"},
		{"w71_v_grouped", "w71_v_empty", "(v) 全員非公開ならフィールド宣言ごと除去"},
	}
	for _, p := range equalPairs {
		a, b := sig(t, p.a), sig(t, p.b)
		if a != b {
			t.Errorf(
				"%s: pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true",
				p.label, p.a, a, p.b, b,
			)
		}
	}

	// (iii): フィールド名を落とさないこと —— 名前が違えばバイト一致しない。
	if ab, ac := sig(t, "w71_iii_ab"), sig(t, "w71_iii_ac"); ab == ac {
		t.Errorf(
			"(iii) pkg %q signature=%q, pkg %q signature=%q: byte-equal=true, want byte-equal=false"+
				"（AC-3-9-1: フィールド名は落とさない。名前が違えば <signature> は"+
				"バイト一致しない）",
			"w71_iii_ab", ab, "w71_iii_ac", ac,
		)
	}
}

// TestExtractRecords_AC3_7_1_QualifiedIdentLineBreakIsInvariant は Issue #93
// レビュー往復8 の指摘 C-8-1 を固定する。AC-3-7-1 の期待値表 (vii)〜(x) を
// そのままテーブルへ落とす。
//
// 【指摘の要旨】
//
//	修飾識別子（`pkg.Name`）を `.` の直後で改行すると（Go の自動セミコロン
//	挿入規則では `.` の直後に改行を入れても文が終わらないため、これは合法な
//	書き方であり、gofmt もこの行取りを保存する —— 本テストの fixture は
//	すべて gofmt 済みであることを別途確認している）、go/printer は
//	SelectorExpr にも行ベースの分岐を持ち、Sel の元の行が `.` より後ろに
//	あると改行を入れて印字する。構造体フィールド・トップレベル var・
//	interface メソッドの引数と結果・関数の引数と結果のいずれの位置でも
//	再現し、oneline 版と byte 不一致になる（偽の SIGNATURE_CHANGED）。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	AC-3-7-1 は「本項は型式が現れるすべての位置へ一様に掛かる」ものとして
//	「修飾識別子（pkg.Name）の内部」を明示的に列挙し、直後の期待値表
//	(vii)〜(x) がテストに落とす形をそのまま定める。各 want は AC-3-6/
//	AC-3-7/AC-3-8 の組み立て規則（struct/interface/var/func の <signature>
//	の形）から導出でき、実測上は oneline 版（バグの影響を受けない）が
//	その導出結果と一致することを (1) の cmp.Diff で確認したうえで、
//	linebreak 版が同じ絶対値とバイト一致することを (2) で確認する
//	（TestExtractRecords_AC3_7_1_IndexListLayoutSpellingIsInvariant と
//	同じ流儀）。
//
// 【固定する表の行】
//
//	(vii) 構造体フィールドの修飾識別子 (viii) トップレベル var の修飾識別子
//	(ix) interface メソッドの引数・結果の修飾識別子（両方を1ケースで同時に
//	改行する） (x) 関数の引数・結果の修飾識別子（同上）。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_7_1_QualifiedIdentLineBreakIsInvariant(t *testing.T) {
	type variant struct {
		name string
		src  string // package 宣言に続けてそのまま書くソース全体（改行・タブを含む）
	}
	groups := []struct {
		name     string
		kind     string
		ident    string
		want     string // AC-3-6/AC-3-7/AC-3-8 から導いた <signature> の絶対値
		variants []variant
	}{
		{
			// (vii): 構造体フィールドの修飾識別子。
			name:  "vii_struct_field",
			kind:  "type",
			ident: "T",
			want:  "struct { Src io.Reader }",
			variants: []variant{
				{"oneline", "import \"io\"\n\ntype T struct {\n\tSrc io.Reader\n}\n"},
				{"linebreak", "import \"io\"\n\ntype T struct {\n\tSrc io.\n\t\tReader\n}\n"},
			},
		},
		{
			// (viii): トップレベル var の修飾識別子。
			name:  "viii_toplevel_var",
			kind:  "var",
			ident: "V",
			want:  "io.Reader",
			variants: []variant{
				{"oneline", "import \"io\"\n\nvar V io.Reader\n"},
				{"linebreak", "import \"io\"\n\nvar V io.\n\tReader\n"},
			},
		},
		{
			// (ix): interface メソッドの引数・結果の修飾識別子。
			name:  "ix_iface_method",
			kind:  "type",
			ident: "T",
			want:  "interface { M(io.Reader) io.Writer }",
			variants: []variant{
				{"oneline", "import \"io\"\n\ntype T interface {\n\tM(r io.Reader) io.Writer\n}\n"},
				{
					"linebreak",
					"import \"io\"\n\ntype T interface {\n\tM(r io.\n\t\tReader) io.\n\t\tWriter\n}\n",
				},
			},
		},
		{
			// (x): 関数の引数・結果の修飾識別子。
			name:  "x_func",
			kind:  "func",
			ident: "F",
			want:  "(io.Reader) (io.Writer)",
			variants: []variant{
				{"oneline", "import \"io\"\n\nfunc F(r io.Reader) io.Writer {\n\treturn nil\n}\n"},
				{
					"linebreak",
					"import \"io\"\n\nfunc F(r io.\n\tReader) io.\n\tWriter {\n\treturn nil\n}\n",
				},
			},
		},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			files := make(map[string]string, len(g.variants))
			want := make([]record, 0, len(g.variants))
			for _, v := range g.variants {
				pkg := g.name + "_" + v.name
				files[pkg+"/a.go"] = "package " + pkg + "\n\n" + v.src
				want = append(want, record{
					Pkg: pkg, Kind: g.kind, Name: g.ident, Signature: g.want,
				})
			}

			dir := writeFixture(t, files)
			got, err := extractRecords(dir)
			if err != nil {
				t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
			}

			var filtered []record
			for _, r := range got {
				if r.Kind == g.kind && r.Name == g.ident {
					filtered = append(filtered, r)
				}
			}

			// (1) 絶対値。相対比較だけにすると、全 variant が同じ誤った
			// 綴りを返す偽 Green を検出できない。
			if diff := cmp.Diff(want, filtered, cmpopts.SortSlices(byRecord)); diff != "" {
				t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
			}

			// (2) 不変条件そのもの。oneline と linebreak がバイト一致すること
			// （AC-4-3 の比較はバイト単位の完全一致であり、部分一致・空白
			// 無視・正規表現へ緩めない）。基準は oneline 版。
			sigByPkg := make(map[string]string, len(filtered))
			for _, r := range filtered {
				sigByPkg[r.Pkg] = r.Signature
			}
			basePkg := g.name + "_" + g.variants[0].name
			base, ok := sigByPkg[basePkg]
			if !ok {
				t.Fatalf("record not found for pkg %q (kind=%q name=%q) in %+v", basePkg, g.kind, g.ident, got)
			}
			for _, v := range g.variants[1:] {
				pkg := g.name + "_" + v.name
				sig, ok := sigByPkg[pkg]
				if !ok {
					t.Fatalf("record not found for pkg %q (kind=%q name=%q) in %+v", pkg, g.kind, g.ident, got)
				}
				if sig != base {
					t.Errorf(
						"pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true"+
							"（AC-3-7-1: 修飾識別子の内部で改行しても "+
							"<signature> は公開 API の変化ではない。AC-4-3 はバイト単位で比較する）",
						basePkg, base, pkg, sig,
					)
				}
			}
		})
	}
}

// TestExtractRecords_AC3_7_1_ArrayLenCompositeLitLineBreakIsInvariant は
// Issue #93 レビュー往復8 の指摘 I-8-1 を固定する。AC-3-7-1 の期待値表 (xi)
// をそのままテーブルへ落とす。
//
// 【指摘の要旨】
//
//	型式の内部に現れる式（配列長の複合リテラル `[len([...]int{1, 2, 3})]int`）
//	を要素ごとに改行し末尾カンマを置くと、oneline 版と byte 不一致になる
//	（偽の SIGNATURE_CHANGED）。AC-3-7-1 は「型式の内部に現れる式（配列長
//	など）の内部」へも一様に掛かると明示している。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	want は AC-3-8 の「カンマ + 空白1つで区切る」規則から導出でき、oneline
//	版（バグの影響を受けない）がその導出結果と一致することを (1) の
//	cmp.Diff で確認したうえで、multiline 版が同じ絶対値とバイト一致する
//	ことを (2) で確認する。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_7_1_ArrayLenCompositeLitLineBreakIsInvariant(t *testing.T) {
	files := map[string]string{
		"xi_oneline/a.go": "package xi_oneline\n\ntype T [len([...]int{1, 2, 3})]int\n",
		"xi_multiline_trailing_comma/a.go": "package xi_multiline_trailing_comma\n\n" +
			"type T [len([...]int{\n\t1,\n\t2,\n\t3,\n})]int\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	want := []record{
		{Pkg: "xi_oneline", Kind: "type", Name: "T", Signature: "[len([...]int{1, 2, 3})]int"},
		{Pkg: "xi_multiline_trailing_comma", Kind: "type", Name: "T", Signature: "[len([...]int{1, 2, 3})]int"},
	}

	// (1) 絶対値。
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
		t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
	}

	// (2) 不変条件そのもの。バイト一致すること（AC-4-3 はバイト単位で比較
	// する）。
	sig := func(t *testing.T, pkg string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == "type" && r.Name == "T" {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q in %+v", pkg, got)
		return ""
	}
	oneline, multiline := sig(t, "xi_oneline"), sig(t, "xi_multiline_trailing_comma")
	if oneline != multiline {
		t.Errorf(
			"pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true"+
				"（AC-3-7-1: 配列長の複合リテラルの行取りは"+
				"公開 API の変化ではない。AC-4-3 はバイト単位で比較する）",
			"xi_oneline", oneline, "xi_multiline_trailing_comma", multiline,
		)
	}
}

// TestExtractRecords_AC3_8_1_GroupedTypeParamDeclsExpand は Issue #93
// レビュー往復8 の指摘 W-8-1 を固定する。AC-3-8-1 の「3-8-1 が要求する
// 期待値（テストに落とす形）」表 (i)〜(vi) をそのままテーブルへ落とす。
//
// 【指摘の要旨】
//
//	型パラメータリストのまとめ宣言（`[T, U any]`）が名前ごとに分けて書いた
//	形（`[T any, U any]`）へ展開されず、両者の <signature> がバイト不一致に
//	なる。AC-3-8-1 は「n 個の名前は n 個の型パラメータとして現れる」ことを
//	func / method の型パラメータリストと type の型パラメータリストの双方に
//	要求する。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	絶対値は「名前ごとに分けて書いた形」（= 展開後にあるべき形。3-8-1 が
//	定義する対照そのもの）の <signature> を使う。この綴り
//	（`[T any, U any]` の形）は本ファイルの
//	TestExtractRecords_AC3_7_1_IndexListLayoutSpellingIsInvariant
//	vi_typeparam_list_split ケース（`func F[T any, U any](t T) error` の
//	oneline 版）が既に確認済みの、go/printer が型パラメータリストへ一様に
//	出す綴りである。**この絶対値が固定するのは
//	「まとめ宣言と展開形がバイト一致すること」という不変条件であって、
//	区切り文字や空白の綴りそのものではない**（3-8-1 の期待値表の直後の
//	解説と同じ扱い）。実装工程が go/printer 以外の組み立てへ変えるなど
//	してこの綴りが変わる場合は、want を実装が採る新しい綴りへ合わせて
//	直してよい —— ただし「まとめ宣言側と展開形側が同じ絶対値になる」こと
//	（下の (1) の cmp.Diff で両方が同じ want を指す構造）は変えないこと。
//
// 【固定する表の行】
//
//	(i) 2名のまとめ宣言（func） (ii) 3名のまとめ宣言（func。個数で場合分け
//	しないこと） (iii) type の型パラメータリスト (iv) 対照: 制約が違えば
//	バイト不一致 (v) 対照: 型パラメータ名を落とさない（AC-9-7） (vi) 対照:
//	受信者型に書かれた型引数リストは本項の対象外（まとめ展開の影響を
//	受けない）。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_8_1_GroupedTypeParamDeclsExpand(t *testing.T) {
	files := map[string]string{
		// (i): 2名のまとめ宣言と分割宣言はバイト一致する（func）。
		"w81_i_grouped/a.go": "package w81_i_grouped\n\nfunc F[T, U any](t T, u U) error { return nil }\n",
		"w81_i_split/a.go":   "package w81_i_split\n\nfunc F[T any, U any](t T, u U) error { return nil }\n",

		// (ii): 3名でも同じ（個数で場合分けしない）。
		"w81_ii_grouped/a.go": "package w81_ii_grouped\n\nfunc F[T, U, V any](t T) error { return nil }\n",
		"w81_ii_split/a.go":   "package w81_ii_split\n\nfunc F[T any, U any, V any](t T) error { return nil }\n",

		// (iii): type の型パラメータリストにも掛かる。
		"w81_iii_grouped/a.go": "package w81_iii_grouped\n\ntype P[K, V any] struct {\n\tKey K\n}\n",
		"w81_iii_split/a.go":   "package w81_iii_split\n\ntype P[K any, V any] struct {\n\tKey K\n}\n",

		// (iv) 対照: 制約が違えばバイト不一致（展開が制約を取り違えない）。
		"w81_iv_a/a.go": "package w81_iv_a\n\nfunc F[T any, U comparable](t T) error { return nil }\n",
		"w81_iv_b/a.go": "package w81_iv_b\n\nfunc F[T, U any](t T) error { return nil }\n",

		// (v) 対照: 型パラメータ名を落とさないこと（AC-9-7）。
		"w81_v_a/a.go": "package w81_v_a\n\nfunc F[T, U any](t T) error { return nil }\n",
		"w81_v_b/a.go": "package w81_v_b\n\nfunc F[T, V any](t T) error { return nil }\n",

		// (vi) 対照: 受信者型に書かれた型引数リストはまとめ宣言ではなく
		// 本項の対象外。Pair 自身の型パラメータリストは split 形で宣言し
		// （(iii) の対象と混同しないため）、メソッドの受信者側
		// `Pair[K, V]` が本項によって `Pair[K any, V any]` へ展開され
		// ないことだけを固定する。
		"w81_vi/a.go": "package w81_vi\n\ntype Pair[K any, V any] struct {\n\tFirst  K\n\tSecond V\n}\n\n" +
			"func (p Pair[K, V]) M() error { return nil }\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	want := []record{
		{Pkg: "w81_i_grouped", Kind: "func", Name: "F", Signature: "[T any, U any] (T, U) (error)"},
		{Pkg: "w81_i_split", Kind: "func", Name: "F", Signature: "[T any, U any] (T, U) (error)"},

		{Pkg: "w81_ii_grouped", Kind: "func", Name: "F", Signature: "[T any, U any, V any] (T) (error)"},
		{Pkg: "w81_ii_split", Kind: "func", Name: "F", Signature: "[T any, U any, V any] (T) (error)"},

		{Pkg: "w81_iii_grouped", Kind: "type", Name: "P", Signature: "[K any, V any] struct { Key K }"},
		{Pkg: "w81_iii_split", Kind: "type", Name: "P", Signature: "[K any, V any] struct { Key K }"},

		{Pkg: "w81_iv_a", Kind: "func", Name: "F", Signature: "[T any, U comparable] (T) (error)"},
		{Pkg: "w81_iv_b", Kind: "func", Name: "F", Signature: "[T any, U any] (T) (error)"},

		{Pkg: "w81_v_a", Kind: "func", Name: "F", Signature: "[T any, U any] (T) (error)"},
		{Pkg: "w81_v_b", Kind: "func", Name: "F", Signature: "[T any, V any] (T) (error)"},

		{Pkg: "w81_vi", Kind: "type", Name: "Pair", Signature: "[K any, V any] struct { First K; Second V }"},
		{Pkg: "w81_vi", Kind: "method", Name: "Pair.M", Signature: "(Pair[K, V]) () (error)"},
	}

	// (1) 絶対値。相対比較だけにすると、まとめ宣言側と展開形側が同じ誤った
	// 綴りを返す偽 Green を検出できない（レビュー往復8 の C-8-1/I-8-1 と
	// 同じ注意）。
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
		t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
	}

	// (2) 期待値表が直接要求する「バイト一致する／しない」を record 同士の
	// 比較でも明示的に確認する（TestExtractRecords_AC3_9_1_... と同じ作法）。
	sig := func(t *testing.T, pkg, kind, name string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == kind && r.Name == name {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q kind %q name %q in %+v", pkg, kind, name, got)
		return ""
	}

	equalPairs := []struct{ a, b, kind, name, label string }{
		{"w81_i_grouped", "w81_i_split", "func", "F", "(i) grouped == split (2 names, func)"},
		{"w81_ii_grouped", "w81_ii_split", "func", "F", "(ii) grouped == split (3 names, func)"},
		{"w81_iii_grouped", "w81_iii_split", "type", "P", "(iii) grouped == split (type の型パラメータリスト)"},
	}
	for _, p := range equalPairs {
		a, b := sig(t, p.a, p.kind, p.name), sig(t, p.b, p.kind, p.name)
		if a != b {
			t.Errorf(
				"%s: pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true",
				p.label, p.a, a, p.b, b,
			)
		}
	}

	notEqualPairs := []struct{ a, b, kind, name, label string }{
		{"w81_iv_a", "w81_iv_b", "func", "F", "(iv) 制約が違えばバイト不一致"},
		{"w81_v_a", "w81_v_b", "func", "F", "(v) 型パラメータ名を落とさない（AC-9-7）"},
	}
	for _, p := range notEqualPairs {
		a, b := sig(t, p.a, p.kind, p.name), sig(t, p.b, p.kind, p.name)
		if a == b {
			t.Errorf(
				"%s: pkg %q signature=%q, pkg %q signature=%q: byte-equal=true, want byte-equal=false",
				p.label, p.a, a, p.b, b,
			)
		}
	}
}

// TestExtractRecords_AC3_9_2_MemberBoundarySeparator は Issue #93
// レビュー往復8 の指摘 W-8-2（最優先）を固定する。AC-3-9-2 の「3-9-2 が
// 要求する期待値（テストに落とす形）」表 (i)〜(x) をそのままテーブルへ
// 落とす。
//
// 【指摘の要旨】
//
//	AC-3-7 の空白畳み込みを字面どおり適用すると、構造体フィールド間・
//	インターフェースのメンバー間の境界（改行）が空白1つへ潰れる。すると
//	名前付きフィールド1個（`Logger` という名前と `Clock` という型）と、
//	埋め込みフィールド2個（`Logger` と `Clock`）が同じ綴り
//	`struct { Logger Clock }` になる —— 公開フィールド `Logger` の消失と
//	メソッド昇格という外形の変更がまるごとバイト一致に吸収され、この型が
//	差分に1行も現れない（偽 Green）。
//
// 【期待値を AC から導出する根拠（実装の出力に合わせたのではない）】
//
//	AC-3-9-2 は「メンバーの並び（個数・各メンバーが名前付きか埋め込みかの
//	区別・名前・型）が異なる型の <signature> は、バイト一致しない」ことを
//	要求し、直後の期待値表 (i)〜(x) がテストに落とす形をそのまま定める。
//	区切り文字の綴りそのものは 3-7 へ委ねられ本項では固定しないが、
//	「メンバー境界に何らかの区切りを出すこと」自体は固定される。
//
//	(ii) は「区切りが無くてもメンバー数の違いだけでバイト不一致になる」
//	ため、相対比較（不一致であること）だけでは区切り要求そのものの
//	ミューテーション検出力を持たない（区切りを一切出さない実装でも
//	(ii) は「不一致」を返してしまう）。同じ注意は (i) 以外の
//	「明らかに不一致・一致になる」行にも当てはまるため、本テストは
//	全行を1つの絶対値テーブル（want）として cmp.Diff で固定し、
//	相対比較（等値・非等値）は AC の期待値表の意図を明示するための
//	二重チェックとして追加する。
//
//	区切り文字の綴り自体（`"; "` の形）は本項が固定しないが、
//	テストとしては何らかの具体的な絶対値が要る。ここでは
//	docs/specs/public-api-diff-check.md の当該条文が持つ「一律の位置
//	落とし＋メンバー境界の改行だけを区切りへ写す」という正規化の説明と
//	整合する最小の綴り（セミコロン + 空白1つ。AC-3-8 が引数リストの
//	区切りに使う「カンマ + 空白1つ」と対になる形）を採用する。
//	**この絶対値が固定するのは「メンバーの並びが違えば異なる綴りになる
//	こと」「行取りに依らないこと」という不変条件であって、区切り文字
//	そのもの（`"; "` という具体的な1文字）ではない**（3-9-2 の期待値表
//	直後の解説と同じ扱い）。実装工程が別の区切り文字を選ぶ場合は、
//	want の区切り部分だけを実装が採る文字へ差し替えてよい —— ただし
//	「(i) が持つ2つの pkg のバイト不一致」「(v)(vi) がメンバー0個・1個
//	には区切りを出さないこと」「(iii)(iv) が行取りに依らずバイト一致
//	すること」は変えないこと。
//
// 【固定する表の行】
//
//	(i) 本項の中心: 名前付きフィールド1個 vs 埋め込み2個
//	(ii) interface にも一様に掛かること（対照。絶対値で押さえる）
//	(iii) struct: 区切りは行取りで変わらない
//	(iv) interface: 同上
//	(v) 対照: メンバー0個には区切りが現れない
//	(vi) 対照: メンバー1個にも区切りが現れない
//	(vii) 対照: 3-9 の除去を先に適用すること
//	(viii) 対照: 引数リストの区切りは 3-8 のカンマのまま変わらない
//	(ix) 入れ子の構造体にも掛かる
//	(x) map のキーに現れる構造体にも掛かる。
//
// 【依存】標準 testing + go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_9_2_MemberBoundarySeparator(t *testing.T) {
	files := map[string]string{
		// (i) 本項の中心: 名前付きフィールド1個（名前 Logger・型 Clock）と
		// 埋め込みフィールド2個（Logger と Clock）。両者の違いは改行だけ
		// であり、どちらも gofmt が保存する書き方である。
		"w82_i_name/a.go": "package w82_i_name\n\ntype Clock struct{}\n\ntype Emb struct {\n\tLogger Clock\n}\n",
		"w82_i_embed/a.go": "package w82_i_embed\n\ntype Logger struct{}\n\ntype Clock struct{}\n\n" +
			"type Emb struct {\n\tLogger\n\tClock\n}\n",

		// (ii) interface にも一様に掛かること。
		"w82_ii_two/a.go": "package w82_ii_two\n\nimport \"io\"\n\ntype T interface {\n\tio.Reader\n\tio.Writer\n}\n",
		"w82_ii_one/a.go": "package w82_ii_one\n\nimport \"io\"\n\ntype T interface {\n\tio.Reader\n}\n",

		// (iii) struct: 区切りは行取りで変わらない。
		"w82_iii_one/a.go":   "package w82_iii_one\n\ntype T struct { A int; B int }\n",
		"w82_iii_split/a.go": "package w82_iii_split\n\ntype T struct {\n\tA int\n\tB int\n}\n",

		// (iv) interface: 同上。
		"w82_iv_one/a.go":   "package w82_iv_one\n\ntype T interface { A() error; B() error }\n",
		"w82_iv_split/a.go": "package w82_iv_split\n\ntype T interface {\n\tA() error\n\tB() error\n}\n",

		// (v) 対照: メンバー0個には区切りが現れない。
		"w82_v_struct/a.go": "package w82_v_struct\n\ntype T struct{}\n",
		"w82_v_iface/a.go":  "package w82_v_iface\n\ntype T interface{}\n",

		// (vi) 対照: メンバー1個にも区切りが現れない。
		"w82_vi_struct/a.go": "package w82_vi_struct\n\ntype T struct {\n\tA int\n}\n",
		"w82_vi_iface/a.go":  "package w82_vi_iface\n\ntype T interface {\n\tM()\n}\n",

		// (vii) 対照: 3-9 の除去を先に適用すること。除去された非公開
		// フィールドのぶんの区切りが残らない。
		"w82_vii_grouped/a.go": "package w82_vii_grouped\n\ntype T struct {\n\ta int\n\tB int\n}\n",
		"w82_vii_single/a.go":  "package w82_vii_single\n\ntype T struct {\n\tB int\n}\n",

		// (viii) 対照: 引数リストの区切りは 3-8 のカンマのまま変わらない。
		"w82_viii_func/a.go": "package w82_viii_func\n\nfunc F(a int, b string) error { return nil }\n",

		// (ix) 入れ子の構造体にも掛かる。
		"w82_ix_two/a.go": "package w82_ix_two\n\ntype T struct {\n\tInner struct {\n\t\tA int\n\t\tB int\n\t}\n}\n",
		"w82_ix_one/a.go": "package w82_ix_one\n\ntype T struct {\n\tInner struct {\n\t\tA int\n\t}\n}\n",

		// (x) map のキーに現れる構造体にも掛かる。
		"w82_x_two/a.go": "package w82_x_two\n\ntype T map[struct {\n\tA int\n\tB int\n}]int\n",
		"w82_x_one/a.go": "package w82_x_one\n\ntype T map[struct {\n\tA int\n}]int\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	want := []record{
		{Pkg: "w82_i_name", Kind: "type", Name: "Emb", Signature: "struct { Logger Clock }"},
		{Pkg: "w82_i_name", Kind: "type", Name: "Clock", Signature: "struct { }"},
		{Pkg: "w82_i_embed", Kind: "type", Name: "Emb", Signature: "struct { Logger; Clock }"},
		{Pkg: "w82_i_embed", Kind: "type", Name: "Logger", Signature: "struct { }"},
		{Pkg: "w82_i_embed", Kind: "type", Name: "Clock", Signature: "struct { }"},

		{Pkg: "w82_ii_two", Kind: "type", Name: "T", Signature: "interface { io.Reader; io.Writer }"},
		{Pkg: "w82_ii_one", Kind: "type", Name: "T", Signature: "interface { io.Reader }"},

		{Pkg: "w82_iii_one", Kind: "type", Name: "T", Signature: "struct { A int; B int }"},
		{Pkg: "w82_iii_split", Kind: "type", Name: "T", Signature: "struct { A int; B int }"},

		{Pkg: "w82_iv_one", Kind: "type", Name: "T", Signature: "interface { A() error; B() error }"},
		{Pkg: "w82_iv_split", Kind: "type", Name: "T", Signature: "interface { A() error; B() error }"},

		{Pkg: "w82_v_struct", Kind: "type", Name: "T", Signature: "struct { }"},
		{Pkg: "w82_v_iface", Kind: "type", Name: "T", Signature: "interface { }"},

		{Pkg: "w82_vi_struct", Kind: "type", Name: "T", Signature: "struct { A int }"},
		{Pkg: "w82_vi_iface", Kind: "type", Name: "T", Signature: "interface { M() }"},

		{Pkg: "w82_vii_grouped", Kind: "type", Name: "T", Signature: "struct { B int }"},
		{Pkg: "w82_vii_single", Kind: "type", Name: "T", Signature: "struct { B int }"},

		{Pkg: "w82_viii_func", Kind: "func", Name: "F", Signature: "(int, string) (error)"},

		{Pkg: "w82_ix_two", Kind: "type", Name: "T", Signature: "struct { Inner struct { A int; B int } }"},
		{Pkg: "w82_ix_one", Kind: "type", Name: "T", Signature: "struct { Inner struct { A int } }"},

		{Pkg: "w82_x_two", Kind: "type", Name: "T", Signature: "map[struct { A int; B int }]int"},
		{Pkg: "w82_x_one", Kind: "type", Name: "T", Signature: "map[struct { A int }]int"},
	}

	// (1) 絶対値。これが本テストの主張の核 —— (ii) をはじめとする「明らか
	// に不一致になる」対照は、相対比較だけでは区切り要求そのものの
	// ミューテーション検出力を持たない（上のコメント参照）。
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
		t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
	}

	// (2) 期待値表が直接要求する「バイト一致する／しない」を record 同士の
	// 比較でも明示的に確認する（期待値リテラルの書き損じを絶対値だけでは
	// 拾えないため。TestExtractRecords_AC3_9_1_... と同じ作法）。
	sig := func(t *testing.T, pkg, name string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == "type" && r.Name == name {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q name %q in %+v", pkg, name, got)
		return ""
	}

	// (i): 本項の中心。名前付きフィールド1個と埋め込み2個はバイト不一致。
	if name, embed := sig(t, "w82_i_name", "Emb"), sig(t, "w82_i_embed", "Emb"); name == embed {
		t.Errorf(
			"(i) pkg %q signature=%q, pkg %q signature=%q: byte-equal=true, want byte-equal=false"+
				"（AC-3-9-2: 名前付きフィールド1個からなる構造体と、埋め込み"+
				"フィールド2個からなる構造体をバイト一致させない —— 境界が"+
				"消えると公開フィールドの消失とメソッド昇格という外形の変更が"+
				"まるごと吸収される）",
			"w82_i_name/Emb", name, "w82_i_embed/Emb", embed,
		)
	}

	// (ii): interface にも一様に掛かる。
	if two, one := sig(t, "w82_ii_two", "T"), sig(t, "w82_ii_one", "T"); two == one {
		t.Errorf(
			"(ii) pkg %q signature=%q, pkg %q signature=%q: byte-equal=true, want byte-equal=false"+
				"（AC-3-9-2: interface のメンバーの並びが違えばバイト一致しない）",
			"w82_ii_two/T", two, "w82_ii_one/T", one,
		)
	}

	equalPairs := []struct{ a, b, label string }{
		{"w82_iii_one", "w82_iii_split", "(iii) struct: 区切りは行取りで変わらない"},
		{"w82_iv_one", "w82_iv_split", "(iv) interface: 区切りは行取りで変わらない"},
		{"w82_vii_grouped", "w82_vii_single", "(vii) 3-9 の除去を先に適用すること"},
	}
	for _, p := range equalPairs {
		a, b := sig(t, p.a, "T"), sig(t, p.b, "T")
		if a != b {
			t.Errorf(
				"%s: pkg %q signature=%q, pkg %q signature=%q: byte-equal=false, want byte-equal=true",
				p.label, p.a, a, p.b, b,
			)
		}
	}

	notEqualPairs := []struct{ a, b, label string }{
		{"w82_ix_two", "w82_ix_one", "(ix) 入れ子の構造体にも掛かる"},
		{"w82_x_two", "w82_x_one", "(x) map のキーに現れる構造体にも掛かる"},
	}
	for _, p := range notEqualPairs {
		a, b := sig(t, p.a, "T"), sig(t, p.b, "T")
		if a == b {
			t.Errorf(
				"%s: pkg %q signature=%q, pkg %q signature=%q: byte-equal=true, want byte-equal=false",
				p.label, p.a, a, p.b, b,
			)
		}
	}
}

// TestExtractRecords_AC3_6_1_RulesApplyInsideExpressions は
// docs/specs/public-api-diff-check.md AC-3-6-1「3-6-1 が要求する期待値
// （テストに落とす形）」表 (i)〜(vii) をそのままテーブルへ落とす。
//
// 【要求の要旨】
//
//	抽出規則（3-6 の引数名除去・3-9 の非公開除去・3-9-1 のまとめ宣言展開・
//	3-9-2 のメンバー境界区切り）は、型式の内部に現れる式（配列長の式、
//	その中の呼び出し `len(...)` / `unsafe.Sizeof(...)` の引数、複合
//	リテラルなど）の内部に現れる構造体型・インターフェース型・関数型にも
//	一様に掛かる。その結果、式の内部に現れる型の綴りは、同じ型を型式の
//	直下に置いた宣言の <signature> とバイト一致する。
//
// 【期待値の形】
//
//	(i)〜(vi) は同項本文の指示どおり、「対照する宣言の <signature> が、
//	対象の宣言の <signature> の部分文字列として現れること」で固定する
//	（対象側は外側の型式のぶんだけ長くなるため、全体のバイト一致には
//	ならない）。(vii) は対照を持たない行であり、`go/parser` が配列長では
//	なく型パラメータリストとして解析する形（複合リテラルの `{}` を持た
//	ない）について、非公開 `hidden` が現れず公開 `Pub` が現れることだけを
//	固定する（型パラメータリストの綴り規則そのものは 3-6-1 の対象外）。
//
// 【依存】標準 testing + google/go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_6_1_RulesApplyInsideExpressions(t *testing.T) {
	files := map[string]string{
		// (i): 3-9 の非公開除去が配列長の式（len の複合リテラル引数）の
		// 内部にも掛かること。
		"ac361_i_t/a.go": "package ac361_i_t\n\n" +
			"type T [len([...]struct{ hidden int; Pub int }{})]int\n",
		"ac361_i_u/a.go": "package ac361_i_u\n\ntype U struct{ hidden int; Pub int }\n",

		// (ii): len 以外の呼び出し（unsafe.Sizeof）の内部にも掛かること
		// （根拠4。呼び出される関数の名前で場合分けしない）。
		"ac361_ii_t/a.go": "package ac361_ii_t\n\nimport \"unsafe\"\n\n" +
			"type T [unsafe.Sizeof(struct{ hidden int; Pub int }{})]int\n",
		"ac361_ii_u/a.go": "package ac361_ii_u\n\ntype U struct{ hidden int; Pub int }\n",

		// (iii): 3-9-2 のメンバー境界の区切りが式の内部にも掛かること。
		"ac361_iii_t/a.go": "package ac361_iii_t\n\n" +
			"type T [len([...]interface{ A() error; B() error }{})]int\n",
		"ac361_iii_u/a.go": "package ac361_iii_u\n\ntype U interface{ A() error; B() error }\n",

		// (iv): 式の中のさらに入れ子（配列長の式の内部の map のキーに
		// 現れる構造体）。
		"ac361_iv_t/a.go": "package ac361_iv_t\n\n" +
			"type T [len([...]map[struct{ P int; Q int }]int{})]int\n",
		"ac361_iv_u/a.go": "package ac361_iv_u\n\ntype U map[struct{ P int; Q int }]int\n",

		// (v): 3-6 の引数名の除去が式の内部にも掛かること。
		"ac361_v_t/a.go": "package ac361_v_t\n\n" +
			"type T [len([...]func(a int, b string){})]int\n",
		"ac361_v_u/a.go": "package ac361_v_u\n\ntype U func(a int, b string)\n",

		// (vi): 3-9-1 のまとめ宣言の展開が式の内部にも掛かること。
		"ac361_vi_t/a.go": "package ac361_vi_t\n\n" +
			"type T [len([...]struct{ A, B int }{})]int\n",
		"ac361_vi_u/a.go": "package ac361_vi_u\n\ntype U struct{ A, B int }\n",

		// (vii): 対照。複合リテラルの `{}` を持たないため go/parser は
		// 配列長ではなく型パラメータリストとして解析する
		// （TypeSpec.TypeParams が非 nil）。これは 3-6-1 の違反ではない。
		// 本行が固定するのは、型パラメータリストの内部に現れる構造体にも
		// 3-9 が掛かること —— <signature> に hidden が現れず、Pub が
		// 現れること。
		"ac361_vii/a.go": "package ac361_vii\n\n" +
			"type T [len([...]struct{ hidden int; Pub int })]int\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	sig := func(t *testing.T, pkg, name string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == "type" && r.Name == name {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q name %q in %+v", pkg, name, got)
		return ""
	}

	containsCases := []struct {
		targetPkg, counterPkg, label string
	}{
		{"ac361_i_t", "ac361_i_u", "(i) len の複合リテラル引数の内部への 3-9 非公開除去"},
		{"ac361_ii_t", "ac361_ii_u", "(ii) unsafe.Sizeof の内部への 3-9 非公開除去（呼び出し名で場合分けしない）"},
		{"ac361_iii_t", "ac361_iii_u", "(iii) 配列長の式の内部への 3-9-2 メンバー境界区切り"},
		{"ac361_iv_t", "ac361_iv_u", "(iv) 式の中のさらに入れ子（map のキーの構造体）"},
		{"ac361_v_t", "ac361_v_u", "(v) 式の内部への 3-6 引数名除去"},
		{"ac361_vi_t", "ac361_vi_u", "(vi) 式の内部への 3-9-1 まとめ宣言展開"},
	}
	for _, c := range containsCases {
		target, counter := sig(t, c.targetPkg, "T"), sig(t, c.counterPkg, "U")
		if !strings.Contains(target, counter) {
			t.Errorf(
				"%s: target pkg %q signature=%q does not contain counter pkg %q signature=%q"+
					"（AC-3-6-1: 式の内部に現れる型の綴りは、同じ型を型式の直下に"+
					"置いた宣言の <signature> とバイト一致するはず）",
				c.label, c.targetPkg, target, c.counterPkg, counter,
			)
		}
	}

	// (vii): 型パラメータリストとして解析される側にも 3-9 が掛かること。
	viiSig := sig(t, "ac361_vii", "T")
	if strings.Contains(viiSig, "hidden") {
		t.Errorf(
			"(vii) pkg %q signature=%q contains %q, want it removed"+
				"（AC-3-6-1: 型パラメータリストとして解析される位置にも "+
				"3-9 の非公開除去が掛かる）",
			"ac361_vii", viiSig, "hidden",
		)
	}
	if !strings.Contains(viiSig, "Pub") {
		t.Errorf(
			"(vii) pkg %q signature=%q does not contain %q, want it present"+
				"（AC-3-6-1: 公開フィールドは残る）",
			"ac361_vii", viiSig, "Pub",
		)
	}
}

// TestExtractRecords_AC3_6_1_FalseGreenCenterInsideArrayLen は Issue #93
// レビュー往復10 の指摘（AC-3-9-2 (i) の偽 Green の中心 ——
// 名前付きフィールド1個〔名前 Logger・型 Clock〕と埋め込みフィールド2個
// 〔Logger と Clock〕をバイト一致させない、という要求）を、AC-3-6-1 が
// 広げる位置（配列長の式の内部）に置いて固定する。
//
// 【なぜ相対比較（部分文字列）だけでは足りないか】
//
//	上のテーブル駆動テスト（(i)〜(vii)）は「対照の <signature> が対象の
//	<signature> に部分文字列として現れること」で固定してよいと 3-6-1 の
//	期待値表の前書きが定める。しかしこの相対比較は、対象・対照の両方が
//	同時に同じ壊れ方（境界の消失）をした場合には空振りする
//	（TestExtractRecords_AC3_9_2_MemberBoundarySeparator のコメント、
//	および MEMORY.md mutation-test-false-green.md と同種の事故）。
//	AC-3-9-2 (i) が「本項の中心」と呼ぶ偽 Green
//	（名前付き1個と埋め込み2個がバイト一致してしまう）を、配列長の式の
//	内部という 3-6-1 が新たに掛ける位置で確実に検出するには、絶対値の
//	want と、両者のバイト不一致という明示的な主張が要る。
//
// 【絶対値の導出根拠（推測ではない）】
//
//	外側の配列長式のテンプレート `[len([...]TYPE{ELEMS})]int` は
//	TestExtractRecords_AC3_7_1_ArrayLenCompositeLitLineBreakIsInvariant の
//	xi_oneline ケース（`type T [len([...]int{1, 2, 3})]int` →
//	`[len([...]int{1, 2, 3})]int`）が既に固定している go/printer の一様な
//	綴りである。今回は複合リテラルの要素が空（ELEMS が空）である点だけが
//	異なり、`{}` の間に何も入らない。
//	内側の TYPE 部分（`struct { Logger Clock }` / `struct { Logger; Clock }`）
//	は TestExtractRecords_AC3_9_2_MemberBoundarySeparator の
//	w82_i_name / w82_i_embed ケースが AC-3-9-2 (i) そのものとして既に
//	固定している絶対値である。
//	両者を機械的に組み合わせたものが本テストの want であり、実装の出力に
//	合わせたものではない。
//
// 【依存】標準 testing + google/go-cmp のみ（ADR 0007）。
func TestExtractRecords_AC3_6_1_FalseGreenCenterInsideArrayLen(t *testing.T) {
	files := map[string]string{
		// 名前付きフィールド1個（名前 Logger・型 Clock）を配列長の式の
		// 内部に置いたもの。
		"ac361_fg_name/a.go": "package ac361_fg_name\n\ntype Clock struct{}\n\n" +
			"type T [len([...]struct {\n\tLogger Clock\n}{})]int\n",

		// 埋め込みフィールド2個（Logger と Clock）を配列長の式の内部に
		// 置いたもの。両者の違いは改行だけ。
		"ac361_fg_embed/a.go": "package ac361_fg_embed\n\ntype Logger struct{}\n\ntype Clock struct{}\n\n" +
			"type T [len([...]struct {\n\tLogger\n\tClock\n}{})]int\n",
	}

	dir := writeFixture(t, files)
	got, err := extractRecords(dir)
	if err != nil {
		t.Fatalf("extractRecords(%q) returned unexpected error: %v", dir, err)
	}

	want := []record{
		{Pkg: "ac361_fg_name", Kind: "type", Name: "T", Signature: "[len([...]struct { Logger Clock }{})]int"},
		{Pkg: "ac361_fg_name", Kind: "type", Name: "Clock", Signature: "struct { }"},

		{Pkg: "ac361_fg_embed", Kind: "type", Name: "T", Signature: "[len([...]struct { Logger; Clock }{})]int"},
		{Pkg: "ac361_fg_embed", Kind: "type", Name: "Logger", Signature: "struct { }"},
		{Pkg: "ac361_fg_embed", Kind: "type", Name: "Clock", Signature: "struct { }"},
	}

	// (1) 絶対値。
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(byRecord)); diff != "" {
		t.Errorf("extractRecords(%q) mismatch (-want +got):\n%s", dir, diff)
	}

	// (2) 偽 Green の中心そのもの。両者はバイト一致してはならない。
	sig := func(t *testing.T, pkg string) string {
		t.Helper()
		for _, r := range got {
			if r.Pkg == pkg && r.Kind == "type" && r.Name == "T" {
				return r.Signature
			}
		}
		t.Fatalf("record not found for pkg %q in %+v", pkg, got)
		return ""
	}
	name, embed := sig(t, "ac361_fg_name"), sig(t, "ac361_fg_embed")
	if name == embed {
		t.Errorf(
			"pkg %q signature=%q, pkg %q signature=%q: byte-equal=true, want byte-equal=false"+
				"（AC-3-6-1 が AC-3-9-2 (i) の偽 Green を配列長の式の内部へ広げる: "+
				"名前付きフィールド1個からなる構造体と、埋め込みフィールド2個から"+
				"なる構造体を、配列長の式の内部に置いてもバイト一致させない）",
			"ac361_fg_name", name, "ac361_fg_embed", embed,
		)
	}
}
