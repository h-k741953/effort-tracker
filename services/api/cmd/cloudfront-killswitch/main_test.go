package main

// 検証対象: docs/specs/infra-terraform.md AC-9-13-1（ランタイムへのハンドラ登録
// ＝起動手順の取り出し）の ①〜⑥。形は同 AC が「同型」と指す
// docs/specs/workmonth-implementation-design.md AC-10-15 / AC-10-18
// （cmd/bootstrap の Run。要求本文はそちらが持ち、ここへ書き写さない＝ADR 0004）。
//
// 本テストが固定するのは AC-9-13-1 の次の6つだけである。
//
//	① 起動の手続きが main() の外へ取り出されていること（＝本テストから
//	   呼べること）。main() に残るのは、取り出した手続きを実際のランタイムへの
//	   登録に結びつけて1回だけ呼び、失敗したら異常終了することだけ
//	   （**main() に残る結線は本テストが観測しない**＝12-34）。
//	② ランタイムへの登録を引数として受け取ること（実際の Lambda ランタイムを
//	   起動せずに手続きを呼べ、登録された対象＝ハンドラを受け取って直接
//	   呼べる）。CloudFront 呼び出しの受け口（DistributionDisabler＝AC-9-12）は
//	   手書きのインメモリ Fake へ差し替えられる形のまま保つ（実 AWS を呼ばない）。
//	③ 順序と回数: 遮断対象の解決と AWS 設定の解決を済ませてからハンドラを
//	   組み立て、登録はちょうど1回。同じ手続きの中で2回以上登録しない。
//	   **2つの解決どうしの前後は仕様が固定していないため、本テストも固定しない**
//	   （両方が登録より前であることまでを見る）。
//	④ 起動時の解決（遮断対象・AWS 設定）に失敗したら、登録を行わずにエラーで
//	   終える（AC-8-5 と同型。既定値へ黙って落ちず、対象を推測もしない）。
//	⑤ 登録するハンドラの振る舞い: SNS イベントを受け取ったとき、AC-9-12 の
//	   受け口越しに無効化が呼ばれ、そこへ渡る対象が**起動時に解決した遮断対象**と
//	   一致すること。無効化が失敗したときは成功として扱わず、失敗として返すこと。
//	   再試行・通知・復旧は持たせない（非スコープ）。
//	⑥ 本文を見ないことがハンドラの形として観測できること: 本文の異なる複数の
//	   SNS イベント（本文が空のものを含む）に対して、同じ遮断対象への無効化が
//	   同じように呼ばれる（11-33 が既に持つ禁止を観測できる形へ写したもの
//	   であり、新しい制約を足すものではない）。
//
// **tester が決めた形（implementer が合わせる対象）**: AC-9-13-1 ② は
// 「手段を『引数で受け取る』形に固定するのはここまでであり、**名前・署名・
// 分割の粒度は固定しない**」と明記している。したがって次の名前と署名は、
// **このテストが暫定的に固定するインターフェース**である（既存の
// disable_distribution_test.go / distribution_id_env_test.go のヘッダが
// 「テストが暫定的に固定する」と述べているのと同じ扱い。実装側は同じ形で
// 作ってよいし、都合が悪ければテストごと見直す）。
//
//	type SNSEventHandler func(ctx context.Context, event events.SNSEvent) error
//
//	func Run(
//	    resolveDistributionID func() (string, error),                          // 遮断対象の解決（AC-9-12。方式の持ち主は AC-5-4）
//	    newDisabler func(ctx context.Context) (DistributionDisabler, error),   // AWS 設定の解決（AC-9-15 ①）と受け口の組み立て
//	    register func(h SNSEventHandler),                                      // ランタイムへの登録（AC-9-13-1 ②）
//	) error
//
// 引数で受け取る形にするのは、**プロセスの環境変数を書き換えず**、**実際の
// AWS 設定を解決せず**、**実際の Lambda ランタイムを起動せず**に、①〜⑥ を
// 観測するためである（AC-13-24 ③ と同型に、main() 側の結線だけが観測外に残る
// ＝12-34）。
//
// 依存: 標準 testing のみ（go-cmp は本テストが比較する対象＝スカラと呼び出し
// 記録に対しては不要）。SNS イベントの型は**既に direct require にある**
// aws-lambda-go の events を使う（AC-9-14。**本テストのために direct require を
// 増やさない**＝AC-9-15・11-16）。**アサーションライブラリ・モックライブラリを
// 入れない**（ADR 0007・11-15）。テストダブルは手書きのインメモリ Fake
// （fakeDistributionDisabler は disable_distribution_test.go の既存のものを
// そのまま使う）。
//
// import しないもの: internal/domain・internal/usecase・internal/adapter
// （AC-9-10・11-17）／AWS SDK（受け口越しに差し替えるため不要＝AC-9-12）。
//
// 担保しないもの:
//   - main() に残る結線（実際の解決・実際のランタイムへの登録を渡していること）
//     ＝12-34。
//   - 実際に Lambda ランタイムへ登録されること・登録されたハンドラが実際の
//     SNS イベントで呼ばれること＝12-34。
//   - 遮断 Lambda が実際にディストリビューションを無効化できること＝12-15。
//   - Terraform が注入する環境変数の名と Go が読む名が一致していること＝12-28。

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// ---- 手書きのテストダブル（モックライブラリを入れない＝ADR 0007） ----------

