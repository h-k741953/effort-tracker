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
			// の解説にある（メンバーが2個以上のときは go/printer が
			// この形しか出さないため、AC-3-7 の下で全メンバー数に
			// 通用する綴りはこれ1つに定まる）。
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
				{Pkg: "mixedunion", Kind: "type", Name: "Mixed", Signature: "interface { ~int | ~string Do() error }"},
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
				{Pkg: "unionmethod", Kind: "type", Name: "ConstraintWithMethod", Signature: "interface { ~int | ~string Do() error }"},
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
					Signature: "struct { Pub struct { Shown int } " +
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
		{Pkg: "i_with", Kind: "type", Name: "T", Signature: "interface { error Code() int }"},
		{Pkg: "i_without", Kind: "type", Name: "T", Signature: "interface { Code() int }"},

		{Pkg: "ii_with", Kind: "type", Name: "T", Signature: "interface { comparable }"},
		{Pkg: "ii_without", Kind: "type", Name: "T", Signature: "interface { }"},

		{Pkg: "iii_with", Kind: "type", Name: "T", Signature: "interface { any }"},
		{Pkg: "iii_without", Kind: "type", Name: "T", Signature: "interface { }"},

		{Pkg: "iv_with", Kind: "type", Name: "T", Signature: "struct { error N int }"},
		{Pkg: "iv_without", Kind: "type", Name: "T", Signature: "struct { N int }"},

		{Pkg: "v_with", Kind: "type", Name: "T", Signature: "struct { int M string }"},
		{Pkg: "v_without", Kind: "type", Name: "T", Signature: "struct { M string }"},

		{Pkg: "vi", Kind: "type", Name: "T", Signature: "struct { io.Reader }"},

		{Pkg: "vii_with", Kind: "type", Name: "T", Signature: "struct { N int }"},
		{Pkg: "vii_without", Kind: "type", Name: "T", Signature: "struct { N int }"},

		{Pkg: "viii_with", Kind: "type", Name: "T", Signature: "struct { N int }"},
		{Pkg: "viii_without", Kind: "type", Name: "T", Signature: "struct { N int }"},

		{Pkg: "ix_with", Kind: "type", Name: "T", Signature: "struct { *int M string }"},
		{Pkg: "ix_without", Kind: "type", Name: "T", Signature: "struct { M string }"},

		{
			Pkg: "allpredeclared_iface", Kind: "type", Name: "All",
			Signature: "interface { any bool byte comparable complex64 complex128 error " +
				"float32 float64 int int8 int16 int32 int64 rune string uint uint8 " +
				"uint16 uint32 uint64 uintptr }",
		},
		{
			Pkg: "allpredeclared_struct", Kind: "type", Name: "AllStruct",
			Signature: "struct { any bool byte complex64 complex128 error " +
				"float32 float64 int int8 int16 int32 int64 rune string uint uint8 " +
				"uint16 uint32 uint64 uintptr }",
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
//	空白の**有無**を変えられない（削除も挿入もしない）。したがって綴りは
//	go/printer が出す2つの形のどちらかに限られる。
//
//	  (1) `struct{ N int }`  … `struct` と `{` のあいだに空白が無い形。
//	      go/printer はソースの `{` と `}` が同じ行にあり、かつ残った
//	      フィールドが**ちょうど1個**（かつ短い）ときにだけこの形を出す。
//	  (2) `struct { N int }` … 空白が入る形。上記以外のすべてで出る。
//
//	メンバーが**2個以上**残る型は (1) では書けない —— go/printer に (1) を
//	選ばせる分岐が存在しない。よって「メンバー数によらず同一の綴り」を
//	AC-3-7 の範囲で満たせる形は (2) ただ1つである。0個のときの (2) は
//	`struct { }`、インターフェースなら `interface { }` になる。
//
//	この導出の帰結として、本ファイルの既存ケースが持っていた `struct{}` /
//	`interface{}` / `struct{ N int }` などの絶対値は (2) の綴りへ改めた。
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
			// go/printer がこの形しか出せない（上記解説 (2)）ため、
			// 綴りの基準点になるグループ。
			name: "struct_two",
			want: "struct { N int M string }",
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
			// インターフェース（struct_two と同じく綴りの基準点）。
			name: "iface_two",
			want: "interface { Do() error Get() int }",
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
