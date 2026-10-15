package workmonth_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 期待値の表（docs/specs/daily-record-entry.md AC-2・AC-3）を読むローダ。
//
// 実装設計 AC-12-18 ① と daily-record-entry-ui.md AC-3-6-d により、時刻の組の行は
// テストコードへ手で写さず、仕様書の表から入力と期待を読む。表へ行を足すと、
// このローダを通して本パッケージのテストに現れる（画面側の
// apps/web/src/lib/working-hours.test.ts も同じ表を読む）。
//
// 表の読み方（画面側のテストと同じ規則）:
//   - 行の入力の列から「`h:m`〜`h:m`」の組を取り出す。
//   - 入力に「／休憩なし」があれば休憩なし、「／休憩 」があれば2つ目の組を休憩とする。
//   - 「／」を持たない入力は、見つかった組のそれぞれを休憩なしの別々の入力として読む
//     （3-5-e の例示）。
//   - 期待の列に「弾く」があれば弾く行、無ければ許可の行として「N時間M分」を稼働時間とする。

// specRow は仕様書の表の1行から読んだ、時刻の組1つ分の入力と期待。
type specRow struct {
	ID string // 行番号（例: "3-5-a"）。1行から複数の入力を読むときは同じ ID が並ぶ
	// 開始時刻・終了時刻（時, 分）
	Start, End [2]int
	// 休憩（nil なら休憩なし）
	Break *[2][2]int
	// 弾く行か
	Reject bool
	// 許可の行の稼働時間（分）。読めなかった行は -1
	WantMinutes int
}

var (
	pairPattern     = regexp.MustCompile("`(\\d+):(\\d+)`〜`(\\d+):(\\d+)`")
	timePattern     = regexp.MustCompile("`(-?\\d+):(-?\\d+)`")
	durationPattern = regexp.MustCompile(`(\d+)時間(\d+)分`)
)

func specPath(t *testing.T) string {
	t.Helper()
	// internal/domain/workmonth → リポジトリのルートへ5階層
	return filepath.Join("..", "..", "..", "..", "..", "docs", "specs", "daily-record-entry.md")
}

// loadSpecSection は仕様書の heading の節（次の nextHeading の手前まで）から、表の行をセルに分けて返す。
func loadSpecSection(t *testing.T, heading, nextHeading string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(specPath(t))
	if err != nil {
		t.Fatalf("仕様書を読めない: %v", err)
	}
	text := string(raw)
	start := strings.Index(text, heading)
	if start < 0 {
		t.Fatalf("仕様書に %q の節が無い", heading)
	}
	end := strings.Index(text[start:], nextHeading)
	if end < 0 {
		t.Fatalf("仕様書に %q の節が無い", nextHeading)
	}
	section := text[start : start+end]

	var rows [][]string
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.Split(line, " | ")
		if len(cells) < 3 {
			continue
		}
		id := strings.TrimSpace(strings.TrimPrefix(cells[0], "|"))
		if id == "#" || strings.HasPrefix(id, "---") || id == "" {
			continue
		}
		rows = append(rows, []string{id, cells[1], strings.TrimSuffix(strings.Join(cells[2:], " | "), " |")})
	}
	return rows
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("整数として読めない: %q", s)
	}
	return n
}

// parseSpecRows は表の行から specRow を取り出す。時刻の組を含まない行は読み飛ばす。
func parseSpecRows(t *testing.T, rows [][]string) []specRow {
	t.Helper()
	var out []specRow
	for _, cells := range rows {
		id, input, expect := cells[0], cells[1], cells[2]
		pairs := pairPattern.FindAllStringSubmatch(input, -1)
		if len(pairs) == 0 {
			continue
		}
		toRange := func(m []string) ([2]int, [2]int) {
			return [2]int{atoi(t, m[1]), atoi(t, m[2])}, [2]int{atoi(t, m[3]), atoi(t, m[4])}
		}

		want := -1
		if m := durationPattern.FindStringSubmatch(expect); m != nil {
			want = atoi(t, m[1])*60 + atoi(t, m[2])
		}
		reject := strings.Contains(expect, "弾く")

		switch {
		case strings.Contains(input, "／休憩なし"):
			s, e := toRange(pairs[0])
			out = append(out, specRow{ID: id, Start: s, End: e, Reject: reject, WantMinutes: want})
		case strings.Contains(input, "／休憩 "):
			if len(pairs) < 2 {
				t.Fatalf("行 %s: 休憩の時刻の組を読めない: %q", id, input)
			}
			s, e := toRange(pairs[0])
			bs, be := toRange(pairs[1])
			out = append(out, specRow{ID: id, Start: s, End: e, Break: &[2][2]int{bs, be}, Reject: reject, WantMinutes: want})
		default:
			for _, p := range pairs {
				s, e := toRange(p)
				out = append(out, specRow{ID: id, Start: s, End: e, Reject: reject, WantMinutes: want})
			}
		}
	}
	return out
}

// loadDailyRecordSpecRows は AC-3 の 3-B・3-C の表（時刻の組の行）を読む。
func loadDailyRecordSpecRows(t *testing.T) []specRow {
	t.Helper()
	rows := parseSpecRows(t, loadSpecSection(t, "### AC-3.", "### AC-4."))
	// 3-A の行（3-1・3-2 等）は時刻1つの行で組を持たないので、ここには現れない。
	return rows
}

// loadReadBackSpecRows は AC-2 の 2-6-a〜2-6-c（保存した時刻の読み出し）を読む。
// 2-6-d は再入力（編集）の行で別のテストが扱う。2-6-b は稼働時間を書かない
// （WantMinutes が -1）。
func loadReadBackSpecRows(t *testing.T) []specRow {
	t.Helper()
	var picked [][]string
	for _, cells := range loadSpecSection(t, "### AC-2.", "### AC-3.") {
		switch cells[0] {
		case "2-6-a", "2-6-b", "2-6-c":
			picked = append(picked, cells)
		}
	}
	return parseSpecRows(t, picked)
}

// specTimeTokens は行の入力の列から、単独の時刻（`h:m`）をすべて読む。3-A の表用。
func specTimeTokens(t *testing.T, id string) [][2]int {
	t.Helper()
	var out [][2]int
	for _, cells := range loadSpecSection(t, "### AC-3.", "### AC-4.") {
		if cells[0] != id {
			continue
		}
		for _, m := range timePattern.FindAllStringSubmatch(cells[1], -1) {
			out = append(out, [2]int{atoi(t, m[1]), atoi(t, m[2])})
		}
	}
	return out
}

// requireRowIDs は、読んだ行に必要な行番号がすべて現れることを確かめる。
// 表の解析が空振りして 0 件の一致で緑になるのを防ぐ（陽性対照）。
func requireRowIDs(t *testing.T, rows []specRow, ids ...string) {
	t.Helper()
	seen := map[string]int{}
	for _, r := range rows {
		seen[r.ID]++
	}
	for _, id := range ids {
		if seen[id] == 0 {
			t.Fatalf("仕様書の表から行 %s を読めなかった（表の形が変わった、または解析の不具合）。読めた行: %v", id, seen)
		}
	}
}