// killswitchStartupRecorder は「遮断対象の解決」「AWS 設定の解決」「ランタイムへの
// 登録」の呼び出しと引数を記録するだけのもの。実際の環境変数・AWS・Lambda
// ランタイムのいずれにも触れない。
type killswitchStartupRecorder struct {
	order            []string // "resolve" / "new-disabler" / "register" の呼び出し順
	resolveCalls     int
	newDisablerCalls int
	registerCalls    int
	registered       SNSEventHandler
}

// resolveReturning は遮断対象の解決（AC-9-13-1 ③）の差し替え。成功し、渡した
// 値を返す。**環境変数の名前は本テストの期待値にしない**（名前の持ち主は
// 構成側＝AC-5-4。読む側の検査は distribution_id_env_test.go が持つ）。
func (r *killswitchStartupRecorder) resolveReturning(distributionID string) func() (string, error) {
	return func() (string, error) {
		r.resolveCalls++
		r.order = append(r.order, "resolve")
		return distributionID, nil
	}
}

// resolveFailing は遮断対象の解決が失敗する差し替え（AC-9-13-1 ④）。
func (r *killswitchStartupRecorder) resolveFailing(err error) func() (string, error) {
	return func() (string, error) {
		r.resolveCalls++
		r.order = append(r.order, "resolve")
		return "", err
	}
}

// disablerReturning は AWS 設定の解決と受け口の組み立て（AC-9-13-1 ③・
// AC-9-15 ①）の差し替え。手書きのインメモリ Fake をそのまま返す
// （実際の AWS 設定を解決しない＝AC-9-12「実 AWS を呼ばない」）。
func (r *killswitchStartupRecorder) disablerReturning(d DistributionDisabler, err error) func(context.Context) (DistributionDisabler, error) {
	return func(context.Context) (DistributionDisabler, error) {
		r.newDisablerCalls++
		r.order = append(r.order, "new-disabler")
		return d, err
	}
}

// register はランタイムへの登録の差し替え（AC-9-13-1 ②）。実際のランタイムは
// 起動せず、登録された対象を保持して、テストから直接呼べるようにする。
func (r *killswitchStartupRecorder) register(h SNSEventHandler) {
	r.registerCalls++
	r.order = append(r.order, "register")
	r.registered = h
}

// indexOf は記録の中で name が最初に現れた位置を返す（見つからなければ -1）。
func (r *killswitchStartupRecorder) indexOf(name string) int {
	for i, v := range r.order {
		if v == name {
			return i
		}
	}
	return -1
}

// errResolveDistributionIDFailed / errAWSConfigResolutionFailed はテスト専用の番兵。
var errResolveDistributionIDFailed = errors.New("cloudfront-killswitch: 遮断対象の解決に失敗した（テスト用の番兵）")
var errAWSConfigResolutionFailed = errors.New("cloudfront-killswitch: AWS 設定の解決に失敗した（テスト用の番兵）")

