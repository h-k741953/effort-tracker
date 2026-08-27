package main

// 検証対象: docs/specs/infra-terraform.md AC-8-1〜AC-8-5・AC-8-11。
//
// SSM からの取得は、呼ぶ側（本パッケージ）が自ら宣言する最小のインター
// フェース越しに受け取る（AC-8-11 ①）。AWS SDK for Go v2 の SSM クライアント
// は構造体でありインターフェースではないため（D-19）、直接名指すとモック
// ライブラリなしでは差し替えられない（ADR 0007）。本テストは SecretFetcher
// という最小のインターフェース（未実装。本テストがコンパイルエラーになる
// ことで Red を確認する）と、それに対する手書きのインメモリ Fake だけで
// ResolveConnectionString の振る舞いを固定する。
//
// テストが固定するのはインターフェース越しの振る舞いまでであり、実際の
// SSM から取得できること・AWS SDK の実装が SecretFetcher を正しく満たす
// ことは検査しない（12-9・AC-8-11 ④）。
//
// 担保しないもの:
//   - AC-8-4「コールドスタート時に1度だけ取得し、以後は再利用する」の
//     うち、cmd/bootstrap の起動手順（Run）へどう組み込み1度だけ呼ぶかは
//     本テストの範囲外。ここで固定するのは ResolveConnectionString 単体が
//     渡された回数だけ Fetch を呼ぶことまでである。Run への配線は実装工程
//     の設計判断であり、対応するテストはその配線を追加する際に足す。
//   - SDK の型を宣言したインターフェースの引数・戻り値へ露出させていない
//     ことは、型シグネチャそのものが担保する（コンパイルが通る時点で
//     SecretFetcher のシグネチャに標準ライブラリの型以外が現れていないと
//     確認できる。動的なテストでは別途検査しない＝12-13 と同型）。

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ---- 手書きのテストダブル（モックライブラリを入れない＝ADR 0007） ----------

// fakeSecretFetcher は SecretFetcher（未実装。本パッケージが宣言する想定の
// 最小インターフェース）を満たす手書きの Fake。呼ばれたパラメータ名を記録し、
// 設定された値またはエラーを返す。
type fakeSecretFetcher struct {
	values    map[string]string
	err       error
	requested []string
	callCount int
}

func (f *fakeSecretFetcher) FetchSecret(_ context.Context, name string) (string, error) {
	f.callCount++
	f.requested = append(f.requested, name)
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.values[name]
	if !ok {
		return "", errors.New("fakeSecretFetcher: 設定されていないパラメータ名が要求された")
	}
	return v, nil
}

// SecretFetcher を満たすことをコンパイル時に固定する。SecretFetcher が
// まだ存在しないため、この行自体が Red の一部になる。
var _ SecretFetcher = (*fakeSecretFetcher)(nil)

const dummyParameterName = "/effort-tracker/test/neon-connection-string"
const dummyResolvedValue = "dummy-resolved-value-not-a-real-connection-string"

// errFetchFailed はテスト専用の番兵。取得または復号の失敗を表す
// （AC-8-5「取得または復号に失敗したらエラーで終える」）。
var errFetchFailed = errors.New("fakeSecretFetcher: 取得または復号に失敗した（テスト用の番兵）")

// TestResolveConnectionString_ReturnsFetchedValue は AC-8-11 ②
// 「パラメータ名を受け取り、復号済みの値を返す」を固定する。
func TestResolveConnectionString_ReturnsFetchedValue(t *testing.T) {
	fetcher := &fakeSecretFetcher{values: map[string]string{dummyParameterName: dummyResolvedValue}}

	got, err := ResolveConnectionString(context.Background(), fetcher, dummyParameterName)
	if err != nil {
		t.Fatalf("ResolveConnectionString がエラーを返した: %v", err)
	}
	if got != dummyResolvedValue {
		t.Errorf("ResolveConnectionString() = %q, want %q", got, dummyResolvedValue)
	}
}

// TestResolveConnectionString_PassesParameterNameToFetcher は、渡した
// パラメータ名がそのまま Fetcher へ渡ることを固定する（AC-8-11 ②）。
func TestResolveConnectionString_PassesParameterNameToFetcher(t *testing.T) {
	fetcher := &fakeSecretFetcher{values: map[string]string{dummyParameterName: dummyResolvedValue}}

	if _, err := ResolveConnectionString(context.Background(), fetcher, dummyParameterName); err != nil {
		t.Fatalf("ResolveConnectionString がエラーを返した: %v", err)
	}

	if fetcher.callCount != 1 {
		t.Errorf("Fetcher の呼び出し回数 = %d, want 1", fetcher.callCount)
	}
	if len(fetcher.requested) != 1 || fetcher.requested[0] != dummyParameterName {
		t.Errorf("Fetcher へ渡ったパラメータ名 = %v, want [%q]", fetcher.requested, dummyParameterName)
	}
}

// TestResolveConnectionString_FetchFails_ReturnsErrorWithoutValue は AC-8-5
// 「取得または復号に失敗したら、既定値へ黙って落ちずにエラーで終える」を
// 固定する（ここでは ResolveConnectionString 単体の振る舞いとして。
// ハンドラ登録の抑止は Run への配線の話であり本テストの範囲外）。
func TestResolveConnectionString_FetchFails_ReturnsErrorWithoutValue(t *testing.T) {
	fetcher := &fakeSecretFetcher{err: errFetchFailed}

	got, err := ResolveConnectionString(context.Background(), fetcher, dummyParameterName)
	if err == nil {
		t.Fatalf("ResolveConnectionString がエラーを返さなかった（取得の失敗を既定値へ落としてはならない＝AC-8-5）")
	}
	if !errors.Is(err, errFetchFailed) {
		t.Errorf("errors.Is で取得のエラーへ辿れない: %v（ラップするなら %%w）", err)
	}
	if got != "" {
		t.Errorf("エラー時に空でない値が返った: %q（取得失敗時は値を返さない）", got)
	}
}

// TestResolveConnectionString_ErrorMessageDoesNotLeakValueOrName は、
// エラーの文言にパラメータ名・取得した値を含めないことを固定する
// （AC-8-9・docs/rules/security.md）。
func TestResolveConnectionString_ErrorMessageDoesNotLeakValueOrName(t *testing.T) {
	fetcher := &fakeSecretFetcher{err: errFetchFailed}

	_, err := ResolveConnectionString(context.Background(), fetcher, dummyParameterName)
	if err == nil {
		t.Fatalf("ResolveConnectionString がエラーを返さなかった")
	}
	if strings.Contains(err.Error(), dummyParameterName) {
		t.Errorf("エラーの文言にパラメータ名が含まれている（AC-8-9）: %v", err)
	}
}
