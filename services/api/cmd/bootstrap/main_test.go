package main

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-17 ③
// （AC-10-18「起動手順の取り出し」。責務は AC-10-15、置き場所は D-12）。
//
// 差し替えた探索・接続・登録（いずれも手書きで、呼び出しと引数を記録するだけの
// もの）を渡し、次の6つを固定する。
//
//	(i)   正常系: 接続がちょうど1回・登録がちょうど1回呼ばれ、接続が登録より
//	      先であること（記録の順序で確認する）。
//	(ii)  必要な設定を返さない探索では、接続も登録も1回も呼ばれずにエラーが
//	      返ること。
//	(iii) 接続がエラーを返す場合は、登録が1回も呼ばれずにエラーが返ること
//	      （(i) と対にして置く）。
//	(iv)  登録に渡された値へ手で組んだイベントを与えると契約の形の応答が
//	      返ること（＝AC-10-16 と AC-10-17 が結線されている）。
//	(v)   返るエラーの文言に、探索が返した値がそのまま含まれないこと
//	      （AC-10-13 ③・docs/rules/security.md）。
//	(vi)  プロセスの環境変数を書き換えない（os.Setenv / t.Setenv を使わない。
//	      AC-12-16 ① と同じ理由。書き換えないと (i)(ii) が Red にならない）。
//
// テストは環境変数の名前に依存しない（AC-10-12 が名前を固定していない）。
// 差し替えた探索は要求された名前を記録するだけで、名前そのものを期待値に
// しない（AC-12-16 ① と同じ形）。
//
// テストは pgx を直接 import しない（AC-12-6・D-11）。手書きの偽 pgx も
// 作らない（AC-12-16・AC-13-23 ②）。本パッケージが import する
// driver/persistence を通じて pgx がビルドに含まれるが、本テストは接続の確立を
// 差し替えるため pgx を直接扱わない。
//
// 担保しないもの（AC-13-24）: ①接続がコールドスタート時に1度だけ確立され
// 再利用されること（観測するのは1回の起動手順の中での回数と順序まで）、
// ③main() に残った部分（実際の探索・接続・登録を渡していること）、
// ④Lambda ランタイムそのもの、⑤SQL 文・トランザクション・実接続。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/persistence"
)

// dummyLookupValue は差し替えた探索が返すダミー値。**実在しうる接続文字列を
// 書かない**（AC-12-17 ③ (v)・docs/rules/security.md）。
const dummyLookupValue = "dummy-setting-value-not-a-real-connection-string"

// ---- 手書きのテストダブル（モックライブラリを入れない＝ADR 0007） ----------

// startupRecorder は探索・接続・登録の呼び出しを記録する。
type startupRecorder struct {
	lookupNames   []string // 要求された環境変数の名前（名前は期待値にしない）
	order         []string // "connect" / "register" の呼び出し順
	connectCalls  int
	registerCalls int
	registered    lambda.EventHandler
}

// lookupReturning は「必要な設定が揃っている」探索。名前によらず同じダミー値を
// 返す（探索する名前を仕様が固定していないため＝AC-10-12）。
func (r *startupRecorder) lookupReturning(value string) func(name string) (string, bool) {
	return func(name string) (string, bool) {
		r.lookupNames = append(r.lookupNames, name)
		return value, true
	}
}

// lookupNothing は「必要な設定を返さない」探索。
func (r *startupRecorder) lookupNothing() func(name string) (string, bool) {
	return func(name string) (string, bool) {
		r.lookupNames = append(r.lookupNames, name)
		return "", false
	}
}

// connectReturning は接続の確立の差し替え。
func (r *startupRecorder) connectReturning(db gateway.DB, err error) func(context.Context, persistence.Config) (gateway.DB, error) {
	return func(_ context.Context, _ persistence.Config) (gateway.DB, error) {
		r.connectCalls++
		r.order = append(r.order, "connect")
		return db, err
	}
}

// register はランタイムへの登録の差し替え。実際のランタイムは起動しない。
func (r *startupRecorder) register(h lambda.EventHandler) {
	r.registerCalls++
	r.order = append(r.order, "register")
	r.registered = h
}

// bootRows は gateway.Rows の手書き Fake。
type bootRows struct {
	rows [][]any
	idx  int
}

func newBootRows(rows ...[]any) *bootRows { return &bootRows{rows: rows} }

