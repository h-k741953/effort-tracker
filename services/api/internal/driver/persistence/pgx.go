package persistence

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
)

// 本ファイルは adapter/gateway が宣言した SQL 実行インターフェース
// （AC-9-14-e＝決定12）の pgx 実装と、その組み立て（接続の確立）を持つ。
//
// **適合は adapter/gateway の型を名指して満たす**（AC-10-14・決定14）。
// 同じメソッド集合を持つ別名の interface を本パッケージに宣言しない
// （決定14 で退けた案 (b)）。適合の検査はコンパイル時に行われる
// （AC-12-16 ②。gateway_conformance_test.go）。
//
// **pgx の型を公開 API の引数・戻り値に露出させない**（AC-10-13 ④）。
// pgx の型が現れるのは、非公開の構造体のフィールドと、その内部の実装だけで
// ある。呼び出し側（driver/lambda）は gateway が宣言した型と標準の error しか
// 見ないため、pgx を import せずに済む（AC-1-6・D-11）。
//
// **pgx 由来のエラーはそのまま返す**（AC-10-13 ②）。usecase/port の番兵へ
// 変換しない（AC-10-13 ①。「行が無い」は Rows の走査に表れ、番兵への変換は
// gateway の責務＝AC-9-19-a）。
//
// 本ファイルが持たないもの: SQL 文（渡された文と引数を実行するだけ＝
// AC-9-14-c）・行 ↔ 集約の変換（AC-9-15）・業務ルール（AC-9-3）。

// Connect は設定を受け取り、確立済みの SQL 実行インターフェースの実装を返す
// （AC-10-10 ②・AC-10-11）。
//
// **接続の確立をパッケージの初期化で行わない**（AC-10-11）。init()・パッケージ
// 変数での接続確立・プロセス内シングルトンを本パッケージは持たない。いつ確立
// し、いくつ保持し、再利用するか（コールドスタート時に1度だけ＝AC-10-2）を
// 決めるのは呼び出し側であり（AC-10-9）、本関数は確立済みの実装を返すところ
// までを担う。
//
// 戻り値を gateway が宣言した interface 型にするのは、pgx の型を公開 API へ
// 露出させないため（AC-10-13 ④）と、呼び出し側（AC-10-8 ③）が受け取る形が
// まさにこの interface であるためである。**接続を閉じる手段は公開しない**
// （AC-10-11 は公開の可否を固定していない。Lambda 実行環境の寿命と接続の寿命
// を一致させ、閉じる判断を呼び出し側に持ち込まない）。
//
// エラーは pgx 由来のものをそのまま返す（AC-10-13 ②）。**設定の値を文言へ
// 足さない**（AC-10-13 ③）。
func Connect(ctx context.Context, cfg Config) (gateway.DB, error) {
	pool, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		return nil, err
	}

	// 確立済みであることを実際に確かめる（AC-10-11「確立済みの実装を返す」）。
	// 失敗したらプールを解放してから返す。
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &pgxDB{pool: pool}, nil
}

// pgxDB は gateway.DB の pgx 実装（AC-10-14）。メソッドは Query / Exec /
// Begin の3つで、足しも省きもしない（AC-9-14-e）。
type pgxDB struct {
	pool *pgxpool.Pool
}

// Query は渡された文と引数をそのまま実行し、行の走査を gateway が要求する形へ
// 写して返す（AC-9-14-e ①）。
func (d *pgxDB) Query(ctx context.Context, query string, args ...any) (gateway.Rows, error) {
	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRows{rows: rows}, nil
}

// Exec は結果行を返さない実行に使う（AC-9-14-e ②）。pgx は実行結果
// （CommandTag）を返すが、gateway 側の形はそれを要求しないため破棄する
// （AC-10-14 の「この形へ写すのが driver/persistence の仕事」）。
func (d *pgxDB) Exec(ctx context.Context, query string, args ...any) error {
	_, err := d.pool.Exec(ctx, query, args...)
	return err
}

// Begin はトランザクションを gateway が要求する形へ写して返す
// （AC-9-14-e ③）。Save の原子性は gateway 側にある（AC-9-16-a・AC-10-7）。
func (d *pgxDB) Begin(ctx context.Context) (gateway.Tx, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxTx{tx: tx}, nil
}

// pgxRows は gateway.Rows の pgx 実装（AC-9-14-e ①）。
// 行 ↔ 集約の変換は持たない（AC-9-15。gateway 側にある）。
type pgxRows struct {
	rows pgx.Rows
}

// Next は次の行へ進める。
func (r *pgxRows) Next() bool { return r.rows.Next() }

// Scan は現在の行の値を dest へ写す。pgx 由来のエラーをそのまま返す
// （AC-10-13 ②）。
func (r *pgxRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }

// Close は走査を終える。**pgx の Close は戻り値を持たない**ため、gateway が
// 要求する形（error を返す）へ写すにあたり nil を返す（AC-10-14）。走査中に
// 生じたエラーは Err が返す（AC-9-14-e ①）ので、ここで握り潰されるものは
// 無い。
func (r *pgxRows) Close() error {
	r.rows.Close()
	return nil
}

// Err は走査中に生じたエラーを返す。pgx 由来のエラーをそのまま返す
// （AC-10-13 ②）。
func (r *pgxRows) Err() error { return r.rows.Err() }

// pgxTx は gateway.Tx の pgx 実装（AC-9-14-e ③）。
// メソッドは Query / Exec / Commit / Rollback の4つで、足しも省きもしない。
type pgxTx struct {
	tx pgx.Tx
}

// Query はトランザクションの中で文を実行し、行の走査を gateway が要求する形へ
// 写して返す。
func (t *pgxTx) Query(ctx context.Context, query string, args ...any) (gateway.Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRows{rows: rows}, nil
}

// Exec はトランザクションの中で結果行を返さない実行を行う。pgx の実行結果
// （CommandTag）は gateway 側の形が要求しないため破棄する。
func (t *pgxTx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.Exec(ctx, query, args...)
	return err
}

// Commit はトランザクションを確定する。
func (t *pgxTx) Commit(ctx context.Context) error { return t.tx.Commit(ctx) }

// Rollback はトランザクションを取り消す。
func (t *pgxTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
