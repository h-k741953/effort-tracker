package persistence

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-12-16 ①
//     （driver/persistence のテストのうち「設定の組み立て」）。
//   - AC-10-12（接続設定の取得。設定の組み立てと接続の確立を別の関数に分け、
//     環境変数の探索を引数として受け取れる形にする。必要な設定が未設定または
//     空なら接続を試みずにエラーを返す。既定値へ黙って落ちない）。
//   - AC-10-13 ③（自ら構築するエラーの文言に、接続文字列・認証情報・環境変数の
//     値を含めない。docs/rules/security.md）。
//
// 本テストが固定しないもの（仕様が固定していないため、期待値にしない）:
//   - 環境変数の名前（AC-10-12 は名前を固定していない＝AC-13-17 と同じ扱い）。
//     差し替えた探索は要求された名前を記録するだけで、名前そのものを期待値に
//     しない（AC-12-16 ①・AC-12-10 と同じ形）。
//   - 設定を表す型の名前・フィールドの構成・戻り値が値か参照か。
//     「組み立つ」ことは「エラーにならず、返った値が型のゼロ値でない」だけで
//     観測する。
//
// 本テストの前提:
//   - プロセスの環境変数を書き換えない（os.Setenv / t.Setenv を使わない）。
//     AC-10-12 の「探索を引数で差し替えられる形」を観測する唯一の手段であり、
//     書き換えると Red にならない（AC-12-16 ①）。
//   - 探索が返す値は不透明なダミー文字列である。AC-10-12 が組み立ての段階で
//     要求しているのは「未設定・空の検出」だけであり、書式の検証は接続の確立
//     （AC-10-11）の側にある。
//
// 本テストが担保しないもの: 実際に Neon へ接続できること、pgx の戻り値を
// gateway の形へ写す部分の正しさ、接続の再利用（いずれも AC-13-23）。
// 「接続を試みずに」エラーを返すことは、設定の組み立てが AC-10-12 の求める
// とおり接続と別の関数に分かれ、ネットワークに触れずに単体で呼べること
// （本テストが実際にそう呼べていること）としてのみ観測される。

// valueFor は差し替えた探索が返すダミー値。**実値は書かない**
// （AC-10-3・docs/rules/security.md）。AC-10-13 ③ の検査で「探索から得た値が
// エラー文言へ漏れたか」を一意に見分けるため、印として名前を含める。
// 環境変数の名前そのものがエラー文言に現れることは禁じられていないが、
// この印はそれより長いため、名前だけが現れても誤検出にならない。
func valueFor(name string) string {
	return "d0-n0t-l3ak-value-of-" + name
}

// recordingLookup は環境変数の探索を差し替える手書きのテストダブル
// （ADR 0007。モックライブラリを使わない）。要求された名前を記録するだけで、
// 記録は「探索が呼ばれたこと」と「欠けたときの挙動を組み立てること」にのみ
// 使い、名前を期待値にしない（AC-12-16 ①）。
type recordingLookup struct {
	names []string
	value func(name string) (string, bool)
}

func (l *recordingLookup) lookup(name string) (string, bool) {
	l.names = append(l.names, name)
	return l.value(name)
}