func (r *bootRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *bootRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("bootRows: 現在行が無い状態で Scan が呼ばれた")
	}
	row := r.rows[r.idx-1]
	if len(dest) != len(row) {
		return fmt.Errorf("bootRows: Scan の宛先の数 = %d, want %d", len(dest), len(row))
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			v, ok := row[i].(string)
			if !ok {
				return fmt.Errorf("bootRows: 第%d列が string でない: %#v", i, row[i])
			}
			*target = v
		case *int:
			v, ok := row[i].(int)
			if !ok {
				return fmt.Errorf("bootRows: 第%d列が int でない: %#v", i, row[i])
			}
			*target = v
		case **int:
			v, ok := row[i].(*int)
			if !ok {
				return fmt.Errorf("bootRows: 第%d列が *int でない: %#v", i, row[i])
			}
			*target = v
		default:
			return fmt.Errorf("bootRows: 未対応の Scan 宛先の型 %T", d)
		}
	}
	return nil
}

func (r *bootRows) Close() error { return nil }
func (r *bootRows) Err() error   { return nil }

// bootDB は gateway.DB の手書き Fake。Query の応答を呼び出し順に消費する。
// SQL 文そのものは観測しない（AC-13-18）。
type bootDB struct {
	queue   []gateway.Rows
	queries int
}

func (db *bootDB) pushQuery(rows gateway.Rows) { db.queue = append(db.queue, rows) }

func (db *bootDB) Query(_ context.Context, _ string, _ ...any) (gateway.Rows, error) {
	db.queries++
	if len(db.queue) == 0 {
		return nil, errors.New("bootDB: Query の応答が設定されていない")
	}
	rows := db.queue[0]
	db.queue = db.queue[1:]
	return rows, nil
}

func (db *bootDB) Exec(_ context.Context, _ string, _ ...any) error {
	return errors.New("bootDB: Exec は本テストで使わない")
}

func (db *bootDB) Begin(_ context.Context) (gateway.Tx, error) {
	return nil, errors.New("bootDB: Begin は本テストで使わない")
}

var _ gateway.DB = (*bootDB)(nil)

// ---- (i) 正常系 -------------------------------------------------------------

// TestRun_ConnectsOnceAndRegistersOnceInOrder は AC-12-17 ③ (i)。
func TestRun_ConnectsOnceAndRegistersOnceInOrder(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err != nil {
		t.Fatalf("Run がエラーを返した: %v", err)
	}

	if rec.connectCalls != 1 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 1（ちょうど1回＝AC-10-18）", rec.connectCalls)
	}
	if rec.registerCalls != 1 {
		t.Errorf("登録の呼び出し回数 = %d, want 1（ちょうど1回＝AC-10-18）", rec.registerCalls)
	}
	if diff := cmp.Diff([]string{"connect", "register"}, rec.order); diff != "" {
		t.Errorf("呼び出し順が不一致 (-want +got):\n%s（接続が登録より先＝AC-10-18）", diff)
	}
	if rec.registered == nil {
		t.Errorf("登録に値が渡っていない（AC-10-17 が返した形を登録する＝AC-10-18）")
	}
}

// ---- (ii) 必要な設定を返さない探索 -------------------------------------------

// TestRun_MissingSetting_SkipsConnectAndRegister は AC-12-17 ③ (ii)。
func TestRun_MissingSetting_SkipsConnectAndRegister(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.lookupNothing(),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった（必要な設定が未設定なら既定値へ黙って落ちない＝AC-10-12・AC-10-18）")
	}

	if rec.connectCalls != 0 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 0（設定の組み立てに失敗したら接続を試みない）", rec.connectCalls)
	}
	if rec.registerCalls != 0 {
		t.Errorf("登録の呼び出し回数 = %d, want 0（要求を受け付けてから失敗させない＝AC-10-18）", rec.registerCalls)
	}
	if len(rec.lookupNames) == 0 {
		t.Errorf("差し替えた探索が1度も呼ばれていない（プロセスの環境変数を暗黙に読んでいる疑い＝AC-10-12・AC-10-18 (i)）")
	}
}

// ---- (iii)(v) 接続がエラーを返す場合 -----------------------------------------

// errConnectFailed は差し替えた接続が返す番兵（テスト専用）。
var errConnectFailed = errors.New("bootDB: 接続の確立に失敗した（テスト用の番兵）")

// TestRun_ConnectFails_SkipsRegister は AC-12-17 ③ (iii)(v)。
func TestRun_ConnectFails_SkipsRegister(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(nil, errConnectFailed),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった（接続の確立に失敗したら登録せずエラー＝AC-10-18）")
	}
	if !errors.Is(err, errConnectFailed) {
		t.Errorf("errors.Is で接続のエラーへ辿れない: %v（ラップするなら %%w＝AC-11-9・AC-10-13 ②）", err)
	}

	if rec.connectCalls != 1 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 1", rec.connectCalls)
	}
	if rec.registerCalls != 0 {
		t.Errorf("登録の呼び出し回数 = %d, want 0（接続に失敗したら登録しない＝AC-10-18）", rec.registerCalls)
	}

	// (v) 返るエラーの文言に、探索が返した値がそのまま含まれない。
	if strings.Contains(err.Error(), dummyLookupValue) {
		t.Errorf("エラーの文言に探索が返した値が含まれている（AC-10-13 ③・docs/rules/security.md）: %v", err)
	}
}