// otherDistributionIDInMessageBody は SNS メッセージの本文にだけ現れるダミー値
// （⑥）。**本文をパースして対象を決める実装なら、この値が無効化へ渡って Red に
// なる**（11-33）。実在しうる識別子を書かない（AC-2-5・docs/rules/security.md）。
const otherDistributionIDInMessageBody = "E2DUMMYNOTTHETARGET"

// snsEventWithMessage は本文だけが異なる SNS イベントを組む補助。
func snsEventWithMessage(message string) events.SNSEvent {
	return events.SNSEvent{
		Records: []events.SNSEventRecord{
			{SNS: events.SNSEntity{Message: message}},
		},
	}
}

// ---- ①②③: 解決を済ませてから、ちょうど1回だけ登録する --------------------

// TestRun_ResolvesTargetAndAWSConfigThenRegistersExactlyOnce は AC-9-13-1
// ①②③ を固定する。取り出した手続きが本テストから呼べること（①）、登録を
// 引数で受け取ること（②）、2つの解決がいずれもちょうど1回・登録より前に
// 行われ、登録がちょうど1回であること（③）。
func TestRun_ResolvesTargetAndAWSConfigThenRegistersExactlyOnce(t *testing.T) {
	rec := &killswitchStartupRecorder{}
	fake := &fakeDistributionDisabler{
		currentConfig: DistributionConfig{ETag: "etag-run-1", Enabled: true},
	}

	err := Run(
		rec.resolveReturning(dummyDistributionID),
		rec.disablerReturning(fake, nil),
		rec.register,
	)
	if err != nil {
		t.Fatalf("Run がエラーを返した: %v", err)
	}

	if rec.resolveCalls != 1 {
		t.Errorf("遮断対象の解決の呼び出し回数 = %d, want 1（ちょうど1回＝AC-9-13-1 ③）", rec.resolveCalls)
	}
	if rec.newDisablerCalls != 1 {
		t.Errorf("AWS 設定の解決の呼び出し回数 = %d, want 1（ちょうど1回＝AC-9-13-1 ③）", rec.newDisablerCalls)
	}
	if rec.registerCalls != 1 {
		t.Errorf("登録の呼び出し回数 = %d, want 1（同じ手続きの中で2回以上登録しない＝AC-9-13-1 ③）", rec.registerCalls)
	}

	// 2つの解決どうしの前後は固定しない（仕様が定めていない）。どちらも
	// 登録より前であることまでを見る。
	registerAt := rec.indexOf("register")
	if registerAt < 0 {
		t.Fatalf("登録が1度も行われていない: order=%v", rec.order)
	}
	if at := rec.indexOf("resolve"); at < 0 || at > registerAt {
		t.Errorf("遮断対象の解決が登録より後（または未実施）である: order=%v（解決を済ませてからハンドラを組み立てる＝AC-9-13-1 ③）", rec.order)
	}
	if at := rec.indexOf("new-disabler"); at < 0 || at > registerAt {
		t.Errorf("AWS 設定の解決が登録より後（または未実施）である: order=%v（解決を済ませてからハンドラを組み立てる＝AC-9-13-1 ③）", rec.order)
	}

	if rec.registered == nil {
		t.Errorf("登録に値（ハンドラ）が渡っていない（AC-9-13-1 ②）")
	}
}

// ---- ④: 起動時の解決に失敗したら登録せずエラーで終える ----------------------

