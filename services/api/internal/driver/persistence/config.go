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
	"fmt"
)

// databaseURLEnv は接続文字列を収める環境変数の名前である。
//
// **名前は仕様が固定していない**（AC-10-12。どの docs にも実体が無いため、
// AC-13-17 と同じ扱いで実装に委ねられている）。テストもこの名前に依存しない
// （AC-12-16 ①）。**値（接続文字列そのもの）はコード・docs・テストに書かない**
// （AC-10-3・docs/rules/security.md）。
const databaseURLEnv = "DATABASE_URL"

// ErrMissingSetting は必要な設定が未設定または空であることを表す
// （AC-10-12。既定値へ黙って落ちない）。
//
// 文言に接続文字列・認証情報・環境変数の値を含めない（AC-10-13 ③・
// docs/rules/security.md）。**環境変数の名前は値ではない**ため、どの設定が
// 欠けているかを運用者へ伝える目的で文言に含める。
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
	// （AC-10-3）。環境変数からのみ与えられる。
	databaseURL string
}

// String は Config を誤って出力したときに接続文字列が漏れないようにするための
// ものである（AC-10-13 ③・docs/rules/security.md）。fmt の %v / %s はこの
// メソッドを使う。
func (c Config) String() string { return "persistence.Config{databaseURL: [REDACTED]}" }

// LoadConfig は環境変数の探索を受け取り、接続設定を組み立てる（AC-10-12）。
//
// **ネットワークに触れない。** 設定の組み立てと接続の確立を別の関数に分ける
// という AC-10-12 の要求に従い、接続の確立は Connect が担う。必要な設定が
// 未設定または空なら、**接続を試みずに**エラーを返す（既定値へ黙って
// 落ちない）。
//
// 探索は引数として受け取る（AC-10-12）。プロセスの環境変数を読むかどうかは
// 呼び出し側が決める（本番の呼び出し側は os.LookupEnv を渡す）。
//
// 返すエラーの文言に、探索から得た値を含めない（AC-10-13 ③）。
func LoadConfig(lookup func(name string) (string, bool)) (Config, error) {
	if lookup == nil {
		return Config{}, ErrNoLookup
	}

	databaseURL, ok := lookup(databaseURLEnv)
	if !ok || databaseURL == "" {
		// 文言に含めるのは環境変数の**名前**までで、探索が返した値は含めない
		// （AC-10-13 ③）。
		return Config{}, fmt.Errorf("%w: %s", ErrMissingSetting, databaseURLEnv)
	}

	return Config{databaseURL: databaseURL}, nil
}
