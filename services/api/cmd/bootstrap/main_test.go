package main

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-17 ③
// （AC-10-18「起動手順の取り出し」。責務は AC-10-15、置き場所は D-12。
// **2026-08-26 追記＝`docs/specs/infra-terraform.md` AC-8-8**: Run が受け取る
// 引数は AC-10-18 の定める4つ（(0) 接続文字列の解決・(i) 環境変数の探索・
// (ii) 接続の確立・(iii) ランタイムへの登録）へ拡げる）。
//
// **2026-08-27 是正（人間承認）**:
//   - (0) の署名は AC-10-18 が明記するとおり「`infra-terraform.md` AC-8-11 が
//     定める SecretFetcher 越しの取得と同じ形」。AC-8-11 ② のメソッドは
//     「パラメータ名を受け取り、復号済みの値を返し、失敗を返すもの」であり、
//     既存の `ResolveConnectionString(ctx, fetcher, parameterName)` /
//     `SecretFetcher.FetchSecret(ctx, name)` と同じ形に揃える。したがって
//     (0) の型は `func(ctx context.Context, parameterName string) (string, error)`
//     とする（`fetcher` は main() 側で束縛済みという想定）。
//   - パラメータ名（AC-8-1「Lambda の環境変数が持つのは SSM パラメータの
//     名前」）は、Run の内側で (i) 環境変数の探索（`lookup`）を使って得る。
//     これにより `lookup` の死に引数を解消し、環境変数の探索が本テストから
//     観測できる。
//
// 差し替えた解決・探索・接続・登録（いずれも手書きで、呼び出しと引数を
// 記録するだけのもの）を渡し、次を固定する。
//
//	(1) 正常系: 探索がパラメータ名を返し、それがそのまま解決へ渡り、解決が
//	    ちょうど1回・接続がちょうど1回・登録がちょうど1回呼ばれ、解決 →
//	    接続 → 登録の順であること（記録の順序で確認する。AC-10-18「順序と
//	    回数を固定する」）。
//	(2) 接続文字列の解決に失敗したら、接続も登録も1回も呼ばれずにエラーが
//	    返ること（AC-10-18・infra-terraform AC-8-5「取得または復号に失敗
//	    したら、ランタイムへハンドラを登録せずにエラーで終える」）。
//	(3) 解決が失敗した経路で、返るエラーの文言に、解決しようとした値が
//	    そのまま含まれないこと（AC-10-13 ③・docs/rules/security.md）。
//	(4) 解決が成功しても、その値が空なら（設定の組み立てが「必要な設定が
//	    未設定または空」と判定し）接続も登録も行われずエラーになること
//	    （AC-10-12 は変わらず適用される）。
//	(5) 接続がエラーを返す場合は、登録が1回も呼ばれずにエラーが返り、
//	    その文言に解決した値がそのまま含まれないこと（(1) と対にして
//	    置く。対にしないと「常にエラーを返す」実装が Green になる）。
//	(6) 登録に渡された値へ手で組んだイベントを与えると契約の形の応答が
//	    返ること（＝AC-10-16 と AC-10-17 が結線されている）。
//	(7) プロセスの環境変数を書き換えない（os.Setenv / t.Setenv を使わない。
//	    AC-12-16 ① と同じ理由。書き換えないと (1)(2) が Red にならない）。
//	(8) パラメータ名の環境変数が未設定・空のときは、解決・接続・登録の
//	    いずれも呼ばれずにエラーで終えること（AC-8-5・既定値へ黙って
//	    落ちない。2026-08-27 是正で追加）。
//	(9) 探索で得た値（＝SSM パラメータ名）が、返るエラーの文言に含まれ
//	    ないこと（infra-terraform.md AC-8-9。AC-12-16 ①(iii) が
//	    driver/persistence 側に置いていた同種の検査の移設先。2026-08-27
//	    是正で追加）。
//
// テストは環境変数の名前に依存しない（AC-10-12 が名前を固定していない）。
// 差し替えた探索は要求された名前を記録するだけで、名前そのものを期待値に
// しない（AC-12-16 ① と同じ形）。**探索が返す値（＝パラメータ名）は本テスト
// が制御するダミー値であり、これを期待値にすることは「環境変数の名前を
// 期待値にする」ことと同義ではない**（探索のキーではなく探索の戻り値）。
//
// テストは pgx を直接 import しない（AC-12-6・D-11）。手書きの偽 pgx も
// 作らない（AC-12-16・AC-13-23 ②）。本パッケージが import する
// driver/persistence を通じて pgx がビルドに含まれるが、本テストは接続の確立を
// 差し替えるため pgx を直接扱わない。
//
// 担保しないもの（AC-13-24）: ①接続がコールドスタート時に1度だけ確立され
// 再利用されること（観測するのは1回の起動手順の中での回数と順序まで）、
// ③main() に残った部分（実際の解決・探索・接続・登録を渡していること）、
// ④Lambda ランタイムそのもの、⑤SQL 文・トランザクション・実接続、
// ⑥接続文字列の解決の中身（SSM の呼び出し。要求本文は
// docs/specs/infra-terraform.md AC-8 が持ち、テストは secret_resolver_test.go
// に既にある）。

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

