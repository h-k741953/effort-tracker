package main

// 検証対象: docs/specs/demo-data-reset.md の単位2「cmd の起動と手順」（AC-14-2 のうち cmd に掛かるもの）。
// AC-7-4（デモの技術者識別子の環境変数が未設定・空）・AC-10（DDL の適用の順序・失敗）・
// AC-11（11-1 リセット日・11-3 失敗でロールバック・11-6 文言・11-7 接続文字列の取得・11-8 同じ日の再実行）・
// AC-6-2/6-3/6-5/6-7（cmd が gateway へ渡す値と手順）・AC-1（破棄が選り分けない）。
//
// 実 DB・実 AWS・実時計を呼ばない。SQL 実行の受け口（gateway.DB）・接続文字列の解決・環境変数の探索・
// 時計・ランタイムへの登録を、すべて手書きのインメモリ Fake へ差し替える（infra-terraform.md AC-8-11・
// AC-9-12 と同型。ADR 0007）。プロセスの環境変数を書き換えない。
//
// 本ファイルが tester として定めた署名（実装が満たす）:
//
//	type Handler func(ctx context.Context, event json.RawMessage) error
//
//	func Run(
//	    resolve  func(ctx context.Context, parameterName string) (string, error),
//	    lookup   func(name string) (string, bool),
//	    connect  func(ctx context.Context, cfg persistence.Config) (gateway.DB, error),
//	    now      func() time.Time,
//	    build    func(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error),
//	    register func(h Handler),
//	) error
//
//	const neonConnectionStringParameterNameEnv = ...  // SSM パラメータ名の環境変数の名前（cmd/bootstrap と同じ形＝AC-5-4）
//	const demoEngineerIDEnv = ...                     // デモの技術者識別子の環境変数の名前（名前の持ち主は構成側＝AC-7-3）
//
// 環境変数の名前は、テストの文字列に書かず、実装が定める定数を名指す（名前の持ち主を二重にしない＝AC-7-3）。
//
// build は組み立ての関数の差し替え点（AC-9-3-a。2026-10-27 人間が決定）。引数の型は buildDemoData と同じで、
// 本番の配線は buildDemoData を渡す。ハンドラの呼び出しのたびに、リセット日（AC-11-1）とデモの技術者識別子
// （AC-7-1）を渡して呼ぶ。
//
// Run はコールドスタートの手続き（環境変数の探索 → 接続文字列の解決（ちょうど1回）→ 設定の組み立て →
// 接続の確立（ちょうど1回）→ 登録（ちょうど1回））で、DB に触れない。DDL の適用・破棄・投入は、
// 登録したハンドラの1回の呼び出しの中で行い、時計はその呼び出しのたびに読む（AC-11-1）。
// DDL は Tx の外の DB.Exec（引数なし）で、Begin より前に適用する（AC-10-5）。
// 破棄と投入と契約期間の書き換えは gateway の DemoResetter.Reset が1つの Tx で行う（AC-15）。
//
// 担保しないもの（AC-14-7）: AC-12-11 の import の範囲、DDL の全文・冪等性そのもの（AC-10-6・AC-13-4）、
// ログの出力先への書き込み、AC-13 の各行。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/persistence"
)

const (
	// 実在しうる接続文字列・パラメータ名・技術者識別子を書かない（ダミー値）。
	rtParamName     = "dummy-ssm-parameter-name-not-real"
	rtConnString    = "dummy-connection-string-not-real"
	rtEngineer      = "demo-engineer-not-a-uuid"
	rtOtherEngineer = "someone-else-must-not-leak"
)

var rtNil = (*int)(nil)

// ---- 手書きの Fake ---------------------------------------------------------

// rtCall は DB・Tx への1回の呼び出し。kind は "db.exec" / "db.query" / "begin" /
// "tx.query" / "tx.exec" / "commit" / "rollback"。
type rtCall struct {
	kind  string
	query string
	args  []any
}

type rtRecorder struct{ events []rtCall }

func (r *rtRecorder) add(kind, query string, args []any) {
	r.events = append(r.events, rtCall{kind, query, args})
}

type rtRows struct {
	engineer string
	has      bool
	advanced bool
}

func (r *rtRows) Next() bool {
	if r.has && !r.advanced {
		r.advanced = true
		return true
	}
	return false
}

func (r *rtRows) Scan(dest ...any) error {
	if len(dest) != 1 {
		return fmt.Errorf("rtRows: 契約の照会の応答は技術者識別子の1列のはず（dest=%d）", len(dest))
	}
	p, ok := dest[0].(*string)
	if !ok {
		return fmt.Errorf("rtRows: dest が *string でない: %T", dest[0])
	}
	*p = r.engineer
	return nil
}

func (r *rtRows) Close() error { return nil }
func (r *rtRows) Err() error   { return nil }