// TestRun_MissingSetting_ErrorDoesNotLeakLookupValue は (v) を、探索が値を返した
// うえで設定が揃わない経路でも固定する。空文字を返す探索（ok=true）は
// 「未設定または空」に当たる（AC-10-12）。
func TestRun_MissingSetting_ErrorDoesNotLeakLookupValue(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.lookupReturning(""),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった（空の設定は未設定と同じく扱う＝AC-10-12）")
	}
	if rec.connectCalls != 0 || rec.registerCalls != 0 {
		t.Errorf("接続 %d 回・登録 %d 回, want 0 回・0 回", rec.connectCalls, rec.registerCalls)
	}
	if strings.Contains(err.Error(), dummyLookupValue) {
		t.Errorf("エラーの文言に探索が返した値が含まれている（AC-10-13 ③）: %v", err)
	}
}

// ---- (iv) 登録に渡された値へイベントを与える ---------------------------------

// TestRun_RegisteredHandlerAnswersContractShape は AC-12-17 ③ (iv)。
// 登録に渡された値へ手で組んだイベント（AC-12-17 ② と同じ形）を与えると、
// 契約の形の応答が返る（＝AC-10-16 と AC-10-17 が結線されている）。
// Fake が行を返すよう仕込んだ E-1 を置く（AC-12-15 ④ (iii) と同じ形）。
//
// 期待値は契約（E-1・AC-10-1）から導いており、実装を走らせて得た値を
// 書き写していない（AC-13-19 ④）。
func TestRun_RegisteredHandlerAnswersContractShape(t *testing.T) {
	rec := &startupRecorder{}

	db := &bootDB{}
	// 契約1件（識別子・契約表示名・技術者識別子・精算幅下限（時・分）・
	// 精算幅上限（時・分）の7列）。
	db.pushQuery(newBootRows([]any{"ctr-boot", "契約ブート", "eng-boot", 140, 0, 180, 0}))
	// 勤務月ヘッダ1件（12列。超過／不足は NULL＝未確定）。
	var null *int
	db.pushQuery(newBootRows([]any{"ctr-boot", 2026, 7, 140, 0, 180, 0, "Draft", null, null, null, null}))
	// 稼働実績1件（年・月・日・稼働時間（時・分）の5列）。
	db.pushQuery(newBootRows([]any{2026, 7, 3, 8, 0}))

	if err := Run(rec.lookupReturning(dummyLookupValue), rec.connectReturning(db, nil), rec.register); err != nil {
		t.Fatalf("Run がエラーを返した: %v", err)
	}
	if rec.registered == nil {
		t.Fatalf("登録に値が渡っていない")
	}

	event := events.LambdaFunctionURLRequest{
		RawPath: "/work-months/ctr-boot/2026-07",
		RequestContext: events.LambdaFunctionURLRequestContext{
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method: http.MethodGet,
			},
		},
	}

	resp, err := rec.registered(context.Background(), event)
	if err != nil {
		t.Fatalf("登録された値がランタイムへエラーを返した: %v（AC-10-17）", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200（body=%s）", resp.StatusCode, resp.Body)
	}
	if resp.IsBase64Encoded {
		t.Errorf("IsBase64Encoded = true, want false（AC-10-17 ③）")
	}

	const wantBody = `{
		"contractId": "ctr-boot",
		"contractDisplayName": "契約ブート",
		"yearMonth": "2026-07",
		"state": "Draft",
		"generated": true,
		"settlementRange": {
			"lowerBound": {"hours": 140, "minutes": 0},
			"upperBound": {"hours": 180, "minutes": 0}
		},
		"totalHours": {"hours": 8, "minutes": 0},
		"excess": null,
		"shortfall": null,
		"dailyRecords": [
			{
				"date": "2026-07-03",
				"workingHours": {"hours": 8, "minutes": 0},
				"roundedWorkingHours": {"hours": 8, "minutes": 0}
			}
		]
	}`

	var got, want any
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatalf("応答ボディの JSON デコードに失敗した: %v（body=%s）", err, resp.Body)
	}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("期待ボディの JSON デコードに失敗した（テスト側の誤り）: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("応答ボディが契約 AC-10-1 の期待と不一致 (-want +got):\n%s", diff)
	}

	// 起動手順で確立した接続がハンドラへ渡っている（要求ごとに確立し直して
	// いない）ことの最小限の観測。**1度だけ確立され再利用されることそのものは
	// 担保しない**（AC-13-24 ①）。
	if db.queries == 0 {
		t.Errorf("SQL 実行 Fake が1度も使われていない（接続がハンドラへ結線されていない）")
	}
	if rec.connectCalls != 1 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 1", rec.connectCalls)
	}
}
