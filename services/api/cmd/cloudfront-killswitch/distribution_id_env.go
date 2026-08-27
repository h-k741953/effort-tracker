// 検証対象: docs/specs/infra-terraform.md AC-5-4・AC-9-12（後段）・AC-11-33。
//
// 遮断対象のディストリビューション ID は、Terraform が遮断 Lambda の環境変数へ
// 注入し、この cmd はそれを読むだけである（SNS メッセージの本文はパースしない
// ＝AC-11-33）。環境変数が未設定・空のときは、既定値へ黙って落ちず、対象を
// 推測もせず、エラーで終える（AC-8-5 と同型）。
//
// この関数は環境変数を読むだけであり、AWS SDK を import しない。
package main

import (
	"errors"
	"os"
)

// cloudfrontDistributionIDEnvVarName は、Terraform 側が遮断 Lambda の
// 環境変数へ注入する名前と一致していなければならない
// （infra/terraform/lambda_cloudfront_killswitch.tf の environment.variables。
// 一致は機械検査されない＝12-28。担保はレビューと規律）。
const cloudfrontDistributionIDEnvVarName = "CLOUDFRONT_DISTRIBUTION_ID"

// distributionIDFromEnv は環境変数からディストリビューション ID を読み取る。
// 設定されていればその値をそのまま返し、未設定・空のときはエラーで終える。
func distributionIDFromEnv() (string, error) {
	v := os.Getenv(cloudfrontDistributionIDEnvVarName)
	if v == "" {
		return "", errors.New("cloudfront-killswitch: " + cloudfrontDistributionIDEnvVarName + " が未設定または空である")
	}
	return v, nil
}