// rtTx は gateway.Tx の Fake。txFailAt は Tx の中の文（Query と Exec を通した番号。0 始まり）。
type rtTx struct {
	rec       *rtRecorder
	engineer  func(contractID string) (string, bool)
	stmts     int
	txFailAt  int
	failErr   error
	commitErr error
}

func (tx *rtTx) Query(_ context.Context, query string, args ...any) (gateway.Rows, error) {
	idx := tx.stmts
	tx.stmts++
	tx.rec.add("tx.query", query, args)
	if idx == tx.txFailAt {
		return nil, tx.failErr
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("rtTx: 契約の照会の引数は契約識別子の1つのはず（got %d）", len(args))
	}
	id, _ := args[0].(string)
	engineer, has := tx.engineer(id)
	return &rtRows{engineer: engineer, has: has}, nil
}

func (tx *rtTx) Exec(_ context.Context, query string, args ...any) error {
	idx := tx.stmts
	tx.stmts++
	tx.rec.add("tx.exec", query, args)
	if idx == tx.txFailAt {
		return tx.failErr
	}
	return nil
}

func (tx *rtTx) Commit(_ context.Context) error {
	tx.rec.add("commit", "", nil)
	return tx.commitErr
}

func (tx *rtTx) Rollback(_ context.Context) error {
	tx.rec.add("rollback", "", nil)
	return nil
}

// rtDB は gateway.DB の Fake。ddlFailAt は Tx の外の Exec（DDL）の番号（0 始まり）。
type rtDB struct {
	rec       *rtRecorder
	tx        *rtTx
	ddlCount  int
	ddlFailAt int
	failErr   error
}

func (db *rtDB) Query(_ context.Context, query string, args ...any) (gateway.Rows, error) {
	db.rec.add("db.query", query, args)
	return nil, errors.New("rtDB: Tx の外の Query は想定しない")
}

func (db *rtDB) Exec(_ context.Context, query string, args ...any) error {
	idx := db.ddlCount
	db.ddlCount++
	db.rec.add("db.exec", query, args)
	if idx == db.ddlFailAt {
		return db.failErr
	}
	return nil
}

func (db *rtDB) Begin(_ context.Context) (gateway.Tx, error) {
	db.rec.add("begin", "", nil)
	return db.tx, nil
}

// rtHarness は Run に渡す差し替えと、その記録をまとめる。
type rtHarness struct {
	rec *rtRecorder
	db  *rtDB
	tx  *rtTx
	now time.Time

	env          map[string]string
	resolveErr   error
	resolveValue string
	connectErr   error
	// build が nil なら本番の組み立て（buildDemoData）を渡す。
	build      func(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error)
	buildCalls int

	order         []string
	resolveCalls  int
	resolveNames  []string
	connectCalls  int
	registerCalls int
	handler       Handler
}

func newRTHarness() *rtHarness {
	rec := &rtRecorder{}
	tx := &rtTx{rec: rec, txFailAt: -1}
	h := &rtHarness{
		rec: rec,
		tx:  tx,
		db:  &rtDB{rec: rec, tx: tx, ddlFailAt: -1},
		// 既定は JST 2026-10-27 00:30。
		now: time.Date(2026, 10, 26, 15, 30, 0, 0, time.UTC),
		env: map[string]string{
			neonConnectionStringParameterNameEnv: rtParamName,
			demoEngineerIDEnv:                    rtEngineer,
		},
		resolveValue: rtConnString,
	}
	// 既定: デモの契約は既にあり、技術者識別子は一致している。
	tx.engineer = func(string) (string, bool) { return rtEngineer, true }
	return h
}

func (h *rtHarness) run() error {
	return Run(
		func(_ context.Context, name string) (string, error) {
			h.resolveCalls++
			h.order = append(h.order, "resolve")
			h.resolveNames = append(h.resolveNames, name)
			if h.resolveErr != nil {
				return "", h.resolveErr
			}
			return h.resolveValue, nil
		},
		func(name string) (string, bool) {
			v, ok := h.env[name]
			return v, ok
		},
		func(_ context.Context, _ persistence.Config) (gateway.DB, error) {
			h.connectCalls++
			h.order = append(h.order, "connect")
			if h.connectErr != nil {
				return nil, h.connectErr
			}
			return h.db, nil
		},
		func() time.Time { return h.now },
		func(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error) {
			h.buildCalls++
			if h.build != nil {
				return h.build(resetDate, engineerID)
			}
			return buildDemoData(resetDate, engineerID)
		},
		func(handler Handler) {
			h.registerCalls++
			h.order = append(h.order, "register")
			h.handler = handler
		},
	)
}

