package controller_test

import (
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// 妥当な値の組（テストの前提を作るためだけに使う。テストの主題ではない）。
const (
	testContractID = "ctr-0001"
	testActorID    = "eng-0001"
)

func mustContractID(t *testing.T, value string) workmonth.ContractID {
	t.Helper()
	id, err := workmonth.NewContractID(value)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewContractID(%q): %v", value, err)
	}
	return id
}

func mustYearMonth(t *testing.T, year, month int) workmonth.YearMonth {
	t.Helper()
	ym, err := workmonth.NewYearMonth(year, month)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewYearMonth(%d, %d): %v", year, month, err)
	}
	return ym
}

func mustDate(t *testing.T, year, month, day int) workmonth.Date {
	t.Helper()
	d, err := workmonth.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("前提の構築に失敗: NewDate(%d, %d, %d): %v", year, month, day, err)
	}
	return d
}
