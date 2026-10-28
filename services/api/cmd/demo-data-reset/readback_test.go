package main

// 検証対象: docs/specs/demo-data-reset.md AC-4-3（デモデータの各勤務月・各稼働実績を gateway で
// 読み出したとき、ErrInvalidValue にならない＝P-16 の 500 を生まない）。検査の単位は AC-14-1。
//
// 手順: ハンドラを1回呼び、リセットの手段が Fake に書いた勤務月・稼働実績の行を記録する。
// その行を、WorkMonthRepository.Find（読み出しの gateway 経路）が受け取る行として与え直し、
// 全勤務月がエラーなく読み出せることを確かめる。実 DB を呼ばない（AC-13-4）。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// rbRows は記録した1行を Scan の宛先へ写す Fake。書き込みの引数は int、NULL 許容の列は *int を
// 取り得るため、宛先が **int のときは int も受ける。
type rbRows struct {
	rows [][]any
	pos  int
}

func (r *rbRows) Next() bool {
	if r.pos < len(r.rows) {
		r.pos++
		return true
	}
	return false
}

func (r *rbRows) Scan(dest ...any) error {
	row := r.rows[r.pos-1]
	if len(row) != len(dest) {
		return fmt.Errorf("rbRows: 列数が合わない（行 %d 列・宛先 %d）", len(row), len(dest))
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			v, ok := row[i].(string)
			if !ok {
				return fmt.Errorf("rbRows: 列 %d が string でない: %T", i, row[i])
			}
			*p = v
		case *int:
			v, ok := row[i].(int)
			if !ok {
				return fmt.Errorf("rbRows: 列 %d が int でない: %T", i, row[i])
			}
			*p = v
		case **int:
			switch v := row[i].(type) {
			case *int:
				*p = v
			case int:
				w := v
				*p = &w
			default:
				return fmt.Errorf("rbRows: 列 %d が int / *int でない: %T", i, row[i])
			}
		default:
			return fmt.Errorf("rbRows: 未対応の宛先 %T", d)
		}
	}
	return nil
}

func (r *rbRows) Close() error { return nil }
func (r *rbRows) Err() error   { return nil }

// rbStored は Fake に書かれた1つの勤務月（ヘッダ行と稼働実績の行）。
type rbStored struct {
	header  []any
	records [][]any
}

// rbDB は gateway.DB の読み出し側 Fake。Find は勤務月のヘッダ（work_months）と
// 稼働実績（daily_records）を別々に Query する。どちらかを問い合わせの表で見分け、呼び出しの
// 順序の状態を持たない（先の Find が途中で失敗しても次の Find に影響しない）。
type rbDB struct {
	stored map[string]rbStored
}

func rbKey(contractID string, year, month int) string {
	return fmt.Sprintf("%s/%04d-%02d", contractID, year, month)
}

func (db *rbDB) Query(_ context.Context, query string, args ...any) (gateway.Rows, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("rbDB: 引数は契約・年・月の3つのはず（got %d）", len(args))
	}
	id, _ := args[0].(string)
	y, _ := args[1].(int)
	m, _ := args[2].(int)
	s, ok := db.stored[rbKey(id, y, m)]
	if !ok {
		return &rbRows{}, nil
	}
	switch {
	case strings.Contains(query, "FROM daily_records"):
		return &rbRows{rows: s.records}, nil
	case strings.Contains(query, "FROM work_months"):
		return &rbRows{rows: [][]any{s.header}}, nil
	default:
		return nil, errors.New("rbDB: 想定しない問い合わせ")
	}
}

func (db *rbDB) Exec(context.Context, string, ...any) error {
	return errors.New("rbDB: 読み出しで Exec は想定しない")
}

func (db *rbDB) Begin(context.Context) (gateway.Tx, error) {
	return nil, errors.New("rbDB: 読み出しで Begin は想定しない")
}