// dummyLookupValue は差し替えた探索（AC-10-18 (i)）が返すダミー値。Run の
// 内側でパラメータ名（AC-8-1）として解決（(0)）へそのまま渡る想定の値。
// **実在しうる接続文字列やパラメータ名を書かない**（AC-12-17 ③ (v)・
// docs/rules/security.md）。
const dummyLookupValue = "dummy-lookup-value-not-a-real-parameter-name"

// dummyResolvedConnectionString は差し替えた解決（AC-10-18 (0)）が返す
// ダミー値。**実在しうる接続文字列を書かない**（同上）。
const dummyResolvedConnectionString = "dummy-resolved-value-not-a-real-connection-string"

// ---- 手書きのテストダブル（モックライブラリを入れない＝ADR 0007） ----------

// startupRecorder は解決・探索・接続・登録の呼び出しを記録する。
type startupRecorder struct {
	lookupNames       []string // 要求された環境変数の名前（名前は期待値にしない）
	order             []string // "resolve" / "connect" / "register" の呼び出し順
	resolveCalls      int
	resolveParamNames []string // resolve へ渡ったパラメータ名（lookup が返した値がそのまま渡ることの検査に使う）
	connectCalls      int
	registerCalls     int
	registered        lambda.EventHandler
}

// resolveReturning は「接続文字列の解決」の差し替え（AC-10-18 (0)。
// infra-terraform AC-8-11 ②の形＝パラメータ名を受け取り、復号済みの値を
// 返す）。成功し、渡した値を返す。
func (r *startupRecorder) resolveReturning(value string) func(context.Context, string) (string, error) {
	return func(_ context.Context, parameterName string) (string, error) {
		r.resolveCalls++
		r.order = append(r.order, "resolve")
		r.resolveParamNames = append(r.resolveParamNames, parameterName)
		return value, nil
	}
}

// resolveFailing は「接続文字列の解決」が失敗する差し替え。
func (r *startupRecorder) resolveFailing(err error) func(context.Context, string) (string, error) {
	return func(_ context.Context, parameterName string) (string, error) {
		r.resolveCalls++
		r.order = append(r.order, "resolve")
		r.resolveParamNames = append(r.resolveParamNames, parameterName)
		return "", err
	}
}

// resolveFailingButReturningValue は「値は返すが、失敗と答える」解決。
// エラーの文言が解決しようとした値を漏らさないこと（AC-10-13 ③）を、
// 失敗する経路でも実際に観測できる形にするために要る（config_test.go の
// `lookupUnsetButReturning` と同じ理由。値を一度も返させない形では、
// 値が漏れる実装があっても検査が空回りする）。
func (r *startupRecorder) resolveFailingButReturningValue(value string, err error) func(context.Context, string) (string, error) {
	return func(_ context.Context, parameterName string) (string, error) {
		r.resolveCalls++
		r.order = append(r.order, "resolve")
		r.resolveParamNames = append(r.resolveParamNames, parameterName)
		return value, err
	}
}

