// Command cloudfront-killswitch は CloudFront 従量遮断回路の実行主体である
// （docs/specs/infra-terraform.md D-17・D-18・AC-9-7〜AC-9-13・AC-5-4）。
//
// この package は業務ロジックではない（P-5）。internal/domain・
// internal/usecase・internal/adapter のいずれも import しない（AC-9-10）。
package main

// 検証対象: docs/specs/infra-terraform.md AC-9-12。
//
// CloudFront の呼び出しは、この cmd が自ら宣言する最小のインターフェース
// （DistributionDisabler。未実装）越しに受け取る。AWS SDK for Go v2 の
// CloudFront クライアントは構造体でありインターフェースではないため
// （D-19）、直接名指すとモックライブラリなしでは差し替えられない
// （ADR 0007）。テストは手書きのインメモリ Fake に差し替えて書き、実際の
// AWS API を呼ばない（AC-10-2 と同じ制約。限界は 12-15）。
//
// 宣言するメソッドは無効化に要る最小（現行設定の取得と更新＝AC-7-2 の
// 2操作に対応するもの）に限る。これ以外のメソッド（例: 既に無効化されて
// いる場合の冪等な扱いの分岐、ディストリビューションの作成・削除）は
// 仕様（AC-5-4・AC-7-2）に記述が無いため、本テストでは作らない。
//
// 担保しないもの（12-15）: 遮断 Lambda が実際にディストリビューションを
// 無効化できること。本テストが見るのは手書き Fake に対する振る舞いまで。
//
// SNS イベントの受け取り自体は aws-lambda-go で足りる（AC-9-14）が、SNS
// メッセージの本文からディストリビューション ID をどう得るかは本仕様に
// 記述が無い。推測で埋めないため、本テストは DisableDistribution（未実装。
// ディストリビューション ID を直接引数に取る）までを対象とし、SNS
// イベントのハンドラ全体は対象にしない。

import (
	"context"
	"errors"
	"testing"
)

// ---- 手書きのテストダブル（モックライブラリを入れない＝ADR 0007） ----------

// fakeDistributionDisabler は DistributionDisabler（未実装。本パッケージが
// 宣言する想定の最小インターフェース）を満たす手書きの Fake。
type fakeDistributionDisabler struct {
	currentConfig DistributionConfig
	getErr        error
	updateErr     error

	getCalls    []string
	updateCalls []distributionUpdateCall
}

type distributionUpdateCall struct {
	distributionID string
	config         DistributionConfig
}

func (f *fakeDistributionDisabler) GetDistributionConfig(_ context.Context, distributionID string) (DistributionConfig, error) {
	f.getCalls = append(f.getCalls, distributionID)
	if f.getErr != nil {
		return DistributionConfig{}, f.getErr
	}
	return f.currentConfig, nil
}

func (f *fakeDistributionDisabler) UpdateDistribution(_ context.Context, distributionID string, cfg DistributionConfig) error {
	f.updateCalls = append(f.updateCalls, distributionUpdateCall{distributionID: distributionID, config: cfg})
	return f.updateErr
}

// DistributionDisabler を満たすことをコンパイル時に固定する。
// DistributionDisabler がまだ存在しないため、この行自体が Red の一部になる。
var _ DistributionDisabler = (*fakeDistributionDisabler)(nil)

const dummyDistributionID = "E1DUMMYDISTRIBUTIONID"

// errGetConfigFailed / errUpdateFailed はテスト専用の番兵。
var errGetConfigFailed = errors.New("fakeDistributionDisabler: 現行設定の取得に失敗した（テスト用の番兵）")
var errUpdateFailed = errors.New("fakeDistributionDisabler: 更新に失敗した（テスト用の番兵）")

// TestDisableDistribution_FetchesCurrentConfigThenDisablesIt は、有効な
// ディストリビューションに対して呼ぶと、現行設定を取得したうえで
// Enabled = false にして更新する（AC-5-4「当該ディストリビューションを
// 無効化できる」・AC-7-2「取得と更新の2操作」）ことを固定する。
func TestDisableDistribution_FetchesCurrentConfigThenDisablesIt(t *testing.T) {
	fake := &fakeDistributionDisabler{
		currentConfig: DistributionConfig{ETag: "etag-1", Enabled: true},
	}

	if err := DisableDistribution(context.Background(), fake, dummyDistributionID); err != nil {
		t.Fatalf("DisableDistribution がエラーを返した: %v", err)
	}

	if len(fake.getCalls) != 1 || fake.getCalls[0] != dummyDistributionID {
		t.Errorf("GetDistributionConfig の呼び出し = %v, want [%q]（ちょうど1回、当該ディストリビューション）", fake.getCalls, dummyDistributionID)
	}

	if len(fake.updateCalls) != 1 {
		t.Fatalf("UpdateDistribution の呼び出し回数 = %d, want 1", len(fake.updateCalls))
	}
	got := fake.updateCalls[0]
	if got.distributionID != dummyDistributionID {
		t.Errorf("UpdateDistribution へ渡ったディストリビューション ID = %q, want %q", got.distributionID, dummyDistributionID)
	}
	if got.config.Enabled {
		t.Errorf("UpdateDistribution へ渡った設定の Enabled = true, want false（無効化していない）")
	}
	if got.config.ETag != "etag-1" {
		t.Errorf("UpdateDistribution へ渡った設定の ETag = %q, want %q（取得した現行設定の ETag を使っていない）", got.config.ETag, "etag-1")
	}
}

// TestDisableDistribution_GetFails_ReturnsErrorWithoutCallingUpdate は、
// 現行設定の取得に失敗したら更新を試みずにエラーを返すことを固定する
// （取得と更新の順序＝AC-7-2 の前提）。
func TestDisableDistribution_GetFails_ReturnsErrorWithoutCallingUpdate(t *testing.T) {
	fake := &fakeDistributionDisabler{getErr: errGetConfigFailed}

	err := DisableDistribution(context.Background(), fake, dummyDistributionID)
	if err == nil {
		t.Fatalf("DisableDistribution がエラーを返さなかった（取得に失敗したら更新を試みない）")
	}
	if !errors.Is(err, errGetConfigFailed) {
		t.Errorf("errors.Is で取得のエラーへ辿れない: %v（ラップするなら %%w）", err)
	}
	if len(fake.updateCalls) != 0 {
		t.Errorf("UpdateDistribution の呼び出し回数 = %d, want 0（取得に失敗したら更新しない）", len(fake.updateCalls))
	}
}

// TestDisableDistribution_UpdateFails_ReturnsError は、更新に失敗したら
// エラーを返すことを固定する。
func TestDisableDistribution_UpdateFails_ReturnsError(t *testing.T) {
	fake := &fakeDistributionDisabler{
		currentConfig: DistributionConfig{ETag: "etag-2", Enabled: true},
		updateErr:     errUpdateFailed,
	}

	err := DisableDistribution(context.Background(), fake, dummyDistributionID)
	if err == nil {
		t.Fatalf("DisableDistribution がエラーを返さなかった（更新の失敗を握りつぶしている）")
	}
	if !errors.Is(err, errUpdateFailed) {
		t.Errorf("errors.Is で更新のエラーへ辿れない: %v（ラップするなら %%w）", err)
	}
}