// TestLoadConfig_RequiresLookedUpSettings は AC-12-16 ① の (i) と (ii) を
// **対にして**置く。対にしないと「常にエラーを返す」実装が Green になる。
func TestLoadConfig_RequiresLookedUpSettings(t *testing.T) {
	tests := []struct {
		name    string
		value   func(name string) (string, bool)
		wantErr bool
	}{
		{
			// (i) 未設定（探索が見つけられない）。
			name:    "必要な設定が未設定ならエラーになる",
			value:   func(string) (string, bool) { return "", false },
			wantErr: true,
		},
		{
			// (i) 空（存在はするが空文字。既定値へ黙って落ちない＝AC-10-12）。
			name:    "必要な設定が空文字ならエラーになる",
			value:   func(string) (string, bool) { return "", true },
			wantErr: true,
		},
		{
			// (ii) 揃っていればエラーにならず設定が組み立つ。
			name:    "必要な設定が揃っていればエラーにならない",
			value:   func(name string) (string, bool) { return valueFor(name), true },
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingLookup{value: tt.value}

			cfg, err := LoadConfig(rec.lookup)

			if tt.wantErr && err == nil {
				t.Errorf("エラーを期待したが nil だった")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("エラーを期待していないが返った: %v", err)
			}

			// 接続情報を環境変数から取得すること（AC-10-3・AC-10-12）。
			// 差し替えた探索が一度も呼ばれていなければ、環境変数を見ていない。
			if len(rec.names) == 0 {
				t.Errorf("環境変数の探索が一度も呼ばれていない（設定を環境変数から取得していない）")
			}

			if tt.wantErr {
				return
			}

			// (ii)「設定が組み立つ」の観測。型名・フィールド名に依存させないため、
			// 返った値が型のゼロ値でないことだけを見る。
			//
			// 出力は %v にする。%#v は Config.String() の伏せ字（[REDACTED]）を
			// 迂回して非公開フィールドをそのまま出すため、伏せ字を用意した型
			// （AC-10-13 ③・docs/rules/security.md）と整合しない。
			v := reflect.ValueOf(cfg)
			if !v.IsValid() || v.IsZero() {
				t.Errorf("必要な設定が揃っているのに、組み立てた設定がゼロ値だった: %v", cfg)
			}

			// (ii) 続き: 「組み立つ」の中身を直接見る。IsZero だけでは、接続文字列
			// 以外のフィールド（例: AC-10-19 の同時接続数）がゼロ値でなくなった
			// 時点で、接続文字列を載せ忘れても検出できない（往復1 W-1）。**接続
			// 文字列そのものを取り出す公開アクセサは足さない**（AC-10-13 ③・
			// docs/rules/security.md）が、本テストは driver/persistence の内部
			// テストパッケージ（package persistence）に置くため、非公開の
			// フィールド・識別子をそのまま読んでよい（AC-12-16 ③(ii) が明記する
			// 扱いを ①(ii) の検査にも適用する）。期待値は環境変数の**名前**を
			// 書かずに、差し替えた探索が実際に記録した名前から valueFor で導く
			// （AC-12-16 ①）。
			if len(rec.names) == 0 {
				t.Fatalf("環境変数の探索が一度も呼ばれていない（設定を組み立てられない）")
			}
			if want := valueFor(rec.names[0]); cfg.databaseURL != want {
				t.Errorf("組み立てた設定に、探索から得た接続文字列がそのまま載っていない: got %q, want %q", cfg.databaseURL, want)
			}
		})
	}
}

// TestLoadConfig_ErrorDoesNotLeakLookedUpValues は AC-12-16 ① の (iii)
// （AC-10-13 ③）を固定する。設定が欠けたときのエラー文言に、探索から得た値が
// そのまま含まれてはならない。
//
// 環境変数の名前を期待値にしないため、まず「揃っている」状態で1度呼んで
// 探索が要求した名前を記録し、その名前を1つずつ欠けさせて（他には値を与えて）
// エラーを起こす。値が漏れる実装（接続文字列や取得値をそのまま文言へ入れる）
// はここで Red になる。
//
// **欠落は「値を返さない」ではなく「見つからなかった印（ok=false）と一緒に値も
// 返す」形で模す。** 探索が返す値は探索の内部事情であり、見つからなかったこと
// を無視して値を文言へ入れる実装（`%s=%q` で取得値を添える等）がある。空文字を
// 返す形では、その実装が漏らすものが空文字になり検査が空回りする。したがって
// **欠けている名前も含む全名の値**が文言に現れないことを見る（AC-10-13 ③）。
// 必須設定が1つしか無い間は、これが (iii) を実際に走らせる唯一の形である。
func TestLoadConfig_ErrorDoesNotLeakLookedUpValues(t *testing.T) {
	rec := &recordingLookup{
		value: func(name string) (string, bool) { return valueFor(name), true },
	}
	if _, err := LoadConfig(rec.lookup); err != nil {
		t.Fatalf("必要な設定が揃っているのにエラーが返った: %v", err)
	}

	names := uniqueNames(rec.names)
	if len(names) == 0 {
		t.Fatalf("環境変数の探索が一度も呼ばれていない（設定を環境変数から取得していない）")
	}

	errored := 0
	for i, missing := range names {
		t.Run(fmt.Sprintf("設定%dが欠けている", i), func(t *testing.T) {
			absent := &recordingLookup{
				value: func(name string) (string, bool) {
					if name == missing {
						// 見つからなかった（ok=false）。値も返すのは、ok を
						// 無視して値を文言へ入れる実装をここで捕まえるため。
						return valueFor(name), false
					}
					return valueFor(name), true
				},
			}

			_, err := LoadConfig(absent.lookup)
			if err == nil {
				// この設定は必須ではなかった。(i) は上のテストが対で固定する。
				return
			}
			errored++

			// 欠けている名前を除外しない。除外すると、必須設定が1つしか無い
			// 現在の実装ではアサーションが一度も走らない（空回りする）。
			msg := err.Error()
			for _, name := range names {
				if strings.Contains(msg, valueFor(name)) {
					t.Errorf("エラーの文言に探索で得た値がそのまま含まれている: %q", msg)
				}
			}
		})
	}

	if errored == 0 {
		t.Errorf("どの設定を欠いてもエラーにならなかった（必要な設定が1つも無い）")
	}
}

func uniqueNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}
