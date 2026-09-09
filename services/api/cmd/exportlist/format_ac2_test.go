package main

// 検証対象: docs/specs/public-api-diff-check.md AC-2（エクスポート一覧の
// レコード書式）。formatRecords（main.go）は record 列を出力行へ直列化し
// AC-2-7 の順序へ並べ替える関数であり、AC-2 の書式とソート順がこの関数の
// 境界に現れる（Issue #93 reviewer 往復5 の指摘 W-5-1: AC-2 の書式と
// ソート順を固定するテストが1本も無かった）。
//
// 【この関数境界で見える条文と、見えない条文の分界】
//
// AC-2-6 は「行の終端は LF。空行を出力しない。ファイル末尾の LF は行の
// 終端であって空行ではない」と定める。formatRecords が返すのは**行の内容**
// であり、行終端の LF は含まない（LF は main() の書き出しが1要素につき
// 1つ付ける）。したがって AC-2-6 はこの境界で次の2つに分かれる。
//
//   - 本テストが固定する側: 返す各要素が**それ自体は行終端を持たず**
//     （"\n" / "\r" を含まない）、**空でない**こと。空文字列の要素があれば
//     呼び出し側が LF を付けた時点で空行になる。
//   - 本テストが固定しない側: 呼び出し側が1要素につき LF をちょうど1つ
//     付けること。これは main() の書き出し（fmt.Println）の性質であり、
//     formatRecords の戻り値からは観測できない。ここを見るには抽出器を
//     プロセスとして起動する必要があり、そのための新しい差し替え点は
//     作らない（`docs/adr/0018` 決定2）。**未検査であることを記録として
//     残す。**
//
// AC-2-2 / 2-3 / 2-4 / 2-5 が定めるのは各フィールドの**中身**であり、
// その生成は抽出器（extract.go、AC-3）の責務である。formatRecords の境界で
// 固定できるのは「与えられた4フィールドを、順序を変えず・値を変えず・
// タブで区切って1行にする」ことまでで、本テストはそこに留める。仕様に無い
// 検査（フィールド内容の妥当性検証など）を formatRecords へ要求しない。
//
// 【依存】標準 testing + google/go-cmp のみ（ADR 0007）。並び順そのものが
// 検査対象（AC-2-7）なので、cmpopts.SortSlices は使わない。

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFormatRecords_AC2(t *testing.T) {
	tests := []struct {
		name    string // AC 番号を含める
		records []record
		want    []string
	}{
		{
			// AC-2-1: 1行 = 1シンボル。タブ区切りのちょうど4フィールドを
			// <pkg> <kind> <name> <signature> の順で並べる。
			//
			// 4フィールドの値をすべて相異なる綴りにして、順序の入れ替えが
			// 起きたら必ず落ちるようにする。
			name: "AC-2-1_tab_separated_four_fields_in_pkg_kind_name_signature_order",
			records: []record{
				{Pkg: "internal/domain", Kind: "func", Name: "Sum", Signature: "(int, int) (int)"},
			},
			want: []string{
				"internal/domain\tfunc\tSum\t(int, int) (int)",
			},
		},
		{
			// AC-2-2: <pkg> は services/api からの相対ディレクトリパス
			// （`/` 区切り、先頭に `./` を付けない）。services/api 直下は `.`。
			// AC-2-3: <kind> は func / method / type / var / const の5つのみ。
			// AC-2-4: <name> は識別子。method のみ
			// `<受信者の基底型名>.<メソッド名>`。
			//
			// formatRecords は与えられた値を書き換えない（`.` を `./` へ
			// 補ったり、method の `.` を別の区切りへ変えたりしない）。
			// 5つの kind をすべて置き、どれか1つでも落ちる・書き換わる実装を
			// 排除する。並び順は AC-2-7（バイト昇順）に従う。
			name: "AC-2-2_3_4_fields_are_emitted_verbatim_for_all_five_kinds",
			records: []record{
				{Pkg: ".", Kind: "var", Name: "Version", Signature: "string"},
				{Pkg: ".", Kind: "type", Name: "Box", Signature: "struct { N int }"},
				{Pkg: ".", Kind: "method", Name: "Box.Get", Signature: "(Box) () (int)"},
				{Pkg: ".", Kind: "func", Name: "New", Signature: "() (Box)"},
				{Pkg: ".", Kind: "const", Name: "Limit", Signature: "int"},
			},
			want: []string{
				".\tconst\tLimit\tint",
				".\tfunc\tNew\t() (Box)",
				".\tmethod\tBox.Get\t(Box) () (int)",
				".\ttype\tBox\tstruct { N int }",
				".\tvar\tVersion\tstring",
			},
		},
		{
			// AC-2-5: <signature> はタブ・改行を含まない（AC-3-7 の正規化で
			// 保証する）。したがって <signature> に空白が含まれていても
			// フィールドの境界にはならず、そのまま4フィールド目に載る。
			// 空白を含む signature を分割・詰めない（例:
			// "(int, int) (error)" を "(int," と "int)" に割らない）。
			name: "AC-2-5_signature_with_spaces_stays_in_the_fourth_field",
			records: []record{
				{
					Pkg:       "internal/usecase/port",
					Kind:      "type",
					Name:      "Repo",
					Signature: "interface { Save(WorkMonth) (error) }",
				},
			},
			want: []string{
				"internal/usecase/port\ttype\tRepo\tinterface { Save(WorkMonth) (error) }",
			},
		},
		{
			// AC-2-7: 出力は LC_ALL=C のバイト昇順でソートする。
			//
			// 入力の順序はわざと逆順に近い形で与える。期待値は**バイト**
			// 昇順であり、ロケールの照合順序ではない: ASCII では
			// '.'(0x2E) < 'Z'(0x5A) < '_'(0x5F) < 'a'(0x61) であるのに対し、
			// en_US.UTF-8 のような照合順序では "alpha" が "Zeta" より前に
			// 来る。両者が食い違う組み合わせを入れて、バイト昇順であること
			// を弁別できるようにする。
			name: "AC-2-7_sorts_by_LC_ALL_C_byte_ascending_not_locale_collation",
			records: []record{
				{Pkg: "alpha", Kind: "func", Name: "A", Signature: "() ()"},
				{Pkg: "_internal", Kind: "func", Name: "A", Signature: "() ()"},
				{Pkg: "Zeta", Kind: "func", Name: "A", Signature: "() ()"},
				{Pkg: ".", Kind: "func", Name: "A", Signature: "() ()"},
			},
			want: []string{
				".\tfunc\tA\t() ()",
				"Zeta\tfunc\tA\t() ()",
				"_internal\tfunc\tA\t() ()",
				"alpha\tfunc\tA\t() ()",
			},
		},
		{
			// AC-2-7（同一 pkg 内の並び）: 行全体のバイト昇順である。
			// タブ(0x09)はどの印字可能文字よりも小さいため、前方が共通で
			// 短い行が先に来る（"Foo" < "Foobar"）。kind の綴り順
			// （const < func < method < type < var）もバイト昇順で決まる。
			name: "AC-2-7_byte_ascending_over_the_whole_line",
			records: []record{
				{Pkg: "p", Kind: "func", Name: "Foobar", Signature: "() ()"},
				{Pkg: "p", Kind: "func", Name: "Foo", Signature: "() ()"},
				{Pkg: "p", Kind: "const", Name: "Foo", Signature: "int"},
				{Pkg: "p", Kind: "var", Name: "Foo", Signature: "int"},
			},
			want: []string{
				"p\tconst\tFoo\tint",
				"p\tfunc\tFoo\t() ()",
				"p\tfunc\tFoobar\t() ()",
				"p\tvar\tFoo\tint",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecords(tt.records)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("formatRecords() mismatch (-want +got):\n%s", diff)
			}

			// AC-2-1 / AC-2-6 の構造的な要求は、上の絶対値とは別に全ケース
			// 共通で確認する。絶対値だけだと、期待値リテラルを書き損じた
			// ケースでこの要求が抜け落ちるため。
			for i, line := range got {
				if line == "" {
					t.Errorf(
						"formatRecords()[%d] が空文字列（AC-2-6: 空行を出力しない）", i,
					)
					continue
				}
				if strings.ContainsAny(line, "\n\r") {
					t.Errorf(
						"formatRecords()[%d] = %q に改行が含まれる"+
							"（AC-2-6: 行の終端 LF は呼び出し側が付ける。"+
							"要素自体は行終端を持たない）",
						i, line,
					)
				}
				if n := strings.Count(line, "\t"); n != 3 {
					t.Errorf(
						"formatRecords()[%d] = %q のタブ数 = %d, want 3"+
							"（AC-2-1: タブ区切りのちょうど4フィールド）",
						i, line, n,
					)
				}
			}
		})
	}
}

// TestFormatRecords_AC2_6_NoLinesForNoRecords は AC-2-6「空行を出力しない」の
// 境界を固定する: レコードが0件なら行も0本であり、空文字列の要素を1つ返す
// （＝呼び出し側が LF を付けて空行になる）ことがない。
//
// nil スライスと長さ0のスライスのどちらを返すかは AC-2 のどの条文も定めて
// いないため、長さだけを見る（仕様に無い区別を要求しない）。
func TestFormatRecords_AC2_6_NoLinesForNoRecords(t *testing.T) {
	got := formatRecords(nil)
	if len(got) != 0 {
		t.Errorf(
			"formatRecords(nil) が返した行数 = %d, want 0（詳細: %q）",
			len(got), got,
		)
	}

	got = formatRecords([]record{})
	if len(got) != 0 {
		t.Errorf(
			"formatRecords([]record{}) が返した行数 = %d, want 0（詳細: %q）",
			len(got), got,
		)
	}
}
