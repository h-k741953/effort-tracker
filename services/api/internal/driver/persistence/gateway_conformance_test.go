package persistence

import (
	"testing"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-12-16 ②
//     （gateway が要求する形への適合を**コンパイル時に**検査する）。
//   - AC-10-14（pgx 実装は AC-9-14-e が確定した形をそのまま満たす。
//     DB の3つ・Rows 相当の4つ・Tx 相当の4つ。メソッドを足さない・省かない）。
//   - AC-9-14-e＝決定12（Query / Exec / Begin の3メソッド。Query は Rows 相当を、
//     Begin は Tx 相当を返す）。
//   - 決定14（driver/persistence は adapter/gateway を import する。
//     driver → adapter は内向きで AC-1-5・AC-1-6 に反しない）。
//
// **適合は adapter/gateway が宣言した型を名指して満たす**（AC-10-14・決定14）。
// 同じメソッド集合を持つ別名の interface を driver/persistence 側に宣言して
// それへ代入しない（決定14 で退けた案 (b)。それでは gateway が要求する形への
// 適合を示さず、常に Green になる）。
//
// メソッドが欠けている・署名が異なれば、**Red はテストの失敗ではなくビルドの
// 失敗として現れる**（AC-12-16 ②・AC-13-12 と同型）。実行時に観測できることは
// 無い。
//
// 本テストが示すのは**メソッドが揃っていること**までであり、**中身は示さない**
// （AC-13-23 ②）。pgx の戻り値を gateway の形へ写す部分の正しさ（Next / Scan /
// Close / 走査中のエラー、Exec の結果の破棄、Tx の確定・取消し）は、テストが
// pgx を import できない（AC-12-6・ADR 0017。手書きの偽 pgx も作らない）ため
// どの単体テストでも Red にならない。実接続・原子性・SQL 文・接続の再利用も
// 同様に射程外（AC-13-23 ①③④⑤）。
//
// 代入先の型は gateway 側から引き、型名を期待値にしない（AC-12-10 と同じ形）。
// 代入元（pgx 実装）の型名は本仕様が固定していない（AC-10-14・AC-13-17 と同じ
// 扱い）ため、本テストが名前を決める。
var (
	_ gateway.DB   = (*pgxDB)(nil)
	_ gateway.Rows = (*pgxRows)(nil)
	_ gateway.Tx   = (*pgxTx)(nil)
)

// TestPgxImplementationsSatisfyGatewayInterfaces は、上の var 宣言が
// コンパイルできること自体が検査の実体であることを、テスト一覧に残すための
// ものである（AC-12-16 ②）。実行時のアサーションは持たない。
func TestPgxImplementationsSatisfyGatewayInterfaces(t *testing.T) {
	t.Log("適合の検査はコンパイル時に行われる（AC-12-16 ②・AC-13-12 と同型）")
}