// lookupReturning は AC-10-18 (i) の差し替え。名前によらず同じダミー値を
// 返す（探索する名前を仕様が固定していないため＝AC-10-12）。この値は
// Run の内側でパラメータ名（AC-8-1）として (0) の解決へそのまま渡る想定
// （2026-08-27 是正）。
func (r *startupRecorder) lookupReturning(value string) func(name string) (string, bool) {
	return func(name string) (string, bool) {
		r.lookupNames = append(r.lookupNames, name)
		return value, true
	}
}

// lookupNotFound は「未設定」を表す探索の差し替え（AC-8-5 相当。名前に
// よらず見つからない）。
func lookupNotFound(string) (string, bool) { return "", false }

// lookupFoundButEmpty は「設定はされているが値が空」を表す探索の差し替え。
func lookupFoundButEmpty(string) (string, bool) { return "", true }

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

// ---- (1) 正常系 --------------------------------------------------------------

// TestRun_ResolvesOnceBeforeConnectAndRegistersOnceInOrder は AC-12-17 ③ の
// 正常系。**解決がちょうど1回**であること（AC-10-15 ⓪①・AC-10-18「順序と
// 回数を固定する」）。あわせて、**探索が返した値（パラメータ名）がそのまま
// 解決へ渡ること**（AC-8-1・AC-10-18 (i) が (0) の引数を作る＝2026-08-27
// 是正）を固定する。
func TestRun_ResolvesOnceBeforeConnectAndRegistersOnceInOrder(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.resolveReturning(dummyResolvedConnectionString),
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err != nil {
		t.Fatalf("Run がエラーを返した: %v", err)
	}

	if rec.resolveCalls != 1 {
		t.Errorf("接続文字列の解決の呼び出し回数 = %d, want 1（ちょうど1回＝AC-10-18）", rec.resolveCalls)
	}
	if rec.connectCalls != 1 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 1（ちょうど1回＝AC-10-18）", rec.connectCalls)
	}
	if rec.registerCalls != 1 {
		t.Errorf("登録の呼び出し回数 = %d, want 1（ちょうど1回＝AC-10-18）", rec.registerCalls)
	}
	if diff := cmp.Diff([]string{"resolve", "connect", "register"}, rec.order); diff != "" {
		t.Errorf("呼び出し順が不一致 (-want +got):\n%s（解決 → 接続 → 登録の順＝AC-10-18）", diff)
	}
	if diff := cmp.Diff([]string{dummyLookupValue}, rec.resolveParamNames); diff != "" {
		t.Errorf("解決へ渡ったパラメータ名が不一致 (-want +got):\n%s（探索が返した値がそのまま渡ること＝AC-8-1・AC-10-18 (i)）", diff)
	}
	if rec.registered == nil {
		t.Errorf("登録に値が渡っていない（AC-10-17 が返した形を登録する＝AC-10-18）")
	}
}

// ---- (2)(3) 接続文字列の解決に失敗する場合 ------------------------------------

// errResolveFailed は差し替えた解決が返す番兵（テスト専用）。
var errResolveFailed = errors.New("secret_resolver: 接続文字列の解決に失敗した（テスト用の番兵）")

