// Command bootstrap はドメイン API の Lambda のエントリポイントである
// （docs/specs/workmonth-implementation-design.md AC-10-15・AC-10-18・D-12）。
//
// 持つのは AC-10-15 の5つだけである: ①環境変数の探索を渡した設定の組み立て、
// ②接続の確立（プロセスの生存期間に1度だけ）、③全体の組み立ての呼び出し
// （AC-10-16）、④イベントアダプタの生成（AC-10-17）、⑤Lambda ランタイムへの
// ハンドラ登録。
//
// 持たないもの: ルーティング・DI 配線の中身（driver/lambda＝AC-10-1・AC-10-2・
// AC-10-16）／SQL 文・行 ↔ 集約の変換・業務ルール／`code`・ステータスの対応表
// （AC-11-10。presenter に一元化したまま）／pgx の型（AC-1-7・AC-10-13 ④）／
// 要求ごとの処理（AC-10-8 ②）。
//
// import してよいのは driver/lambda・driver/persistence・標準ライブラリ・
// aws-lambda-go と、SQL 実行インターフェースの型を名指すための adapter/gateway
// だけである（AC-1-7）。usecase/* ・domain を直接 import しない。pgx も
// import しない（AC-1-6・D-11）。
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	awslambda "github.com/aws/aws-lambda-go/lambda"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/persistence"
)

// neonConnectionStringParameterNameEnv は、Neon 接続文字列を収める SSM
// パラメータの名前を渡す Lambda の環境変数の名前である（AC-8-1）。
// **値ではなく名前**であるため、ここに書いてよい（docs/rules/security.md。
// infra/terraform/lambda_domain_api.tf が同じ名前で環境変数を設定する）。
const neonConnectionStringParameterNameEnv = "NEON_CONNECTION_STRING_SSM_PARAMETER_NAME"

// errMissingParameterName は、SSM パラメータ名の環境変数が未設定または空
// であることを表す（infra-terraform AC-8-5 と同型。既定値へ黙って落ちず、
// 解決も接続も試みない）。
var errMissingParameterName = errors.New("bootstrap: SSM パラメータ名の環境変数が未設定または空である")

