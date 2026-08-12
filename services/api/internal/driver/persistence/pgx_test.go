package persistence

import (
	"context"
	"errors"
	"testing"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-10-12
//     （必要な設定が未設定または空なら、**接続を試みずに**エラーを返す。
//     既定値へ黙って落ちない）。
//
// AC-10-12 が禁じた「既定値へ黙って落ちる」経路は、LoadConfig を通さずに
// Connect を呼ぶと復活しうる。Config は公開型であり、フィールドが非公開でも
// **パッケージ外からゼロ値を構築できる**ためである。空の接続文字列を pgx へ
// 渡すと、pgx は libpq 互換の既定値（PGHOST 等の環境変数・既定ホスト）へ
// フォールバックする。したがって空の検査は Connect の側にも要る（**新しい仕様
// 判断ではなく AC-10-12 の再掲**）。
//
// 本テストが担保しないもの: 実際に Neon へ接続できること、pgx の戻り値を
// gateway の形へ写す部分の正しさ、接続の再利用、pgx 由来のエラーの文言
// （いずれも AC-13-23 ①②④⑤）。テストは pgx を import しない（AC-12-6・
// ADR 0017。手書きの偽 pgx も作らない）ため、観測できるのは「接続へ進む前に
// 弾く」ところまでである。

// TestConnect_RejectsUnconfiguredConfig は、LoadConfig を経由せずに構築された
// 設定（＝接続文字列が空）で Connect を呼んでも、pgx の既定値へ落ちずに
// エラーになることを固定する（AC-10-12）。
//
// **既にキャンセルした context を渡す。** 空の検査が接続より前に効いていれば
// context の状態は結果に影響しない。効いていなければ pgx の接続が試みられ、
// 返るのは context 由来ないし接続失敗のエラーであって ErrMissingSetting では
// ないため Red になる。あわせて、テストがネットワークへ出て待つことも防ぐ。
func TestConnect_RejectsUnconfiguredConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	db, err := Connect(ctx, Config{})

	if !errors.Is(err, ErrMissingSetting) {
		t.Errorf("接続設定が空なのに、未設定を表すエラーが返らなかった: %v", err)
	}
	if db != nil {
		t.Errorf("接続設定が空なのに、実装が返った")
	}
}
