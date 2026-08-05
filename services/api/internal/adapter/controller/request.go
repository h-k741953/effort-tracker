package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 本ファイルは全ハンドラが共有する構文検査（AC-9-6）と操作者ヘッダの写し
// （AC-9-7）を置く。業務ルールは持たない（AC-9-1「やらないこと」）。

// contractIDPattern は契約 AC-1-10 の書式（1〜64文字の英数字・アンダースコア・
// ハイフン）。workmonth.NewContractID は空文字しか弾かないため、書式の検査は
// controller が持つ（AC-9-6-a）。
var contractIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// yearMonthPattern・datePattern は `YYYY-MM` / `YYYY-MM-DD` の書式検査
// （AC-9-6-b・AC-9-6-c）。暦上の妥当性は workmonth.NewYearMonth / NewDate に委ねる。
var (
	yearMonthPattern = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
	datePattern      = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
)

// parseContractID は契約識別子をパス値から構築する（AC-9-6-a）。
// workmonth.ErrInvalidValue を要求側の識別子（ErrInvalidRequest）へ変換して
// から返す（AC-9-9-b・AC-9-9-c。%w に含めるのは ErrInvalidRequest だけ）。
func parseContractID(value string) (workmonth.ContractID, error) {
	if !contractIDPattern.MatchString(value) {
		return workmonth.ContractID{}, fmt.Errorf("%w: invalid contractId format: %q", ErrInvalidRequest, value)
	}
	id, err := workmonth.NewContractID(value)
	if err != nil {
		return workmonth.ContractID{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return id, nil
}

// parseYearMonth は年月をパス値から構築する（AC-9-6-b）。
func parseYearMonth(value string) (workmonth.YearMonth, error) {
	m := yearMonthPattern.FindStringSubmatch(value)
	if m == nil {
		return workmonth.YearMonth{}, fmt.Errorf("%w: invalid yearMonth format: %q", ErrInvalidRequest, value)
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	ym, err := workmonth.NewYearMonth(year, month)
	if err != nil {
		return workmonth.YearMonth{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return ym, nil
}

// parseDate は対象日をパス値から構築する（AC-9-6-c）。
func parseDate(value string) (workmonth.Date, error) {
	m := datePattern.FindStringSubmatch(value)
	if m == nil {
		return workmonth.Date{}, fmt.Errorf("%w: invalid date format: %q", ErrInvalidRequest, value)
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	d, err := workmonth.NewDate(year, month, day)
	if err != nil {
		return workmonth.Date{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return d, nil
}

// 契約 D-2・AC-1-4 の操作者ヘッダ名。
const (
	headerActorID   = "X-Actor-Id"
	headerActorRole = "X-Actor-Role"
)

// parseRole は X-Actor-Role の値を port.Role へ変換する（契約 AC-1-5。
// 大文字小文字を含めて一致させる）。
func parseRole(value string) (port.Role, bool) {
	switch port.Role(value) {
	case port.RoleEngineer, port.RoleApprover:
		return port.Role(value), true
	default:
		return "", false
	}
}

// actorHeaderValues は操作者ヘッダ2本の生値を返す。
func actorHeaderValues(r *http.Request) (id, role string) {
	return r.Header.Get(headerActorID), r.Header.Get(headerActorRole)
}

// buildActorAllowGuest は参照系（E-1・`engineerId` 指定の一覧）向けに操作者
// ヘッダを Actor へ写す（AC-9-7-a②・AC-9-7-d）。両ヘッダ不在は弾かず、未認証の
// Actor を返す。片方だけ・ロール値が2値以外は要求の構文不正として弾く
// （AC-9-7-c）。
func buildActorAllowGuest(r *http.Request) (port.Actor, error) {
	id, role := actorHeaderValues(r)
	if id == "" && role == "" {
		return port.Actor{}, nil
	}
	if id == "" || role == "" {
		return port.Actor{}, fmt.Errorf("%w: actor headers must be present together", ErrInvalidRequest)
	}
	roleValue, ok := parseRole(role)
	if !ok {
		return port.Actor{}, fmt.Errorf("%w: invalid actor role: %q", ErrInvalidRequest, role)
	}
	return port.Actor{ID: id, Role: roleValue, Authenticated: true}, nil
}

// requireActorHeader は更新系（E-3〜E-7）・承認待ち一覧（`engineerId` 省略の
// E-2）向けに操作者ヘッダを Actor へ写す（決定10・AC-9-7-a①）。**ヘッダの
// 有無の判定を他のどの構文検査よりも前に置く**ため、呼び出し元（各ハンドラ）は
// 本関数を最初に呼ぶこと。両ヘッダ不在は port.ErrUnauthenticated（構文不正の
// 識別子＝ErrInvalidRequest とは異なる番兵。AC-9-9-a）を返し、入力 DTO を組み
// 立てさせない。片方だけ・ロール値が2値以外は ErrInvalidRequest。
func requireActorHeader(r *http.Request) (port.Actor, error) {
	id, role := actorHeaderValues(r)
	if id == "" && role == "" {
		return port.Actor{}, port.ErrUnauthenticated
	}
	if id == "" || role == "" {
		return port.Actor{}, fmt.Errorf("%w: actor headers must be present together", ErrInvalidRequest)
	}
	roleValue, ok := parseRole(role)
	if !ok {
		return port.Actor{}, fmt.Errorf("%w: invalid actor role: %q", ErrInvalidRequest, role)
	}
	return port.Actor{ID: id, Role: roleValue, Authenticated: true}, nil
}