// TestRun_StartupResolutionFails_SkipsRegister は AC-9-13-1 ④ を固定する。
// 遮断対象の解決・AWS 設定の解決のいずれが失敗しても、登録を行わずにエラーで
// 終える（要求を受け付けてから失敗させず、コールドスタートで失敗させる。
// 既定値へ黙って落ちず、対象を推測もしない＝AC-8-5 と同型・AC-9-12）。
func TestRun_StartupResolutionFails_SkipsRegister(t *testing.T) {
	tests := []struct {
		name    string
		build   func(rec *killswitchStartupRecorder) (func() (string, error), func(context.Context) (DistributionDisabler, error))
		wantErr error
	}{
		{
			name: "遮断対象の解決に失敗したら登録しない",
			build: func(rec *killswitchStartupRecorder) (func() (string, error), func(context.Context) (DistributionDisabler, error)) {
				return rec.resolveFailing(errResolveDistributionIDFailed),
					rec.disablerReturning(&fakeDistributionDisabler{}, nil)
			},
			wantErr: errResolveDistributionIDFailed,
		},
		{
			name: "AWS 設定の解決に失敗したら登録しない",
			build: func(rec *killswitchStartupRecorder) (func() (string, error), func(context.Context) (DistributionDisabler, error)) {
				return rec.resolveReturning(dummyDistributionID),
					rec.disablerReturning(nil, errAWSConfigResolutionFailed)
			},
			wantErr: errAWSConfigResolutionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &killswitchStartupRecorder{}
			resolve, newDisabler := tt.build(rec)

			err := Run(resolve, newDisabler, rec.register)
			if err == nil {
				t.Fatalf("Run がエラーを返さなかった（起動時の解決に失敗したら既定値へ黙って落ちない＝AC-9-13-1 ④・AC-8-5 と同型）")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("errors.Is で起動時の解決のエラーへ辿れない: %v（ラップするなら %%w）", err)
			}
			if rec.registerCalls != 0 {
				t.Errorf("登録の呼び出し回数 = %d, want 0（登録を行わずにエラーで終える＝AC-9-13-1 ④）", rec.registerCalls)
			}
			if rec.registered != nil {
				t.Errorf("登録に値が渡っている（登録を行わない経路である＝AC-9-13-1 ④）")
			}
		})
	}
}

// ---- ⑤: 登録されたハンドラの振る舞い ----------------------------------------

// TestRun_RegisteredHandlerDisablesResolvedDistribution は AC-9-13-1 ⑤ の
// 正常系。登録されたハンドラへ SNS イベントを直接与えると、AC-9-12 の受け口
// 越しに無効化が呼ばれ、そこへ渡る対象が**起動時に解決した遮断対象**と一致
// する（識別子の値は仕様に書かれておらず、テストが自ら与えた値と突き合わせる）。
func TestRun_RegisteredHandlerDisablesResolvedDistribution(t *testing.T) {
	rec := &killswitchStartupRecorder{}
	fake := &fakeDistributionDisabler{
		currentConfig: DistributionConfig{ETag: "etag-run-5", Enabled: true},
	}

	if err := Run(
		rec.resolveReturning(dummyDistributionID),
		rec.disablerReturning(fake, nil),
		rec.register,
	); err != nil {
		t.Fatalf("Run がエラーを返した: %v", err)
	}
	if rec.registered == nil {
		t.Fatalf("登録に値（ハンドラ）が渡っていない（AC-9-13-1 ②）")
	}

	if err := rec.registered(context.Background(), snsEventWithMessage(`{"dummy":"budget threshold breached"}`)); err != nil {
		t.Fatalf("登録されたハンドラがエラーを返した: %v", err)
	}

	if len(fake.getCalls) != 1 || fake.getCalls[0] != dummyDistributionID {
		t.Errorf("GetDistributionConfig の呼び出し = %v, want [%q]（起動時に解決した遮断対象＝AC-9-13-1 ⑤）", fake.getCalls, dummyDistributionID)
	}
	if len(fake.updateCalls) != 1 {
		t.Fatalf("UpdateDistribution の呼び出し回数 = %d, want 1（無効化が呼ばれていない＝AC-9-13-1 ⑤）", len(fake.updateCalls))
	}
	got := fake.updateCalls[0]
	if got.distributionID != dummyDistributionID {
		t.Errorf("無効化へ渡った遮断対象 = %q, want %q（起動時に解決した対象と一致すること＝AC-9-13-1 ⑤）", got.distributionID, dummyDistributionID)
	}
	if got.config.Enabled {
		t.Errorf("無効化へ渡った設定の Enabled = true, want false（無効化していない＝AC-5-4）")
	}
}

