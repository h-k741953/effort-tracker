package persistence

import (
	"reflect"
	"testing"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-12-16 ①
//     （driver/persistence のテストのうち「設定の組み立て」）。
//   - AC-10-12（**2026-08-26 追記＝`docs/specs/infra-terraform.md` AC-8-8**。
//     接続文字列の解決は設定の組み立ての前段に置き、**設定の組み立ては、
//     取得済みの値を引数として受け取る形**にする。設定の組み立ての内側から
//     SSM を呼ばない。必要な設定が未設定または空なら、接続を試みずに
//     エラーを返す。既定値へ黙って落ちない）。
//   - `docs/specs/infra-terraform.md` AC-8-6（設定の組み立てには取得済みの値を
//     渡す。組み立ての内側から SSM を呼ばない）。
//
// **2026-08-26 の設計変更**: これまで LoadConfig は `lookup(databaseURLEnv)` を
// 自ら呼んで接続文字列を得ていたが、AC-10-12 の更新により**接続文字列は
// 解決済みの値として引数で渡す形**へ変わった（接続文字列の解決＝SSM からの
// 取得は `cmd/bootstrap` 側の前段の手続きが担い、要求本文はそちらにはない
// ＝ADR 0004。docs/specs/infra-terraform.md AC-8 を参照）。**「環境変数の
// 探索が見つけられない（not found）」と「値が空」の区別は、接続文字列が
// 直接の引数になったことで意味を失う**（引数はただの文字列であり、
// found/not-found の状態を持たない）。したがって本テストが固定するのは
// 「空文字列ならエラー」「非空ならそのまま Config へ載る」の2点である。
//
// tester が決めた名前・形（implementer が合わせる対象。AC-13-17 と同じ
// 扱い。max_open_connections_test.go の同種の宣言に揃える）:
//   - LoadConfig の新しいシグネチャ: `LoadConfig(lookup func(name string)
//     (string, bool), connectionString string) (Config, error)`。
//     **`lookup` は引数として残す**（AC-10-15 ①「プロセスの環境変数の探索を
//     渡した設定の組み立て」の文言に合わせ、将来 databaseURL 以外の設定が
//     環境変数から加わる余地を残すため）。**現時点では `lookup` は
//     databaseURL の取得に使われない**ため、本テストは `lookup` の呼び出し
//     回数を期待値にしない（呼ばれても呼ばれなくても Green であってよい）。
//   - 接続文字列の持ち方（AC-10-13 ③）: Config の非公開フィールド
//     databaseURL（string）。本テストは package persistence の内部テスト
//     パッケージに置くため直接読む（AC-12-16 ③(ii) が明記する扱いを
//     ①(ii) の検査にも適用する）。**接続文字列を取り出す公開アクセサは
//     足さない**（AC-10-13 ③・docs/rules/security.md）。
//
// 本テストが固定しないもの:
//   - databaseURL 以外のフィールドの構成・型の名前・戻り値が値か参照か。
//   - `lookup` がどう使われるか（現時点では未使用でもよい）。
//
// 本テストが担保しないもの（AC-13-23 と同型）: 接続文字列の**解決**
// （SSM からの取得）が正しく行われること・実際に Neon へ接続できること・
// pgx の戻り値を gateway の形へ写す部分の正しさ・接続の再利用。
// **エラーの文言が接続文字列を漏らさないこと（AC-10-13 ③）の検査は、
// 本レイヤーには自然な失敗経路（空文字以外の失敗要因）が無いため置かない。
// この検査は解決の失敗経路を持つ層（cmd/bootstrap。main_test.go）に置く。**

// dummyConnectionString は「取得済みの値」として直接渡すダミー値。
// **実値は書かない**（AC-10-3・docs/rules/security.md）。
const dummyConnectionString = "dummy-resolved-connection-string-not-a-real-value"

// noopLookup は「何も見つけられない」探索。databaseURL の取得に使われない
// ことを確認するためのものではなく、単に LoadConfig のシグネチャを満たす
// ためのプレースホルダである。
func noopLookup(string) (string, bool) { return "", false }

// TestLoadConfig_RequiresConnectionStringArgument は AC-12-16 ① (i)(ii) を
// **対にして**置く。対にしないと「常にエラーを返す」実装が Green になる。
//
// **2026-08-26 更新**: 接続文字列は `lookup` 経由ではなく引数として直接
// 渡す（AC-10-12・infra-terraform AC-8-6）。
func TestLoadConfig_RequiresConnectionStringArgument(t *testing.T) {
	tests := []struct {
		name             string
		connectionString string
		wantErr          bool
	}{
		{
			// (i) 空文字（既定値へ黙って落ちない＝AC-10-12）。
			name:             "接続文字列が空ならエラーになる",
			connectionString: "",
			wantErr:          true,
		},
		{
			// (ii) 取得済みの値が渡っていればエラーにならず設定が組み立つ。
			name:             "接続文字列が渡っていればエラーにならない",
			connectionString: dummyConnectionString,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(noopLookup, tt.connectionString)

			if tt.wantErr && err == nil {
				t.Errorf("エラーを期待したが nil だった")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("エラーを期待していないが返った: %v", err)
			}

			if tt.wantErr {
				return
			}

			// 「設定が組み立つ」の観測は2段で行う（往復2 W-4 と同じ形）。
			//
			// 1段目: 返った値が型のゼロ値でないことだけを見る。
			v := reflect.ValueOf(cfg)
			if !v.IsValid() || v.IsZero() {
				t.Errorf("接続文字列が渡っているのに、組み立てた設定がゼロ値だった: %v", cfg)
			}

			// 2段目: 「組み立てた設定に、渡した接続文字列がそのまま載っている
			// こと」を直接見る（AC-10-12「取得済みの値を引数として受け取る
			// 形」の中心の検査。1段目だけでは、接続文字列以外のフィールド
			// （例: AC-10-19 の同時接続数）がゼロ値でなくなった時点で、
			// 接続文字列を載せ忘れても検出できないため）。**接続文字列その
			// ものを取り出す公開アクセサは足さない**（AC-10-13 ③）が、本
			// テストは driver/persistence の内部テストパッケージ
			// （package persistence）に置くため、非公開のフィールドを
			// そのまま読んでよい（AC-12-16 ③(ii) と同じ扱い）。
			if cfg.databaseURL != tt.connectionString {
				t.Errorf("組み立てた設定に、渡した接続文字列がそのまま載っていない: got %q, want %q", cfg.databaseURL, tt.connectionString)
			}
		})
	}
}
