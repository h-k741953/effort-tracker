// Command cloudfront-killswitch は CloudFront 従量遮断回路の実行主体である
// （docs/specs/infra-terraform.md D-17・D-18・AC-9-7〜AC-9-13・AC-5-4）。
//
// この package は業務ロジックではない（P-5）。internal/domain・
// internal/usecase・internal/adapter のいずれも import しない（AC-9-10）。
package main

// 検証対象: docs/specs/infra-terraform.md AC-9-12（後段）・AC-8-5 と同型・
// AC-11-33。
//
// 遮断対象のディストリビューション ID は、Terraform が遮断 Lambda の
// 環境変数へ注入し、この cmd はそれを読む（SNS メッセージの本文はパースし
// ない＝AC-11-33。SNS は「閾値に触れた」という契機としてのみ使う）。
//
// 本テストが固定するのは次の2点だけである。
//   - 環境変数を読み取れること（設定されていれば、その値をそのまま返す）
//   - 環境変数が未設定・空のときは、既定値へ黙って落ちず、対象を推測も
//     せず、エラーで終える（AC-8-5 と同型）
//
// 環境変数の「名前」は本仕様（AC-9-12）が持たない（名前の持ち主は構成側）。
// ここで使う CLOUDFRONT_DISTRIBUTION_ID という名前は、このテストが暫定的に
// 固定するインターフェースである（.tftest.hcl のリソース名の扱いと同型）。
// 実装側は同じ名前で作ってよいし、都合が悪ければテストごと見直す。
//
// distributionIDFromEnv（未実装）は本テストが要求する最小の関数である。
// この識別子が存在しないため、本テストはコンパイルエラーとして Red を踏む。
//
// 手書きのテストダブル・モックライブラリは用いない（ADR 0007）。環境変数の
// 読み取りという単純な I/O であり、実際の process 環境変数を t.Setenv で
// 操作して検証する（プロセス全体の状態を汚さない）。

import (
	"os"
	"testing"
)

const cloudfrontDistributionIDEnvVarNameForTest = "CLOUDFRONT_DISTRIBUTION_ID"

// TestDistributionIDFromEnv は、環境変数が設定されているときはその値を
// そのまま返し、未設定・空のときはエラーで終える（既定値へ落ちない・対象を
// 推測しない）ことをテーブル駆動で固定する（AC-9-12・AC-8-5 と同型）。
func TestDistributionIDFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		set     bool   // 環境変数を設定するか（false なら未設定のまま検査する）
		value   string // set が true のときに設定する値
		want    string
		wantErr bool
	}{
		{
			name:    "未設定のときはエラーで終える",
			set:     false,
			wantErr: true,
		},
		{
			name:    "空文字のときはエラーで終える（既定値へ落ちない）",
			set:     true,
			value:   "",
			wantErr: true,
		},
		{
			name:    "設定されていればその値をそのまま返す",
			set:     true,
			value:   "E1DUMMYDISTRIBUTIONID",
			want:    "E1DUMMYDISTRIBUTIONID",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(cloudfrontDistributionIDEnvVarNameForTest, tt.value)
			} else {
				unsetEnvForTest(t, cloudfrontDistributionIDEnvVarNameForTest)
			}

			got, err := distributionIDFromEnv()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("distributionIDFromEnv() がエラーを返さなかった（未設定・空のときはエラーで終える＝AC-8-5 と同型）")
				}
				return
			}

			if err != nil {
				t.Fatalf("distributionIDFromEnv() が予期せずエラーを返した: %v", err)
			}
			if got != tt.want {
				t.Errorf("distributionIDFromEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

// unsetEnvForTest は対象の環境変数を未設定の状態にしたうえで、テスト終了時に
// 元の状態（設定されていたか・その値）へ復元する。t.Setenv は「値を設定する」
// ことしかできず「未設定にする」ことができないため、未設定を検査するケースは
// この補助関数で行う（プロセス全体の状態を汚さない）。
func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	original, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("os.Unsetenv(%q) に失敗した: %v", key, err)
	}

	t.Cleanup(func() {
		if existed {
			if err := os.Setenv(key, original); err != nil {
				t.Fatalf("os.Setenv(%q, ...) による復元に失敗した: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("os.Unsetenv(%q) による復元に失敗した: %v", key, err)
		}
	})
}
