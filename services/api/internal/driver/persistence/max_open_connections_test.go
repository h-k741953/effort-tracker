package persistence

import "testing"

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-12-16 ③
//     （driver/persistence のテストのうち「同時に開く接続の本数」。
//     2026-08-13 追記＝決定15）。
//   - AC-10-19 ①②③④（driver/persistence が確立する接続は、1つの実行環境
//     （プロセス）あたり同時に最大1本。本数の出所を driver/persistence の
//     内側の1箇所にする。pgx も実 DB も要さずに読み取れる形にする。本数を
//     環境変数から読まない）。
//
// tester が決めた名前・形（implementer が合わせる対象。AC-10-19 ②③が
// 「名前・形は固定しない」としているため、本テストが決める＝AC-13-17 と
// 同じ扱い）:
//   - 本数の出所（AC-10-19 ②）: driver/persistence 内の非公開定数
//     maxOpenConnections（int）。値は 1。config.go に置く想定（LoadConfig が
//     使う、ネットワークに触れない側＝AC-10-12 の結果に載せるため。
//     AC-10-19 ③）。
//   - 設定が本数をどう持つか（AC-10-19 ③）: Config の非公開フィールド
//     maxConns（int）。接続文字列へ埋め込む形は採らない。標準ライブラリ
//     だけで読み取れるよう、フィールドを同一パッケージ内のテストから直接
//     読む形を選んだ（AC-12-16 ③・AC-10-13 ③「本数を読み取るためだけに
//     公開のアクセサを足さない」）。
//
// 本テストが担保しないもの（AC-13-25。「担保した」と書かない）:
//   - 組み立てた本数の指定が pgx に実際に効いていること（AC-13-25 ①）。
//   - Neon 側で同時に開かれる接続の本数が予約同時実行数との積に収まる
//     こと、Neon 側の許容値そのもの（AC-13-25 ②）。
//   - 実行環境の使い回しをまたいで接続が再利用され、確立が再び起きない
//     こと（AC-13-25 ③）。
//   - 本数を環境変数から読んでいないことを名前に依存しない形で完全に
//     観測すること（AC-13-25 ④。探索を読んだうえで 1 へ落とす実装は
//     本テストでは Green のまま残る）。
// 上記の担保はレビューと、デプロイ後の経路の確認に留まる（AC-13-25）。

// TestMaxOpenConnections_IsOne は AC-12-16 ③ (i) を固定する。
//
// 期待値の 1 はテスト側に直書きし、実装側の値どうしを突き合わせない
// （突き合わせるとどの値でも Green になる）。出所（maxOpenConnections）を
// 1 以外へ変えると Red になる。
//
// (i) だけでは、出所の値を確立の手段（LoadConfig）が使っていない実装が
// Green になるため、TestLoadConfig_SetsMaxOpenConnectionsToOne
// （(ii)(iii)）と対にして置く。
func TestMaxOpenConnections_IsOne(t *testing.T) {
	if maxOpenConnections != 1 {
		t.Errorf("同時に開く接続の本数の出所（AC-10-19 ②）が 1 ではない: %d", maxOpenConnections)
	}
}

// TestLoadConfig_SetsMaxOpenConnectionsToOne は AC-12-16 ③ (ii)(iii) を
// 固定する。
//
// (ii): 接続文字列が渡された状態（config_test.go の「渡っていればエラーに
// ならない」ケースと同じ形）で設定を組み立て、その結果（Config の非公開
// フィールド maxConns）から標準ライブラリだけで本数の指定を読み取り、値が
// 1 であることを見る（AC-10-19 ③）。指定が載っていない（＝ゼロ値のまま）
// 実装、および 1 以外が載る実装では Red になる。
//
// (iii): **2026-08-26 更新**: 接続文字列は `lookup` 経由ではなく引数として
// 直接渡す形へ変わった（AC-10-12・infra-terraform AC-8-6。config_test.go を
// 参照）ため、本数が接続文字列の値に依存しないことは、ダミーの接続文字列
// （本数として解釈できない不透明な文字列）を渡しても maxConns が 1 になる
// ことで示す（AC-10-19 ④「本数を環境変数から読まない」の帰結として、接続
// 文字列からも読まないことを兼ねて確認する）。`lookup` はシグネチャを
// 満たすためだけに渡し、本テストは呼び出しの有無・回数を期待値にしない。
//
// プロセスの環境変数を書き換えない（os.Setenv / t.Setenv を使わない。
// AC-10-12 の「探索を引数で差し替えられる形」を観測する唯一の手段であり、
// 書き換えると Red にならない＝AC-12-16 ①）。
func TestLoadConfig_SetsMaxOpenConnectionsToOne(t *testing.T) {
	cfg, err := LoadConfig(noopLookup, dummyConnectionString)
	if err != nil {
		t.Fatalf("必要な設定が揃っているのにエラーが返った: %v", err)
	}

	if cfg.maxConns != 1 {
		t.Errorf("組み立てた設定に載る同時接続数の指定が 1 ではない: %d", cfg.maxConns)
	}
}