// invoke は登録されたハンドラを1回呼ぶ。記録と失敗の注入の番号は、呼び出しごとに数え直す。
func (h *rtHarness) invoke(t *testing.T, event string) ([]rtCall, error) {
	t.Helper()
	if h.handler == nil {
		t.Fatalf("ハンドラが登録されていない")
	}
	h.rec.events = nil
	h.db.ddlCount = 0
	h.tx.stmts = 0
	h.buildCalls = 0
	var raw json.RawMessage
	if event != "" {
		raw = json.RawMessage(event)
	}
	err := h.handler(context.Background(), raw)
	return append([]rtCall(nil), h.rec.events...), err
}

// started は Run が成功しハンドラが登録された状態のハーネスを返す。
func startedRTHarness(t *testing.T) *rtHarness {
	t.Helper()
	h := newRTHarness()
	if err := h.run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	return h
}

// ---- コールドスタート（AC-7-4・AC-11-7） --------------------------------------

func TestRun_ColdStart_RegistersOnceAfterResolveAndConnectWithoutTouchingDB(t *testing.T) {
	h := newRTHarness()
	if err := h.run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if diff := cmp.Diff([]string{"resolve", "connect", "register"}, h.order); diff != "" {
		t.Errorf("解決 → 接続 → 登録の順・各ちょうど1回 (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{rtParamName}, h.resolveNames); diff != "" {
		t.Errorf("解決へ渡るのは環境変数が持つパラメータ名 (-want +got):\n%s", diff)
	}
	if len(h.rec.events) != 0 {
		t.Errorf("コールドスタートで DB に触れてはならない（DDL・破棄・投入はハンドラの中）: %v", h.rec.events)
	}
}

func TestRun_ColdStart_FailureRegistersNothing(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *rtHarness)
		// 以下は、その失敗で「呼んではならないもの」。
		noResolve bool
		noConnect bool
	}{
		{"デモの技術者識別子の環境変数が未設定（AC-7-4）", func(h *rtHarness) { delete(h.env, demoEngineerIDEnv) }, false, true},
		{"デモの技術者識別子の環境変数が空（AC-7-4。既定値へ黙って落ちない）", func(h *rtHarness) { h.env[demoEngineerIDEnv] = "" }, false, true},
		{"SSM パラメータ名の環境変数が未設定（AC-11-7・AC-5-4）", func(h *rtHarness) { delete(h.env, neonConnectionStringParameterNameEnv) }, true, true},
		{"SSM パラメータ名の環境変数が空", func(h *rtHarness) { h.env[neonConnectionStringParameterNameEnv] = "" }, true, true},
		{"接続文字列の取得・復号に失敗（AC-11-7）", func(h *rtHarness) { h.resolveErr = errors.New("resolve failed") }, false, true},
		{"接続文字列が空で返る", func(h *rtHarness) { h.resolveValue = "" }, false, true},
		{"接続の確立に失敗", func(h *rtHarness) { h.connectErr = errors.New("connect failed") }, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRTHarness()
			tt.setup(h)
			err := h.run()
			if err == nil {
				t.Fatalf("Run はエラーを返さなければならない（ハンドラを登録せずにコールドスタートで失敗させる）")
			}
			if h.registerCalls != 0 {
				t.Errorf("ハンドラの登録回数 = %d, want 0", h.registerCalls)
			}
			if tt.noResolve && h.resolveCalls != 0 {
				t.Errorf("解決の呼び出し回数 = %d, want 0", h.resolveCalls)
			}
			if tt.noConnect && h.connectCalls != 0 {
				t.Errorf("接続の呼び出し回数 = %d, want 0", h.connectCalls)
			}
			if len(h.rec.events) != 0 {
				t.Errorf("DB に触れてはならない: %v", h.rec.events)
			}
			for _, secret := range []string{rtParamName, rtConnString} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("エラーの文言に接続文字列またはパラメータ名が含まれている（AC-11-6）: %q", err.Error())
				}
			}
		})
	}
}

// 接続文字列は、コールドスタートで1度だけ取得する（AC-11-7）。ハンドラの呼び出しのたびに取得・接続しない。
func TestHandler_DoesNotResolveOrConnectAgainPerInvocation(t *testing.T) {
	h := startedRTHarness(t)
	for i := 0; i < 3; i++ {
		if _, err := h.invoke(t, `{}`); err != nil {
			t.Fatalf("invoke #%d failed: %v", i, err)
		}
	}
	if h.resolveCalls != 1 || h.connectCalls != 1 || h.registerCalls != 1 {
		t.Errorf("解決=%d 接続=%d 登録=%d, want 各1回", h.resolveCalls, h.connectCalls, h.registerCalls)
	}
}

// ---- 手順の解析 ------------------------------------------------------------

// parsedInvocation は 1 回のハンドラ呼び出しの記録を、手順の段に切り分けたもの。
type parsedInvocation struct {
	ddl      []rtCall // Begin より前の DB.Exec
	txStmts  []rtCall // Tx の中の Query と Exec（出現順）
	begins   int
	commits  int
	rollback int
	head     []rtCall // ① 契約の照会と挿入／書き換え
	discards []rtCall // ② 引数を持たない Exec
	tail     []rtCall // ③ 集約の保存の Exec
}