// TestRun_ResolveFails_SkipsConnectAndRegister は AC-12-17 ③ (2)。
// infra-terraform AC-8-5「取得または復号に失敗したら、ランタイムへハンドラを
// 登録せずにエラーで終える」を固定する。
func TestRun_ResolveFails_SkipsConnectAndRegister(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.resolveFailing(errResolveFailed),
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった（接続文字列の解決に失敗したら既定値へ黙って落ちない＝infra-terraform AC-8-5）")
	}
	if !errors.Is(err, errResolveFailed) {
		t.Errorf("errors.Is で解決のエラーへ辿れない: %v（ラップするなら %%w）", err)
	}

	if rec.resolveCalls != 1 {
		t.Errorf("接続文字列の解決の呼び出し回数 = %d, want 1", rec.resolveCalls)
	}
	if rec.connectCalls != 0 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 0（解決に失敗したら接続を試みない＝infra-terraform AC-8-5）", rec.connectCalls)
	}
	if rec.registerCalls != 0 {
		t.Errorf("登録の呼び出し回数 = %d, want 0（要求を受け付けてから失敗させない＝infra-terraform AC-8-5）", rec.registerCalls)
	}
}

// TestRun_ResolveFails_ErrorDoesNotLeakResolvedValue は AC-12-17 ③ (3)。
// 返るエラーの文言に、解決しようとした値がそのまま含まれないこと
// （AC-10-13 ③・docs/rules/security.md）。
//
// **解決には値を返させたうえで失敗と答えさせる**（config_test.go の
// `lookupUnsetButReturning` と同じ理由。値を一度も返させない形では、
// 値が漏れる実装があっても検査が空回りする）。
func TestRun_ResolveFails_ErrorDoesNotLeakResolvedValue(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.resolveFailingButReturningValue(dummyResolvedConnectionString, errResolveFailed),
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった（解決に失敗したら既定値へ黙って落ちない）")
	}
	if rec.resolveCalls != 1 {
		t.Errorf("接続文字列の解決の呼び出し回数 = %d, want 1（この経路を実際に通ったことの確認。呼ばれずに常にエラーを返すだけの実装を排除する）", rec.resolveCalls)
	}
	if rec.connectCalls != 0 || rec.registerCalls != 0 {
		t.Errorf("接続 %d 回・登録 %d 回, want 0 回・0 回", rec.connectCalls, rec.registerCalls)
	}
	if strings.Contains(err.Error(), dummyResolvedConnectionString) {
		t.Errorf("エラーの文言に解決しようとした値が含まれている（AC-10-13 ③・docs/rules/security.md）: %v", err)
	}
}

// TestRun_ResolveFails_ErrorDoesNotLeakLookedUpParameterName は AC-12-17 ③
// (9)（2026-08-27 是正・人間承認で追加）。返るエラーの文言に、**探索で
// 得た値（＝SSM パラメータ名）**がそのまま含まれないこと（
// infra-terraform.md AC-8-9・docs/rules/security.md）。
//
// AC-10-13 ③ が固定するのは「解決した値（接続文字列）」の非漏洩であり、
// 本テストが固定するのはそれとは別の値（＝探索で得てパラメータ名として
// 解決へ渡した値）の非漏洩である。対にしないと、探索で得た値をそのまま
// エラー文言へ埋め込む実装が Green のまま残る。
func TestRun_ResolveFails_ErrorDoesNotLeakLookedUpParameterName(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.resolveFailing(errResolveFailed),
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった")
	}
	if rec.resolveCalls != 1 {
		t.Errorf("接続文字列の解決の呼び出し回数 = %d, want 1（この経路を実際に通ったことの確認。呼ばれずに常にエラーを返すだけの実装を排除する）", rec.resolveCalls)
	}
	if strings.Contains(err.Error(), dummyLookupValue) {
		t.Errorf("エラーの文言に、探索で得た値（SSM パラメータ名）が含まれている（infra-terraform.md AC-8-9）: %v", err)
	}
}

// ---- (4) 解決は成功したが値が空の場合 -----------------------------------------