// Run は AC-10-15 の⓪〜⑤を、main() から呼ばれる1つの手続きとして取り出したもの
// である（AC-10-18。テストは AC-12-17 ③）。
//
// 受け取るのは4つである（AC-10-18）:
//
//	resolve  接続文字列の解決（infra-terraform AC-8-11 が定める SecretFetcher
//	         越しの取得と同じ形。パラメータ名を受け取り、復号済みの値を
//	         返す）。
//	lookup   環境変数の探索（AC-10-12 と同じ形。プロセスの環境変数を暗黙に
//	         読まない）。SSM パラメータ名（AC-8-1）も本関数の内側でこの
//	         探索を使って得る。
//	connect  接続の確立（AC-10-11 が公開する手段と同じ形。設定を受け取り、
//	         SQL 実行インターフェースの実装を返す）。
//	register ランタイムへの登録（AC-10-17 が返した形を受け取る）。
//
// port.Clock の実装は引数にせず、内側で SystemClock（AC-10-4）に固定する。
// 差し替えの引数を作らないことで「本番で SystemClock が渡ること」を構造として
// 固定する（AC-10-18。AC-13-19 ② の残る範囲を狭める）。
//
// 順序と回数を固定する: パラメータ名の探索 → 接続文字列の解決（ちょうど1回）→
// 設定の組み立て → 接続の確立（ちょうど1回）→ AC-10-16 の組み立て →
// AC-10-17 の生成 → 登録（ちょうど1回）。パラメータ名の探索・接続文字列の
// 解決・設定の組み立て・接続の確立のいずれかに失敗したら、登録を行わずに
// エラーを返す（要求を受け付けてから失敗させず、コールドスタートで失敗
// させる）。
//
// 接続の確立と組み立ては、登録するハンドラの内側（＝要求ごとに実行される位置）
// に置かない（AC-10-2 の「コールドスタート時に1度だけ確立し再利用する」を構造と
// して満たすため。実際に1度だけ確立され再利用されることは依然として観測でき
// ない＝AC-13-24 ①）。
//
// 返すエラーの文言に、探索で得た値・接続文字列・認証情報を含めない
// （AC-10-13 ③・docs/rules/security.md）。ラップは %w で行い errors.Is が通る
// 形にする（AC-11-9）。
func Run(
	resolve func(ctx context.Context, parameterName string) (string, error),
	lookup func(name string) (string, bool),
	connect func(ctx context.Context, cfg persistence.Config) (gateway.DB, error),
	register func(h lambda.EventHandler),
) error {
	// ⓪(i) SSM パラメータ名の探索（AC-8-1「Lambda の環境変数が持つのは
	// SSM パラメータの名前」）。未設定または空なら、解決も接続も登録も
	// 行わずにエラーで終える（infra-terraform AC-8-5 と同型。既定値へ
	// 黙って落ちない）。
	parameterName, ok := lookup(neonConnectionStringParameterNameEnv)
	if !ok || parameterName == "" {
		return fmt.Errorf(
			"bootstrap: %w: %s",
			errMissingParameterName,
			neonConnectionStringParameterNameEnv,
		)
	}

	// ⓪ 接続文字列の解決（AC-10-18 (0)）。探索で得たパラメータ名（値）を
	// エラー文言に含めない（infra-terraform AC-8-9）。
	connectionString, err := resolve(context.Background(), parameterName)
	if err != nil {
		return fmt.Errorf("bootstrap: 接続文字列の解決に失敗した: %w", err)
	}

	// ① 設定の組み立て（ネットワークに触れない＝AC-10-12）。
	cfg, err := persistence.LoadConfig(lookup, connectionString)
	if err != nil {
		return fmt.Errorf("bootstrap: 接続設定の組み立てに失敗した: %w", err)
	}

	// ② 接続の確立（ちょうど1回。要求ごとの位置には置かない）。
	db, err := connect(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("bootstrap: データベース接続の確立に失敗した: %w", err)
	}

	// ③④ 全体の組み立て（AC-10-16）とイベントアダプタの生成（AC-10-17）。
	// Clock は差し替えず SystemClock に固定する（AC-10-18）。
	handler := lambda.NewEventHandler(lambda.NewHandler(db, lambda.SystemClock{}))

	// ⑤ ランタイムへの登録（ちょうど1回）。
	register(handler)

	return nil
}

// startLambda は Lambda ランタイムへの登録を Run が要求する形（AC-10-18 (iii)）
// へ合わせる。aws-lambda-go の Start は任意の値を受け取るため、型を名指すのに
// この1行が要る。分岐も計算も持たない。
func startLambda(h lambda.EventHandler) { awslambda.Start(h) }

// resolveConnectionString は Run が受け取る (0) の実体である（AC-10-18）。
// SSM クライアントの組み立て（newSSMSecretFetcher・secret_resolver.go）を
// 呼び出しの中へ遅延させ、Run が実際にパラメータ名を得たとき（＝パラメータ名
// の環境変数が設定されているとき）にのみ AWS 設定の解決を試みる。
func resolveConnectionString(ctx context.Context, parameterName string) (string, error) {
	fetcher, err := newSSMSecretFetcher(ctx)
	if err != nil {
		return "", err
	}
	return ResolveConnectionString(ctx, fetcher, parameterName)
}

// main は取り出した手続き（Run）を、プロセスの環境変数・実際の接続文字列の
// 解決・実際の接続の確立・実際のランタイムへの登録に結びつけて1回だけ呼び、
// 失敗したら異常終了するだけである（AC-10-15。分岐と計算を残さない。
// ここに残った分はどのテストも観測しない＝AC-13-24 ③）。
func main() {
	if err := Run(resolveConnectionString, os.LookupEnv, persistence.Connect, startLambda); err != nil {
		log.Fatalf("bootstrap: 起動に失敗した: %v", err)
	}
}
