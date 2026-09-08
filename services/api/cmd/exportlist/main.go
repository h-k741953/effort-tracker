// Package main は公開 API 一覧の抽出器（Issue #93）。
//
// 用途: services/api モジュール配下の1ディレクトリを引数に取り、AC-3 の
// 抽出規則を適用した結果を AC-2 のレコード書式（タブ区切り4フィールド、
// LC_ALL=C のバイト昇順）で標準出力へ書く。
//
// docs/specs/public-api-diff-check.md D-1 により、この抽出器は
// services/api/cmd/exportlist/ に置く。AC-3-15 により、トップレベルの
// 公開シンボルを1つも持たない（func main を含め、すべて非公開の識別子で
// 書く。main は Go の言語仕様上すでに小文字で始まる非公開の識別子であり、
// この要求と衝突しない）。
//
// 呼び出しは .github/workflows/ci.yml の go ジョブ（警告 step）が担う
// （AC-7）。使い方: exportlist <対象ディレクトリ>
package main

import (
	"fmt"
	"os"
	"sort"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: exportlist <対象ディレクトリ>")
		os.Exit(1)
	}

	records, err := extractRecords(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "exportlist: "+err.Error())
		os.Exit(1)
	}

	for _, line := range formatRecords(records) {
		fmt.Println(line)
	}
}

// formatRecords は AC-2 のレコード書式に従って record 列を出力行へ直列化し、
// AC-2-7 の順序に並べ替えて返す。返す各要素は行の内容であり、行終端の LF は
// 含まない（LF は呼び出し側の書き出しが付ける＝AC-2-6）。
//
// AC-3-15 により、この関数と引数・戻り値の型はすべて非公開の識別子で書く。
func formatRecords(records []record) []string {
	lines := make([]string, 0, len(records))
	for _, r := range records {
		// AC-2-1: タブ区切りのちょうど4フィールド
		// （<pkg> <kind> <name> <signature>）。
		lines = append(lines, r.Pkg+"\t"+r.Kind+"\t"+r.Name+"\t"+r.Signature)
	}
	// AC-2-7: LC_ALL=C のバイト昇順。Go の文字列比較（sort.Strings）は
	// バイト単位の比較であり、これと一致する。
	sort.Strings(lines)

	return lines
}