func parseInvocation(t *testing.T, events []rtCall, contracts int) parsedInvocation {
	t.Helper()
	var p parsedInvocation
	seenBegin := false
	for _, e := range events {
		switch e.kind {
		case "db.exec":
			if seenBegin {
				t.Errorf("Begin より後に Tx の外の Exec がある（DDL は破棄と投入より前に適用する＝AC-10-5）")
			}
			p.ddl = append(p.ddl, e)
		case "begin":
			seenBegin = true
			p.begins++
		case "tx.query", "tx.exec":
			p.txStmts = append(p.txStmts, e)
		case "commit":
			p.commits++
		case "rollback":
			p.rollback++
		default:
			t.Errorf("想定しない呼び出し: %v", e)
		}
	}
	headLen := 2 * contracts
	if len(p.txStmts) < headLen {
		return p
	}
	p.head = p.txStmts[:headLen]
	rest := p.txStmts[headLen:]
	i := 0
	for i < len(rest) && rest[i].kind == "tx.exec" && len(rest[i].args) == 0 {
		i++
	}
	p.discards, p.tail = rest[:i], rest[i:]
	return p
}

// contractWrites は ① の Exec（挿入 11引数／書き換え 5引数）だけを返す。
func contractWrites(p parsedInvocation) []rtCall {
	var out []rtCall
	for _, e := range p.head {
		if e.kind == "tx.exec" {
			out = append(out, e)
		}
	}
	return out
}

// periodOfWrite は ① の Exec から契約期間の4列（開始の年・月、終了の年・月 *int）を取り出す。
func periodOfWrite(t *testing.T, e rtCall) []any {
	t.Helper()
	switch len(e.args) {
	case 5:
		return e.args[1:]
	case 11:
		return e.args[7:]
	default:
		t.Fatalf("契約への Exec の引数が5個でも11個でもない: %d", len(e.args))
		return nil
	}
}

// ---- 正常系の手順（AC-10-5・AC-11-2・AC-1・AC-6-3・AC-6-7・AC-6-11） ------------

func TestHandler_Success_StepOrder(t *testing.T) {
	h := startedRTHarness(t)
	events, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	p := parseInvocation(t, events, 2)

	if len(p.ddl) < 1 {
		t.Errorf("DDL の適用が無い（毎回のリセットで、破棄と投入より前に適用する＝AC-10-5）")
	}
	for i, e := range p.ddl {
		if strings.TrimSpace(e.query) == "" {
			t.Errorf("DDL #%d の文が空", i)
		}
	}
	if p.begins != 1 || p.commits != 1 || p.rollback != 0 {
		t.Errorf("Begin=%d Commit=%d Rollback=%d, want 1/1/0（破棄と投入は1つのトランザクション＝AC-11-2）", p.begins, p.commits, p.rollback)
	}
	if last := events[len(events)-1]; last.kind != "commit" {
		t.Errorf("最後の呼び出しは commit のはず: %v", last)
	}
	if len(p.head) != 4 || p.head[0].kind != "tx.query" || p.head[1].kind != "tx.exec" || p.head[2].kind != "tx.query" || p.head[3].kind != "tx.exec" {
		t.Fatalf("① 契約ごとの [照会, Exec] の2組が先頭に無い: %v", p.txStmts)
	}
	// 既に契約がある状態: 期間の4列だけを書き換える（5引数。AC-6-3・AC-6-8）。
	for _, e := range contractWrites(p) {
		if len(e.args) != 5 {
			t.Errorf("既存の契約への Exec は期間の4列だけの5引数のはず（got %d。AC-6-3・AC-6-8）", len(e.args))
		}
	}
	if len(p.discards) < 1 {
		t.Errorf("② 引数を持たない破棄の Exec が無い（全行を選り分けずに破棄する＝AC-1-1・AC-1-2）")
	}
	if len(p.tail) == 0 {
		t.Errorf("③ 集約の保存の Exec が無い")
	}
}