// rbStoredFrom は1回の呼び出しの tail を勤務月ごとの行へ分ける。並びは
// 勤務月の行（12引数）→ 稼働実績の全削除（3引数）→ 稼働実績の行（先頭に契約識別子を持つ12引数）…。
// 勤務月の行と稼働実績の行は引数の数が同じなので、直後が全削除かどうかで見分ける。
func rbStoredFrom(t *testing.T, tail []rtCall) map[string]rbStored {
	t.Helper()
	out := map[string]rbStored{}
	var cur string
	for i, e := range tail {
		switch len(e.args) {
		case 3:
			// 稼働実績の全削除。
		case 12:
			if i+1 < len(tail) && len(tail[i+1].args) == 3 {
				cur = rbKey(e.args[0].(string), e.args[1].(int), e.args[2].(int))
				out[cur] = rbStored{header: e.args}
				continue
			}
			s, ok := out[cur]
			if !ok {
				t.Fatalf("勤務月の行より前に稼働実績の行がある: %v", e)
			}
			// 読み出しが受け取る稼働実績の行は、契約識別子を除く11列。
			s.records = append(s.records, e.args[1:])
			out[cur] = s
		default:
			t.Fatalf("想定しない引数の数: %d", len(e.args))
		}
	}
	return out
}

// AC-4-3: リセット日を変えても（月末・うるう日・年またぎ）、組み立てたデモデータの全勤務月が
// 読み出せる。
func TestDemoData_ReadsBackThroughGatewayWithoutErrInvalidValue(t *testing.T) {
	tests := []struct {
		name    string
		y, m, d int
	}{
		{"通常の日", 2026, 10, 27},
		{"月初", 2026, 11, 1},
		{"月末（31日）", 2026, 3, 31},
		{"うるう日", 2024, 2, 29},
		{"年初（過去の月が前年に入る）", 2026, 1, 1},
		{"年末", 2026, 12, 31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := startedRTHarness(t)
			// 協定世界時の正午は日本時間の同じ日の21時。
			h.now = time.Date(tt.y, time.Month(tt.m), tt.d, 12, 0, 0, 0, time.UTC)
			events, err := h.invoke(t, `{}`)
			if err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			stored := rbStoredFrom(t, parseInvocation(t, events, 2).tail)

			_, built := mustBuild2(t, tt.y, tt.m, tt.d)
			if len(built) == 0 {
				t.Fatalf("組み立てた勤務月が0件（テストが何も検証しない）")
			}
			if diff := cmp.Diff(len(built), len(stored)); diff != "" {
				t.Fatalf("保存された勤務月の数が組み立てと違う (-built +stored):\n%s", diff)
			}

			repo := gateway.NewWorkMonthRepository(&rbDB{stored: stored})
			for _, wm := range built {
				got, err := repo.Find(context.Background(), wm.ContractID(), wm.YearMonth())
				if errors.Is(err, workmonth.ErrInvalidValue) {
					t.Errorf("%s: 読み出しが ErrInvalidValue（P-16 の 500 になる）: %v",
						rbKey(wm.ContractID().String(), wm.YearMonth().Year(), wm.YearMonth().Month()), err)
					continue
				}
				if err != nil {
					t.Errorf("%s: 読み出しに失敗: %v",
						rbKey(wm.ContractID().String(), wm.YearMonth().Year(), wm.YearMonth().Month()), err)
					continue
				}
				if diff := cmp.Diff(
					[2]any{wm.State(), len(wm.DailyRecords())},
					[2]any{got.State(), len(got.DailyRecords())},
				); diff != "" {
					t.Errorf("読み出した勤務月が組み立てと違う（状態・稼働実績の件数）(-built +read):\n%s", diff)
				}
			}
		})
	}
}

func mustBuild2(t *testing.T, y, m, d int) ([]gateway.DemoContract, []*workmonth.WorkMonth) {
	t.Helper()
	date, err := workmonth.NewDate(y, m, d)
	if err != nil {
		t.Fatalf("NewDate failed: %v", err)
	}
	c, w, err := buildDemoData(date, rtEngineer)
	if err != nil {
		t.Fatalf("buildDemoData failed: %v", err)
	}
	return c, w
}