// TestRun_ResolvedValueEmpty_SkipsConnectAndRegister は、解決自体は成功して
// も値が空なら、設定の組み立て（AC-10-12）が「必要な設定が未設定または空」
// と判定し、接続・登録を行わずエラーになることを固定する。
func TestRun_ResolvedValueEmpty_SkipsConnectAndRegister(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.resolveReturning(""),
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(&bootDB{}, nil),
		rec.register,
	)
	if err == nil {
		t.Fatalf("Run がエラーを返さなかった（解決した値が空なら既定値へ黙って落ちない＝AC-10-12）")
	}
	if rec.connectCalls != 0 || rec.registerCalls != 0 {
		t.Errorf("接続 %d 回・登録 %d 回, want 0 回・0 回", rec.connectCalls, rec.registerCalls)
	}
}

// ---- (5) 接続がエラーを返す場合 -----------------------------------------------

// errConnectFailed は差し替えた接続が返す番兵（テスト専用）。
var errConnectFailed = errors.New("bootDB: 接続の確立に失敗した（テスト用の番兵）")

// TestRun_ConnectFails_SkipsRegister は AC-12-17 ③ (5)。
func TestRun_ConnectFails_SkipsRegister(t *testing.T) {
	rec := &startupRecorder{}

	err := Run(
		rec.resolveReturning(dummyResolvedConnectionString),
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

	if rec.resolveCalls != 1 {
		t.Errorf("接続文字列の解決の呼び出し回数 = %d, want 1", rec.resolveCalls)
	}
	if rec.connectCalls != 1 {
		t.Errorf("接続の確立の呼び出し回数 = %d, want 1", rec.connectCalls)
	}
	if rec.registerCalls != 0 {
		t.Errorf("登録の呼び出し回数 = %d, want 0（接続に失敗したら登録しない＝AC-10-18）", rec.registerCalls)
	}

	// 返るエラーの文言に、解決した接続文字列がそのまま含まれない。
	if strings.Contains(err.Error(), dummyResolvedConnectionString) {
		t.Errorf("エラーの文言に解決した接続文字列が含まれている（AC-10-13 ③・docs/rules/security.md）: %v", err)
	}
}

// ---- (8) パラメータ名の環境変数が未設定・空の場合 -----------------------------

// TestRun_ParameterNameLookupUnsetOrEmpty_SkipsResolveConnectAndRegister は
// AC-12-17 ③ (8)（2026-08-27 是正・人間承認で追加）。パラメータ名の環境
// 変数が未設定・空のときは、解決・接続・登録のいずれも呼ばれずにエラーで
// 終えること（AC-8-1・AC-8-5 と同じ思想。既定値へ黙って落ちない）。
func TestRun_ParameterNameLookupUnsetOrEmpty_SkipsResolveConnectAndRegister(t *testing.T) {
	tests := []struct {
		name   string
		lookup func(name string) (string, bool)
	}{
		{
			name:   "未設定（見つからない）",
			lookup: lookupNotFound,
		},
		{
			name:   "設定されているが値が空",
			lookup: lookupFoundButEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &startupRecorder{}

			err := Run(
				rec.resolveReturning(dummyResolvedConnectionString),
				tt.lookup,
				rec.connectReturning(&bootDB{}, nil),
				rec.register,
			)
			if err == nil {
				t.Fatalf("Run がエラーを返さなかった（パラメータ名が未設定・空なら既定値へ黙って落ちない）")
			}
			if rec.resolveCalls != 0 {
				t.Errorf("接続文字列の解決の呼び出し回数 = %d, want 0（パラメータ名が無ければ解決を試みない）", rec.resolveCalls)
			}
			if rec.connectCalls != 0 {
				t.Errorf("接続の確立の呼び出し回数 = %d, want 0", rec.connectCalls)
			}
			if rec.registerCalls != 0 {
				t.Errorf("登録の呼び出し回数 = %d, want 0", rec.registerCalls)
			}
		})
	}
}

// ---- (6) 登録に渡された値へイベントを与える -----------------------------------

// TestRun_RegisteredHandlerAnswersContractShape は AC-12-17 ③ (6)。
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

	if err := Run(
		rec.resolveReturning(dummyResolvedConnectionString),
		rec.lookupReturning(dummyLookupValue),
		rec.connectReturning(db, nil),
		rec.register,
	); err != nil {
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