// 契約が無いとき、デモの契約を挿入する（AC-6-2）。値は V-2。リセット日は 2026-10-27。
func TestHandler_Success_InsertsDemoContractsWhenAbsent(t *testing.T) {
	h := startedRTHarness(t)
	h.tx.engineer = func(string) (string, bool) { return "", false }
	events, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	p := parseInvocation(t, events, 2)

	type inserted struct {
		Name     string
		Engineer string
		Range    [4]int
		Period   []any
	}
	got := map[string]inserted{}
	for _, e := range contractWrites(p) {
		if len(e.args) != 11 {
			t.Fatalf("契約が無いときの Exec は11引数の挿入のはず（got %d）", len(e.args))
		}
		name, _ := e.args[1].(string)
		engineer, _ := e.args[2].(string)
		got[name] = inserted{
			Name: name, Engineer: engineer,
			Range:  [4]int{e.args[3].(int), e.args[4].(int), e.args[5].(int), e.args[6].(int)},
			Period: e.args[7:],
		}
	}
	want := map[string]inserted{
		"基幹システム更改": {"基幹システム更改", rtEngineer, [4]int{140, 0, 180, 0}, []any{2026, 4, rtNil, rtNil}},
		"運用保守":     {"運用保守", rtEngineer, [4]int{20, 0, 40, 0}, []any{2026, 7, intP(2026), intP(9)}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("挿入するデモの契約 (-want +got):\n%s", diff)
	}
}

func intP(v int) *int { return &v }

// ---- リセット日（AC-11-1・AC-6-7-a〜6-7-c） -------------------------------------

func TestHandler_ResetDateIsTheTokyoDateOfTheClock(t *testing.T) {
	jst := time.FixedZone("JST", 9*3600)
	pst := time.FixedZone("PST", -8*3600)
	tests := []struct {
		name         string
		now          time.Time
		aStart       []any // 開始の年・月
		bPeriod      []any // 開始の年・月、終了の年・月
		wantCurrentM int   // 当月（Draft）の月
	}{
		{"UTC 15:30 は JST の翌日 00:30（2026-10-27）", time.Date(2026, 10, 26, 15, 30, 0, 0, time.UTC),
			[]any{2026, 4}, []any{2026, 7, intP(2026), intP(9)}, 10},
		{"UTC 14:59:59 は JST の当日 23:59:59（2026-10-31）", time.Date(2026, 10, 31, 14, 59, 59, 0, time.UTC),
			[]any{2026, 4}, []any{2026, 7, intP(2026), intP(9)}, 10},
		{"UTC 15:00:00 は JST 2026-11-01 00:00（月が切り替わる）", time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC),
			[]any{2026, 5}, []any{2026, 8, intP(2026), intP(10)}, 11},
		{"JST で 2026-03-15（N か月前が前年）", time.Date(2026, 3, 15, 4, 0, 0, 0, jst),
			[]any{2025, 9}, []any{2025, 12, intP(2026), intP(2)}, 3},
		{"UTC 2026-12-31 15:00 は JST 2027-01-01（年をまたぐ）", time.Date(2026, 12, 31, 15, 0, 0, 0, time.UTC),
			[]any{2026, 7}, []any{2026, 10, intP(2026), intP(12)}, 1},
		{"UTC-8 の時刻も Asia/Tokyo の日付へ換算する（2026-10-31 20:00 PST は JST 2026-11-01 13:00）", time.Date(2026, 10, 31, 20, 0, 0, 0, pst),
			[]any{2026, 5}, []any{2026, 8, intP(2026), intP(10)}, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := startedRTHarness(t)
			h.now = tt.now
			events, err := h.invoke(t, `{}`)
			if err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			p := parseInvocation(t, events, 2)

			var gotA, gotB []any
			for _, e := range contractWrites(p) {
				period := periodOfWrite(t, e)
				if period[2] == any(rtNil) { // 終了未定が A
					gotA = period[:2]
				} else {
					gotB = period
				}
			}
			if diff := cmp.Diff(tt.aStart, gotA); diff != "" {
				t.Errorf("A の契約期間の開始 (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.bPeriod, gotB); diff != "" {
				t.Errorf("B の契約期間 (-want +got):\n%s", diff)
			}
			// 当月の勤務月（Draft）が、リセット日の月で保存されている。
			foundCurrent := false
			for _, e := range p.tail {
				if len(e.args) == 12 && e.args[7] == "Draft" && e.args[1] == tt.now.In(jst).Year() {
					if e.args[2] == tt.wantCurrentM {
						foundCurrent = true
					}
				}
			}
			if !foundCurrent {
				t.Errorf("当月 (%d月) の Draft の勤務月の保存が無い", tt.wantCurrentM)
			}
		})
	}
}

// 時計はハンドラの呼び出しのたびに読む（コールドスタートで1度読んで使い回さない）。
func TestHandler_ReadsTheClockOnEveryInvocation(t *testing.T) {
	h := startedRTHarness(t)

	aStartOf := func(events []rtCall) []any {
		p := parseInvocation(t, events, 2)
		for _, e := range contractWrites(p) {
			if period := periodOfWrite(t, e); period[2] == any(rtNil) {
				return period[:2]
			}
		}
		t.Fatalf("終了未定の契約への Exec が無い")
		return nil
	}

	h.now = time.Date(2026, 10, 26, 15, 30, 0, 0, time.UTC) // JST 2026-10-27
	first, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	h.now = time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC) // JST 2026-11-01
	second, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	if diff := cmp.Diff([]any{2026, 4}, aStartOf(first)); diff != "" {
		t.Errorf("1回目 (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]any{2026, 5}, aStartOf(second)); diff != "" {
		t.Errorf("2回目は時計の進みに従う (-want +got):\n%s", diff)
	}
}