// TestRun_RegisteredHandlerReturnsErrorWhenDisableFails は AC-9-13-1 ⑤ の
// 後段。無効化が失敗したときは成功として扱わず、失敗として返す（黙って
// 握り潰さない）。**再試行は持たせない**（非スコープ）ため、受け口の呼び出しは
// 1回だけであることも合わせて見る。
func TestRun_RegisteredHandlerReturnsErrorWhenDisableFails(t *testing.T) {
	rec := &killswitchStartupRecorder{}
	fake := &fakeDistributionDisabler{
		currentConfig: DistributionConfig{ETag: "etag-run-5b", Enabled: true},
		updateErr:     errUpdateFailed,
	}

	if err := Run(
		rec.resolveReturning(dummyDistributionID),
		rec.disablerReturning(fake, nil),
		rec.register,
	); err != nil {
		t.Fatalf("Run がエラーを返した: %v", err)
	}
	if rec.registered == nil {
		t.Fatalf("登録に値（ハンドラ）が渡っていない（AC-9-13-1 ②）")
	}

	err := rec.registered(context.Background(), snsEventWithMessage(`{"dummy":"budget threshold breached"}`))
	if err == nil {
		t.Fatalf("登録されたハンドラがエラーを返さなかった（無効化の失敗を成功として扱っている＝AC-9-13-1 ⑤）")
	}
	if !errors.Is(err, errUpdateFailed) {
		t.Errorf("errors.Is で無効化のエラーへ辿れない: %v（ラップするなら %%w）", err)
	}
	if len(fake.updateCalls) != 1 {
		t.Errorf("UpdateDistribution の呼び出し回数 = %d, want 1（再試行を持たせない＝AC-9-13-1 ⑤）", len(fake.updateCalls))
	}
}

// ---- ⑥: 本文を見ないことがハンドラの形として観測できる ----------------------

// TestRun_RegisteredHandlerIgnoresSNSMessageBody は AC-9-13-1 ⑥ を固定する。
// **本文の異なる複数の SNS イベント（本文が空のものを含む）**に対して、
// 同じ遮断対象への無効化が同じように呼ばれる。本文に別の識別子が現れる
// ケースを置くのは、本文をパースして対象を決める実装・本文から補う
// フォールバックを置く実装がここで Red になるようにするためである
// （禁止の持ち主は 11-33。本行は新しい制約を足すものではない）。
func TestRun_RegisteredHandlerIgnoresSNSMessageBody(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "本文が空",
			message: "",
		},
		{
			name:    "本文が Budget 通知らしき JSON",
			message: `{"dummy":"budget threshold breached","AlarmName":"dummy-budget"}`,
		},
		{
			name:    "本文に別のディストリビューション識別子が現れる",
			message: `{"dummy":"budget threshold breached","distributionId":"` + otherDistributionIDInMessageBody + `"}`,
		},
		{
			name:    "本文が JSON ですらない平文",
			message: "dummy plain text notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &killswitchStartupRecorder{}
			fake := &fakeDistributionDisabler{
				currentConfig: DistributionConfig{ETag: "etag-run-6", Enabled: true},
			}

			if err := Run(
				rec.resolveReturning(dummyDistributionID),
				rec.disablerReturning(fake, nil),
				rec.register,
			); err != nil {
				t.Fatalf("Run がエラーを返した: %v", err)
			}
			if rec.registered == nil {
				t.Fatalf("登録に値（ハンドラ）が渡っていない（AC-9-13-1 ②）")
			}

			if err := rec.registered(context.Background(), snsEventWithMessage(tt.message)); err != nil {
				t.Fatalf("登録されたハンドラがエラーを返した: %v（本文によって振る舞いが変わってはならない＝AC-9-13-1 ⑥）", err)
			}

			if len(fake.updateCalls) != 1 {
				t.Fatalf("UpdateDistribution の呼び出し回数 = %d, want 1（本文によらず同じように呼ばれる＝AC-9-13-1 ⑥）", len(fake.updateCalls))
			}
			if got := fake.updateCalls[0].distributionID; got != dummyDistributionID {
				t.Errorf("無効化へ渡った遮断対象 = %q, want %q（SNS メッセージの本文をパースして対象を決めない＝11-33）", got, dummyDistributionID)
			}
			if fake.updateCalls[0].config.Enabled {
				t.Errorf("無効化へ渡った設定の Enabled = true, want false（無効化していない＝AC-5-4）")
			}
		})
	}
}
