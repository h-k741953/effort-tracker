// Package persistence は Neon（PostgreSQL）への接続を担う最外周である
// （docs/specs/workmonth-implementation-design.md AC-10-3・AC-10-10）。
//
// 本パッケージが持つのは次の2つだけである（AC-10-10）:
//
//  1. adapter/gateway が宣言した SQL 実行インターフェース（AC-9-14-e＝決定12。
//     DB の Query / Exec / Begin と Rows 相当・Tx 相当）の pgx 実装（pgx.go）。
//  2. その実装を組み立てる手段（本ファイルの設定の組み立てと、pgx.go の接続の
//     確立。AC-10-11・AC-10-12）。
//
// 持たないもの: SQL 文（adapter/gateway が持つ＝AC-9-14-c）／行 ↔ 集約の変換
// （AC-9-15。domain/workmonth を import しない）／業務ルール（AC-9-3）／
// usecase/port の番兵への変換（AC-10-13 ①。gateway の責務）／ルーティング・
// DI 配線（AC-10-1・AC-10-2。driver/lambda の責務）／認可・HTTP の関心事。
//
// pgx を import してよい唯一のパッケージである（AC-1-6・D-11・ADR 0017）。
// adapter/gateway は import するが（決定14。driver → adapter は内向き）、
// usecase/interactor・adapter/controller・adapter/presenter・driver/lambda は
// import しない（AC-10-10）。
package persistence

import (
	"errors"
)

// maxOpenConnections は1つの実行環境（プロセス）あたり同時に開く接続の
// 上限である（AC-10-19 ①②。決定15）。
//
// Lambda の1つの実行環境は同時に1要求しか処理しないため、同時実行数と
// 接続本数が1対1に対応し、Neon 側で同時に開かれる本数の上限は予約同時
// 実行数（docs/rules/cost-guardrails.md）だけで決まる。**この値そのものは
// docs/rules/cost-guardrails.md へ足さない**（本行が定めるのは「実行あたり
// 1本」という構造であり、値の実体は同ファイルの予約同時実行数の1箇所の
// まま＝ADR 0004）。
//
// 本数の出所はこの1箇所だけとし（AC-10-19 ②）、環境変数からは読まない
// （AC-10-19 ④。外から変えられる形にしない）。LoadConfig がこの値を
// Config.maxConns へ載せ、Connect（pgx.go）がそれを使って接続の確立を
// 行う。
const maxOpenConnections = 1

// ErrMissingSetting は必要な設定が未設定または空であることを表す
// （AC-10-12。既定値へ黙って落ちない）。
//
// 文言に接続文字列・認証情報を含めない（AC-10-13 ③・docs/rules/security.md）。
var ErrMissingSetting = errors.New("persistence: 必要な接続設定が未設定または空である")

// ErrNoLookup は環境変数の探索が渡されなかったことを表す。
//
// 本パッケージはプロセスの環境変数を暗黙に読まない（AC-10-12。「プロセスの
// 環境変数を書き換えないとテストできない形にしない」）。呼び出し側が
// os.LookupEnv 等を明示的に渡す。
var ErrNoLookup = errors.New("persistence: 環境変数の探索が渡されていない")

// Config は接続の確立に必要な設定である（AC-10-10 ②）。
//
// 組み立ては LoadConfig だけが行い、フィールドは公開しない。設定の実体
// （接続文字列）は認証情報を含むため、値を取り出す手段も公開しない
// （docs/rules/security.md）。Connect は同一パッケージから直接読む。
//
// **型の名前・フィールドの構成・値か参照かを仕様は固定していない**
// （AC-13-17 と同じ扱い。テストもこれらに依存しない＝AC-12-16 ①）。
type Config struct {
	// databaseURL は Neon への接続文字列。**実値をコードに書かない**
	// （AC-10-3）。LoadConfig の引数（取得済みの値）からのみ与えられる。
	databaseURL string

	// maxConns は同時に開く接続の上限（AC-10-19 ③）。LoadConfig が
	// maxOpenConnections から代入し、環境変数からは読まない（AC-10-19 ④）。
	// 接続文字列とは別のフィールドとして持つ（接続文字列へ埋め込む形は
	// 採らない。tester が決めた形）。
	maxConns int
}

// String は Config を誤って出力したときに接続文字列が漏れないようにするための
// ものである（AC-10-13 ③・docs/rules/security.md）。fmt の %v / %s はこの
// メソッドを使う。
func (c Config) String() string { return "persistence.Config{databaseURL: [REDACTED]}" }

// LoadConfig は環境変数の探索と、取得済みの接続文字列を受け取り、接続設定を
// 組み立てる（AC-10-12・infra-terraform AC-8-6）。
//
// **ネットワークに触れない。** 接続文字列の解決（SSM からの取得）は
// 呼び出し側（cmd/bootstrap）が本関数の前段で行い、その結果を
// connectionString としてそのまま受け取る。本関数の内側から SSM を呼ばない
// （AC-8-6）。connectionString が未設定または空なら、**接続を試みずに**
// エラーを返す（既定値へ黙って落ちない）。
//
// lookup は引数として残す（AC-10-15 ①「プロセスの環境変数の探索を渡した
// 設定の組み立て」の形に合わせ、将来 databaseURL 以外の設定が環境変数から
// 加わる余地を残すため）。**現時点では lookup は接続文字列の取得に使わない**
// （接続文字列の解決は cmd/bootstrap 側が担う＝AC-10-12・AC-8-6）。
//
// 返すエラーの文言に、探索から得た値・接続文字列を含めない（AC-10-13 ③）。
func LoadConfig(lookup func(name string) (string, bool), connectionString string) (Config, error) {
	if lookup == nil {
		return Config{}, ErrNoLookup
	}

	if connectionString == "" {
		return Config{}, ErrMissingSetting
	}

	return Config{databaseURL: connectionString, maxConns: maxOpenConnections}, nil
}