// イベントの本文からリセット日を読まない（AC-11-1）。同じ日に何度実行しても、同じ文の並びになる（AC-11-8）。
func TestHandler_IgnoresEventBodyAndIsRepeatable(t *testing.T) {
	h := startedRTHarness(t)
	baseline, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	events := map[string]string{
		"同じ本文を再実行（AC-11-8）":  `{}`,
		"本文が null":           `null`,
		"本文が空":               ``,
		"本文に別の日付を持つ（読まない）":   `{"date":"1999-01-01","reset_date":"2030-12-31","time":"2030-12-31T00:00:00Z"}`,
		"Scheduler の本文の形に近い": `{"version":"0","source":"aws.scheduler","time":"2031-05-05T19:00:00Z"}`,
	}
	for name, body := range events {
		t.Run(name, func(t *testing.T) {
			got, err := h.invoke(t, body)
			if err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			if diff := cmp.Diff(baseline, got, cmp.AllowUnexported(rtCall{})); diff != "" {
				t.Errorf("文の並びが基準と異なる (-baseline +got):\n%s", diff)
			}
		})
	}
}

// ---- 組み立てた集約が保存される（AC-6-4・AC-9・V-8 の配線） ---------------------

// V-8（リセット日 2026-10-27）の勤務月が、保存の Exec に現れる。契約は期間の形（終了未定が A）で区別する。
func TestHandler_Success_SavesTheAssembledWorkMonths(t *testing.T) {
	h := startedRTHarness(t)
	events, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	p := parseInvocation(t, events, 2)

	idOfA, idOfB := "", ""
	for _, e := range contractWrites(p) {
		id, _ := e.args[0].(string)
		if periodOfWrite(t, e)[2] == any(rtNil) {
			idOfA = id
		} else {
			idOfB = id
		}
	}
	if idOfA == "" || idOfB == "" || idOfA == idOfB {
		t.Fatalf("契約の識別子を A・B で区別できない: A=%q B=%q", idOfA, idOfB)
	}

	// 勤務月のヘッダ行は、全削除（3引数）の直前の12引数の Exec。
	type header struct {
		Contract string
		YM       string
		State    string
		Excess   *[2]int
		Short    *[2]int
	}
	toPair := func(h, m any) *[2]int {
		hp, mp := h.(*int), m.(*int)
		if hp == nil || mp == nil {
			return nil
		}
		return &[2]int{*hp, *mp}
	}
	var got []header
	for i, e := range p.tail {
		if len(e.args) != 3 {
			continue
		}
		if i == 0 || len(p.tail[i-1].args) != 12 {
			t.Fatalf("稼働実績の全削除（3引数）の直前が勤務月の行（12引数）でない: %v", p.tail[i-1])
		}
		hd := p.tail[i-1].args
		who := "?"
		switch hd[0] {
		case idOfA:
			who = "A"
		case idOfB:
			who = "B"
		}
		got = append(got, header{who, fmt.Sprintf("%04d-%02d", hd[1], hd[2]), hd[7].(string), toPair(hd[8], hd[9]), toPair(hd[10], hd[11])})
	}
	zero := &[2]int{0, 0}
	want := []header{
		{"A", "2026-05", "Approved", zero, zero},
		{"A", "2026-06", "Approved", &[2]int{23, 30}, zero},
		{"A", "2026-07", "Approved", zero, zero},
		{"A", "2026-08", "Approved", zero, &[2]int{16, 45}},
		{"A", "2026-09", "PendingApproval", zero, zero},
		{"A", "2026-10", "Draft", nil, nil},
		{"B", "2026-07", "Approved", zero, zero},
		{"B", "2026-08", "Approved", zero, zero},
		{"B", "2026-09", "Approved", zero, zero},
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].Contract != got[j].Contract {
			return got[i].Contract < got[j].Contract
		}
		return got[i].YM < got[j].YM
	})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("保存された勤務月（V-8。契約・年月・状態・超過／不足）(-want +got):\n%s", diff)
	}
}

// ---- DDL（AC-10-3・AC-10-4・AC-10-5） -------------------------------------------

// DDL の全文は固定しない（AC-10-6）。固定するのは「作り直さない」「無ければ足す形」の字面の最小限だけ。
func TestHandler_DDLDoesNotRecreateTables(t *testing.T) {
	h := startedRTHarness(t)
	events, err := h.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	p := parseInvocation(t, events, 2)
	if len(p.ddl) == 0 {
		t.Fatalf("DDL が適用されていない")
	}
	for i, e := range p.ddl {
		text := strings.Join(strings.Fields(strings.ToUpper(e.query)), " ")
		if strings.Contains(text, "DROP TABLE") {
			t.Errorf("DDL #%d にテーブルの削除がある（契約のテーブルを作り直さない＝AC-10-4）", i)
		}
		for _, clause := range []string{"CREATE TABLE", "ADD COLUMN"} {
			for rest := text; ; {
				idx := strings.Index(rest, clause)
				if idx < 0 {
					break
				}
				rest = rest[idx+len(clause):]
				if !strings.HasPrefix(rest, " IF NOT EXISTS") {
					t.Errorf("DDL #%d の %s が IF NOT EXISTS を伴わない（冪等に書く＝AC-10-3・AC-10-4）", i, clause)
				}
			}
		}
	}
}

