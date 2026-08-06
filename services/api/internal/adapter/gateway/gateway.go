// Package gateway は usecase/port の repository / 参照ポートを実装する
// （AC-9-3）。SQL と行 ↔ 集約（Reconstruct。AC-2-5）の変換を持ち、SQL の
// 実行手段は自身が宣言する最小の interface 越しに受け取る（D-11・AC-9-14）。
//
// **持たないのは業務ルールである**（AC-9-3）。丸め・超過／不足の算出・状態
// 遷移を SQL 側で行わない（AC-9-16-f。いずれも domain の責務）。SQL 文と
// 行 ↔ 集約の変換を本パッケージが持つこと自体は責務分担のとおりで
// （AC-9-14-c・AC-9-15・決定12）、driver/persistence は渡された文と引数を
// 実行するだけである。
//
// 依存の向きは docs/specs/workmonth-implementation-design.md AC-1-5 に従い、
// usecase/port・domain・標準ライブラリのみを import する。**driver は import
// しない**（AC-1-5）。**pgx も import しない**（AC-1-6・AC-9-14-a）。
//
// 本ファイルは SQL 実行インターフェースの宣言のみを置く。ポートの実装は
// 同パッケージの各ファイル（勤務月・契約）にある。
package gateway

import "context"

// 以下は gateway が自ら宣言する SQL 実行インターフェース（D-11・AC-9-14）。
// 具体的な形は 2026-08-05 に人間が確定した（決定12・AC-9-14-e）:
//
//   - Query / Exec / Begin の3メソッドに限る（これ以外を足さない）。
//   - Query は行の走査を返し、gateway は database/sql の Rows 相当の最小
//     interface（行を進める／値を写す／閉じる／走査中のエラーを返す）で受ける。
//   - Exec は結果行を返さない実行に使う。
//   - Begin はトランザクションを表す最小 interface（その中での Query / Exec と、
//     確定・取消しに相当する2つ）を返す。Save の原子性（AC-9-16-a・AC-10-7）は
//     この Tx を経由することで gateway 側に置く。
//
// pgx 実装は driver/persistence に置き、driver/lambda が注入する
// （AC-9-14-a）。database/sql 経由か pgx のネイティブ API かの選択は実装 PR に
// 委ねたまま（AC-13-3）で、この形はその選択に依らない。

// Rows は database/sql の Rows 相当の最小 interface（AC-9-14-e①）。
// 行 ↔ 集約の変換は gateway 側に保つ（AC-9-15・D-11 の責務分担）。
type Rows interface {
	// Next は次の行へ進める。行が無ければ false を返す。
	Next() bool
	// Scan は現在の行の値を dest（各列へのポインタ）へ写す。
	Scan(dest ...any) error
	// Close は走査を終える。
	Close() error
	// Err は走査中に生じたエラーを返す（Next が false を返した後に確認する）。
	Err() error
}

// Tx はトランザクションを表す最小 interface（AC-9-14-e③）。
// Save はこれを経由して勤務月の行と稼働実績の行の両方を書き込み、
// 成功時に Commit を1回、途中の失敗では Rollback を呼び Commit を呼ばない
// （AC-9-16-a・AC-10-7・AC-12-11④）。
type Tx interface {
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	Exec(ctx context.Context, query string, args ...any) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DB は gateway が自ら宣言する SQL 実行インターフェース（AC-9-14-a・決定12）。
// pgx 実装は driver/persistence に置き、driver/lambda が注入する。
type DB interface {
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	Exec(ctx context.Context, query string, args ...any) error
	Begin(ctx context.Context) (Tx, error)
}