// ---- 失敗（AC-10-5・AC-11-3・AC-6-5・AC-11-6） -----------------------------------

func TestHandler_Failure_RollsBackAndStops(t *testing.T) {
	baselineHarness := startedRTHarness(t)
	baselineEvents, err := baselineHarness.invoke(t, `{}`)
	if err != nil {
		t.Fatalf("baseline handler failed: %v", err)
	}
	base := parseInvocation(t, baselineEvents, 2)
	ddlCount := len(base.ddl)
	headLen := 2 * 2
	discardEnd := headLen + len(base.discards)
	lastTx := len(base.txStmts) - 1

	t.Run("DDL の失敗は、破棄と投入へ進まず失敗として終える（AC-10-5）", func(t *testing.T) {
		for _, at := range []int{0, ddlCount - 1} {
			injected := errors.New("injected ddl failure")
			h := startedRTHarness(t)
			h.db.ddlFailAt, h.db.failErr = at, injected
			events, err := h.invoke(t, `{}`)
			if err == nil || !errors.Is(err, injected) {
				t.Fatalf("DDL #%d の失敗でハンドラはエラーを返さなければならない: %v", at, err)
			}
			if got, want := len(events), at+1; got != want {
				t.Errorf("DDL #%d の失敗後の呼び出し数 = %d, want %d（以降の DDL・Begin・破棄・投入を行わない）", at, got, want)
			}
			for _, e := range events {
				if e.kind != "db.exec" {
					t.Errorf("DDL の失敗の後に %s が呼ばれた", e.kind)
				}
			}
			assertNoLeak(t, err, events)
		}
	})

	txPositions := []struct {
		name string
		at   int
	}{
		{"①の1件目の契約（照会）", 0},
		{"①の1件目の契約（書き換え）", 1},
		{"②（最初の破棄）", headLen},
		{"②（最後の破棄）", discardEnd - 1},
		{"③の1件目の集約", discardEnd},
		{"③の途中", discardEnd + (lastTx-discardEnd)/2},
		{"③の最後", lastTx},
	}
	for _, p := range txPositions {
		t.Run(p.name+"の失敗はロールバックし、前日のデータを残す（AC-11-3）", func(t *testing.T) {
			injected := errors.New("injected tx failure")
			h := startedRTHarness(t)
			h.tx.txFailAt, h.tx.failErr = p.at, injected
			events, err := h.invoke(t, `{}`)
			if err == nil || !errors.Is(err, injected) {
				t.Fatalf("ハンドラはエラーを返さなければならない: %v", err)
			}
			parsed := parseInvocation(t, events, 2)
			if len(parsed.ddl) != ddlCount {
				t.Errorf("Begin より前に DDL を %d 件適用するはず, got %d", ddlCount, len(parsed.ddl))
			}
			if parsed.begins != 1 || parsed.commits != 0 || parsed.rollback != 1 {
				t.Errorf("Begin=%d Commit=%d Rollback=%d, want 1/0/1", parsed.begins, parsed.commits, parsed.rollback)
			}
			if got, want := len(parsed.txStmts), p.at+1; got != want {
				t.Errorf("Tx の中の文の数 = %d, want %d（失敗の位置より後の文を実行しない）", got, want)
			}
			assertNoLeak(t, err, events)
		})
	}

	t.Run("コミットの失敗は失敗として終える", func(t *testing.T) {
		injected := errors.New("injected commit failure")
		h := startedRTHarness(t)
		h.tx.commitErr = injected
		events, err := h.invoke(t, `{}`)
		if err == nil || !errors.Is(err, injected) {
			t.Fatalf("ハンドラはエラーを返さなければならない: %v", err)
		}
		parsed := parseInvocation(t, events, 2)
		if parsed.commits != 1 {
			t.Errorf("Commit の試み = %d, want 1", parsed.commits)
		}
		if got, want := len(parsed.txStmts), len(base.txStmts); got != want {
			t.Errorf("Tx の中の文の数 = %d, want %d", got, want)
		}
		assertNoLeak(t, err, events)
	})
}

// DB にあるデモの契約の技術者識別子が、環境変数から受け取った値と一致しないとき、失敗してロールバックする
// （AC-6-5）。契約の行を書き換えて合わせない（契約への Exec が1つも無い）。
func TestHandler_EngineerMismatchFailsAndRollsBack(t *testing.T) {
	h := startedRTHarness(t)
	h.tx.engineer = func(string) (string, bool) { return rtOtherEngineer, true }
	events, err := h.invoke(t, `{}`)
	if err == nil {
		t.Fatalf("技術者識別子の不一致でハンドラはエラーを返さなければならない")
	}
	parsed := parseInvocation(t, events, 2)
	if parsed.begins != 1 || parsed.commits != 0 || parsed.rollback != 1 {
		t.Errorf("Begin=%d Commit=%d Rollback=%d, want 1/0/1", parsed.begins, parsed.commits, parsed.rollback)
	}
	for _, e := range parsed.txStmts {
		if e.kind == "tx.exec" {
			t.Errorf("不一致のとき Tx の中で Exec を実行してはならない（契約の行を書き換えて合わせない・以降の文を実行しない）: %v", e)
		}
	}
	if strings.Contains(err.Error(), rtOtherEngineer) {
		t.Errorf("エラーの文言に DB から取得した値が含まれている（AC-11-6）: %q", err.Error())
	}
	assertNoLeak(t, err, events)
}

// assertNoLeak は、返ったエラーの文言に接続文字列・SSM のパラメータ名・SQL 文を含めないこと（AC-11-6）を確かめる。
func assertNoLeak(t *testing.T, err error, events []rtCall) {
	t.Helper()
	msg := err.Error()
	for _, secret := range []string{rtParamName, rtConnString} {
		if strings.Contains(msg, secret) {
			t.Errorf("エラーの文言に %q が含まれている（AC-11-6）: %q", secret, msg)
		}
	}
	for _, e := range events {
		if q := strings.TrimSpace(e.query); q != "" && strings.Contains(msg, q) {
			t.Errorf("エラーの文言に SQL 文が含まれている（AC-11-6）: %q", msg)
		}
	}
}

// ---- 組み立ての失敗（AC-9-3・AC-9-3-a） -------------------------------------------

// 組み立ての関数がエラーを返したとき、ハンドラは失敗として終え（①）、Commit を呼ばず（②）、
// トランザクションを開いていたなら Rollback を1回呼び（③）、契約の書き込み・破棄・集約の保存の文を
// 1つも発行しない（④。DDL は対象外）。組み立てをトランザクションの前に行うか後に行うかは固定しない
// （Begin の回数と Rollback の回数が一致することで③を表す）。
func TestHandler_BuildFailure_FailsWithoutCommitAndWritesNothing(t *testing.T) {
	injected := errors.New("injected build failure")
	h := startedRTHarness(t)
	var gotDate workmonth.Date
	var gotEngineer string
	h.build = func(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error) {
		gotDate, gotEngineer = resetDate, engineerID
		return nil, nil, injected
	}
	events, err := h.invoke(t, `{}`)

	// ① ハンドラの呼び出しがエラーを返す（原因を %w で包む）。
	if err == nil || !errors.Is(err, injected) {
		t.Fatalf("組み立ての失敗でハンドラはエラーを返し、原因を %%w で包まなければならない: %v", err)
	}
	if h.buildCalls != 1 {
		t.Errorf("組み立ての呼び出し回数 = %d, want 1", h.buildCalls)
	}
	// 組み立てへ渡る入力は、時計のリセット日（JST 2026-10-27。AC-11-1）と環境変数のデモの技術者識別子（AC-9-5）。
	if gotDate.Year() != 2026 || gotDate.Month() != 10 || gotDate.Day() != 27 {
		t.Errorf("組み立てへ渡るリセット日 = %d-%02d-%02d, want 2026-10-27", gotDate.Year(), gotDate.Month(), gotDate.Day())
	}
	if gotEngineer != rtEngineer {
		t.Errorf("組み立てへ渡る技術者識別子 = %q, want 環境変数の値", gotEngineer)
	}

	begins, commits, rollbacks := 0, 0, 0
	for _, e := range events {
		switch e.kind {
		case "begin":
			begins++
		case "commit":
			commits++
		case "rollback":
			rollbacks++
		case "tx.exec":
			// ④ 契約の書き込み・破棄・集約の保存はいずれも Tx の中の Exec。
			t.Errorf("組み立ての失敗の後に契約の書き込み・破棄・集約の保存の文を発行してはならない: %v", e)
		}
	}
	// ② Commit を呼ばない。
	if commits != 0 {
		t.Errorf("Commit = %d, want 0", commits)
	}
	// ③ トランザクションを開いていたなら Rollback を1回呼ぶ（開いていなければ呼ばない）。
	if rollbacks != begins {
		t.Errorf("Begin=%d Rollback=%d, want 開いた回数と同じ（開いていたなら1回）", begins, rollbacks)
	}
	if begins > 1 {
		t.Errorf("Begin = %d, want 高々1回", begins)
	}
	assertNoLeak(t, err, events)
}
