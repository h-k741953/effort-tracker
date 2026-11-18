# プロセス検査の除外軸 — spec-link と review-trail に infra/ を加える

Issue #174 の仕様。2つのプロセス検査（`.github/workflows/spec-link.yml` と `.github/workflows/review-trail.yml`）が「変更なし」として SKIP する判定（除外軸）の範囲を、`services/` `apps/` から `services/` `apps/` `infra/` へ広げる。その受け入れ条件を、変更ファイルの集合ごとの期待値として定める。

- **決定の正解**: `docs/adr/0025-reference-model-with-infra-in-exclusion-axis.md`（提案中。ADR 0004 を置き換える）。決定の理由・代替案はこの仕様へ書き写さない（ADR 0004 / 0025 決定3）
- **決定者**: 人間（プロジェクトオーナー）。2026-11-18 に discussion #167 で、論点1 = (a)・論点2 = (a-1) を明示選択した（オーケストレーター経由で受領）。この仕様の条文は仕様工程が起草した
- **決定済み**: Q-1（`infra/` だけを変える PR で往復を回さない場合の扱い）・Q-2（判定をどこで検査するか）・Q-3（切り出すスクリプトが持つ判定の範囲）と、AC-6-6 を要求として残すかは、2026-11-18 に人間が明示選択した（main の AskUserQuestion）。内容は「決定（D）」節が持つ
- **未決**: なし

---

## 対象

| 項目 | 値 |
|---|---|
| 検査1 | `.github/workflows/spec-link.yml`（PR 本文が `docs/` を参照しているか） |
| 検査2 | `.github/workflows/review-trail.yml`（往復証跡が定型で残り、往復数が上限内か） |
| 判定の入力 | 検査の名前（`spec-link` / `review-trail`）、PR のラベル集合、変更ファイルの一覧（`git diff --name-only "$BASE_SHA" HEAD` の出力。1行に1パス、リポジトリのルートからの相対パス）。スクリプトへの渡し方は AC-6-7 |
| 往復証跡の本体 | `.github/scripts/check-review-trail.sh`（本仕様では変えない） |
| SKIP／検査する／失敗 の判定 | `.github/scripts/check-exclusion-axis.sh`（本仕様で新設する。P-2 の判定のうち、PR 本文の検査を除く部分を持つ。D-2・D-5・AC-6） |
| テスト | `.github/scripts/test-check-exclusion-axis.sh`（本仕様で新設する。`make test-scripts` から回す。D-2・AC-6） |

## 前提（P）

本仕様の着手時点（2026-11-18）に、実装の字面から読んだ事実である（特定時点の記録）。現況の持ち主は2つの yml と `.github/scripts/` であり、実装が変われば、この節ではなく実装が正である。出所は、ブランチ `ci/174-infra-exclusion-axis` のコミット 590c439 の時点の `.github/workflows/spec-link.yml`・`.github/workflows/review-trail.yml`・`.github/scripts/test-check-review-trail.sh` である。

### P-1. 除外軸はワークフローの `run:` に直接書かれている

- spec-link.yml・review-trail.yml のいずれも、除外軸を `grep -qE '^(services|apps)/'` で判定する。変更ファイルの一覧のうち1行でも一致すれば検査し、1行も一致しなければ SKIP する
- この判定は**どちらの検査でも、`.github/scripts/` のスクリプトではなくワークフローの `run:` の中にある**。`make test-scripts` が回す fixture（`.github/scripts/test-check-review-trail.sh`）は `check-review-trail.sh` の本体だけを試験し、除外軸の分岐を試験していない（同ファイルに `services` `apps` `SKIP` の字面は無い）

### P-2. 判定の順序

review-trail.yml の判定は次の順で進む。

1. `no-spec` ラベルがあれば SKIP する
2. `.github/scripts/check-review-trail.sh` と `.github/workflows/review-trail.yml` の実在を確かめ、どちらかが無ければ**失敗**する
3. 変更ファイルの一覧に、正規表現 `^\.github/(scripts/|workflows/review-trail\.yml$)` に一致する行があれば検査する（チェッカ自身の変更。SKIP させない）
4. 3 に当たらず、除外軸に一致する行が無ければ SKIP する
5. それ以外は検査する

spec-link.yml の判定は次の順で進む。

1. `no-spec` ラベルがあれば SKIP する
2. 除外軸に一致する行が無ければ SKIP する
3. PR 本文が `docs/[A-Za-z0-9._/-]+\.md` に一致する文字列を含めば OK、含まなければ NG

spec-link.yml には、review-trail.yml の 2・3 に当たる分岐（チェッカ自身の変更で SKIP しない分岐）が無い。

### P-3. SKIP を許すラベル

- **2つの検査に共通して SKIP を許すラベルは `no-spec` だけである。** review-trail.yml だけを SKIP させるラベルも、spec-link.yml だけを SKIP させるラベルも無い
- review-trail.yml が読むもう1つのラベル `loop-extended` は、往復数の上限超過を人間が承認したことを記録するもので、SKIP を許さない（`check-review-trail.sh` へ渡り、上限の判定にだけ効く）

---

## 判定の定義

**除外軸の一致**を次のとおり定める。2つの検査は、この1つの定義を使う。

> 変更ファイルの一覧のうち、**行頭が `services/` `apps/` `infra/` のいずれかである行**が1行以上あるとき、一覧は除外軸に**一致する**。

- 「行頭が `infra/` である」は、行の先頭から `infra/` の6文字（スラッシュを含む）が並ぶことをいう。大文字と小文字を区別する
- この形（行頭への固定・末尾のスラッシュ・大文字と小文字の区別）は、既存の `services/` `apps/` の判定の形をそのまま `infra/` へ広げたものである。形そのものは変えない

---

## 受け入れ条件

| AC | 一行趣旨 | 状態 |
|---|---|---|
| AC-1 | 除外軸の一致を、パスの境界を含む期待値表で定める | 確定 |
| AC-2 | review-trail は、ラベルの無い PR で `infra/` の変更を検査する | 確定 |
| AC-3 | spec-link は、ラベルの無い PR で `infra/` の変更に `docs/` 参照を求める | 確定 |
| AC-4 | 2つの検査の除外軸はそろっている | 確定 |
| AC-5 | `no-spec` と `infra/` の変更が重なったときの扱い | 確定（D-1） |
| AC-6 | 判定を検査する手段と、スクリプトの入出力の形 | 確定（D-2・D-5・D-6） |
| AC-7 | SKIP の出力と docs の記述 | 確定 |
| AC-8 | 機械検査の限界 | 確定 |

AC-2・AC-3・AC-5 の期待値は、各検査の判定として書く。`check-exclusion-axis.sh` の fixture では、これを AC-6-7 の出力へ次のとおり読み替える: 「検査する」「OK」「NG」は `CHECK`、「SKIP」は `SKIP`、「失敗する」は終了コード 1。OK と NG の区別（PR 本文の検査）はスクリプトの外にあり、fixture に入らない（AC-8-6）。

### AC-1. 除外軸の一致の期待値

入力は変更ファイルの一覧（1行に1パス）である。各行の期待値は「一致する」か「一致しない」のどちらかとする。

| # | 変更ファイルの一覧 | 期待値 | 確かめること |
|---|---|---|---|
| 1-1 | `infra/terraform/iam_github_oidc.tf` | 一致する | `infra/` を加えたこと（本仕様の主眼） |
| 1-2 | `infra/bootstrap/versions.tf` | 一致する | `infra/` の下の別のディレクトリでも一致する |
| 1-3 | `services/api/go.mod` | 一致する | 既存の `services/` を保つ |
| 1-4 | `apps/web/package.json` | 一致する | 既存の `apps/` を保つ |
| 1-5 | `docs/specs/process-check-exclusion-axis.md` | 一致しない | docs だけの変更 |
| 1-6 | `docs/infra/notes.md` | 一致しない | `infra/` が行の途中に出ても一致しない（行頭への固定） |
| 1-7 | `infra-legacy/main.tf` | 一致しない | `infra` で始まる別のディレクトリ名は一致しない（末尾のスラッシュ） |
| 1-8 | `infrastructure/main.tf` | 一致しない | 同上 |
| 1-9 | `infra` | 一致しない | スラッシュを持たない、ちょうど `infra` という名前のファイル |
| 1-10 | `Infra/main.tf` | 一致しない | 大文字と小文字を区別する |
| 1-11 | `.github/workflows/ci.yml` | 一致しない | `.github/` は除外軸に入らない |
| 1-12 | `docs/adr/0025-reference-model-with-infra-in-exclusion-axis.md` と `infra/terraform/budgets.tf` の2行 | 一致する | 混在。1行でも一致すれば一致する |
| 1-13 | `services/api/go.mod`・`apps/web/package.json`・`infra/terraform/locals.tf` の3行 | 一致する | 3つすべて |
| 1-14 | `docs/infra/notes.md` と `infra-legacy/main.tf` の2行 | 一致しない | 境界の外の行だけが並んでも一致しない |
| 1-15 | 空（変更ファイルが1つも無い） | 一致しない | 空の一覧 |

- 1-1・1-2・1-12 と 1-6〜1-10・1-14 は、**`infra/` を加えたことと、その境界の両側**を見る。前者だけ（一致する側だけ）を試験すると、`infra` を行頭に固定しない・スラッシュを要しない実装が通る。後者だけを試験すると、`infra/` を加えていない実装が通る。**両側を同じテストで持つ**
- 1-3・1-4 は既存の範囲を保つことを見る。`infra/` を加えるときに `services/` `apps/` を落とす実装を落とす
- スクリプトの出力への読み替えは AC-6-5 が定める（一致する = `CHECK`、一致しない = `SKIP`）

### AC-2. review-trail の判定（ラベル無し）

ラベルの無い PR について、P-2 の順序で判定した結果を定める。判定に使う除外軸は「判定の定義」による。2-1〜2-7・2-9 はいずれも、チェッカ自身の2つのパスが実在する状態とする。

| # | 変更ファイルの一覧 | 期待値 |
|---|---|---|
| 2-1 | `infra/terraform/iam_github_oidc.tf` | 検査する（本仕様の前は SKIP していた） |
| 2-2 | `docs/specs/process-check-exclusion-axis.md` | SKIP |
| 2-3 | `docs/infra/notes.md` と `infra-legacy/main.tf` の2行 | SKIP |
| 2-4 | `.github/scripts/check-review-trail.sh` | 検査する（チェッカ自身の変更。本仕様で変えない） |
| 2-5 | `.github/workflows/review-trail.yml` と `docs/specs/process-check-exclusion-axis.md` の2行 | 検査する（同上） |
| 2-6 | `.github/workflows/spec-link.yml` | SKIP（review-trail から見て自身の変更ではない。本仕様で変えない） |
| 2-7 | `services/api/go.mod` | 検査する（本仕様で変えない） |
| 2-8 | `infra/terraform/budgets.tf`。ただし `.github/scripts/check-review-trail.sh` が実在しない（`.github/workflows/review-trail.yml` は実在する） | 失敗する（P-2 の 2。本仕様で変えない） |
| 2-9 | `.github/scripts/check-exclusion-axis.sh` と `docs/specs/process-check-exclusion-axis.md` の2行 | 検査する（D-2 で新設するスクリプトも、P-2 の 3 の `.github/scripts/` の接頭辞に入る） |
| 2-10 | `infra/terraform/budgets.tf`。ただし `.github/workflows/review-trail.yml` が実在しない（`.github/scripts/check-review-trail.sh` は実在する） | 失敗する（P-2 の 2。2つのパスのどちらが欠けても失敗する。本仕様で変えない） |

- 「検査する」は、往復証跡の取得と `check-review-trail.sh` の実行へ進むことをいう。その合否は本仕様の範囲の外である
- 2-4〜2-6・2-8・2-10 は、除外軸を広げるときにチェッカ自身の分岐とパスの実在の確認を壊さないことを見る。2-8 と 2-10 は対で持つ。片方だけでは、2つのパスの一方しか確かめない実装と区別できない
- 2-9 は、除外軸の判定を `.github/scripts/` へ切り出したあとも、その判定を変える PR で review-trail が SKIP しないことを見る
- **fixture での与え方**: 検査の名前は `review-trail`、`PR_LABELS` は空文字とする。「チェッカ自身の2つのパスが実在する状態」は、カレントディレクトリにその2つのパスを置くことで作る（AC-6-7）

### AC-3. spec-link の判定（ラベル無し）

| # | 変更ファイルの一覧 | PR 本文 | 期待値 | スクリプトの出力（AC-6-7） |
|---|---|---|---|---|
| 3-1 | `infra/terraform/iam_github_oidc.tf` | `docs/` 参照を含まない（例: `IAM を狭める`） | NG（本仕様の前は SKIP していた） | `CHECK` |
| 3-2 | `infra/terraform/iam_github_oidc.tf` | `docs/specs/infra-terraform.md` を含む | OK | `CHECK` |
| 3-3 | `docs/infra/notes.md` | `docs/` 参照を含まない | SKIP | `SKIP` |
| 3-4 | `infra-legacy/main.tf` | `docs/` 参照を含まない | SKIP | `SKIP` |
| 3-5 | `services/api/go.mod` | `docs/` 参照を含まない | NG（本仕様で変えない） | `CHECK` |
| 3-6 | `.github/workflows/spec-link.yml` | `docs/` 参照を含まない | SKIP（spec-link に自身の変更の分岐は無い。本仕様で変えない。AC-8-3） | `SKIP` |
| 3-7 | `.github/scripts/check-exclusion-axis.sh` | `docs/` 参照を含まない | SKIP（3-6 と同じ理由。D-2 で新設するスクリプトの変更も spec-link は SKIP する。AC-8-3） | `SKIP` |

- 3-1 と 3-2 は対で持つ。3-1 だけでは、`infra/` の変更で常に NG を返す実装と区別できない
- **スクリプトは PR 本文を受け取らない**（D-5）。3-1・3-2・3-5 のスクリプトの出力は、いずれも `CHECK`（本文の検査へ進む）である。OK と NG の区別は spec-link.yml の `run:` に残り、fixture に入らない（AC-8-6）。したがって 3-1 と 3-2 の対が見分けるもの（本文で OK と NG が分かれること）は、fixture では確かめられない
- **fixture での与え方**: 検査の名前は `spec-link`、`PR_LABELS` は空文字とする

### AC-4. 2つの検査の除外軸はそろっている

| # | 条件 |
|---|---|
| 4-1 | AC-1 の 1-1〜1-15 の**どの一覧についても**、spec-link の除外軸の判定（P-2 の spec-link の 2）と review-trail の除外軸の判定（P-2 の review-trail の 4）は同じ結果を返し、いずれも AC-1 の期待値に等しい。review-trail のチェッカ自身の変更の分岐（P-2 の review-trail の 3）は、この比較に含めない（1-1〜1-15 にはこの分岐に当たる一覧が無い） |
| 4-2 | 一方の検査の除外軸から `infra/` を落とす、または一方にだけ別のディレクトリを足すと、4-1 を確かめるテストが落ちる。片方の検査だけを試験して 4-1 を満たしたと扱わない |

4-1 は、判定を1か所（`.github/scripts/check-exclusion-axis.sh`）に置き、2つの検査がそれを呼ぶ形で満たす（D-2・AC-6-4）。スクリプトの中で検査の名前ごとに除外軸を分けて持つ実装は、AC-6-5 が2つの名前の両方で 1-1〜1-15 を試すことで落とす。4-2 の「一方にだけ別のディレクトリを足す」は、ワークフローの `run:` に除外軸の判定を自前で書き足すことを含む。これを落とすのは AC-6-4 の検査である。

### AC-5. `no-spec` と `infra/` の変更が重なったときの扱い

D-1 により、`no-spec` は `infra/` の変更でも `services/` `apps/` の変更と同じく2つの検査をどちらも SKIP させる。往復を回さない `infra/` の変更を示す新しい手段（専用のラベル等）は設けない（P-3 を変えない）。

ラベルは名前の完全一致で比べる（P-3 の `no-spec` は、その名前のラベルをいう）。特記の無い行は、チェッカ自身の2つのパスが実在する状態とする。

| # | ラベル | 変更ファイルの一覧 | 検査 | 期待値 |
|---|---|---|---|---|
| 5-1 | `no-spec` | `infra/terraform/iam_github_oidc.tf` | review-trail | SKIP（D-1） |
| 5-2 | — | — | — | 該当なし（D-1 により、review-trail だけを SKIP させるラベルの行は置かない。番号は欠番とする） |
| 5-3 | `no-spec` | `infra/terraform/iam_github_oidc.tf` | spec-link | SKIP（spec-link の `no-spec` は本仕様で変えない） |
| 5-4 | `no-spec` | `services/api/go.mod` | review-trail | SKIP（本仕様で変えない） |
| 5-5 | `no-spec` | `.github/scripts/check-review-trail.sh` | review-trail | SKIP（`no-spec` はチェッカ自身の分岐より先に判定される。本仕様で変えない。P-2） |
| 5-6 | `loop-extended` のみ | `docs/specs/process-check-exclusion-axis.md` | review-trail | SKIP（`loop-extended` は SKIP にも検査にも効かない。P-3） |
| 5-7 | `no-spec-draft` と `draft-no-spec` の2つ | `infra/terraform/iam_github_oidc.tf` | review-trail | 検査する（`no-spec` を名前の一部に含むだけのラベルは `no-spec` ではない。本仕様で変えない） |
| 5-8 | `no-spec-draft` と `draft-no-spec` の2つ | `infra/terraform/iam_github_oidc.tf` | spec-link | 検査する（同上。スクリプトの出力は `CHECK`） |
| 5-9 | `loop-extended` と `no-spec` の2つ（この順） | `infra/terraform/iam_github_oidc.tf` | review-trail | SKIP（`no-spec` がラベル集合のどこにあっても効く） |
| 5-10 | `no-spec` | `infra/terraform/iam_github_oidc.tf`。ただし `.github/scripts/check-review-trail.sh` が実在しない | review-trail | SKIP（`no-spec` はパスの実在の確認より先に判定される。本仕様で変えない。P-2 の 1・2） |

- 5-7・5-8 は、ラベルの比べ方が部分一致・前方一致・後方一致のいずれであっても SKIP を返す実装を落とす（`no-spec-draft` は前方一致、`draft-no-spec` は後方一致で当たる）
- 5-9 は、ラベル集合の最初の1つだけを見る実装を落とす
- **fixture での与え方**: 「ラベル」列は `PR_LABELS` に1行に1つで与える（AC-6-7）

### AC-6. 判定を検査する手段と、スクリプトの入出力の形

D-2 により、判定を `.github/scripts/` の1本のスクリプトへ切り出し、2つのワークフローがそれを呼ぶ。D-5 により、スクリプトは各検査の「SKIP／検査する／失敗」の判定までを持つ。fixture は `make test-scripts` から回す。

| # | 条件 |
|---|---|
| 6-1 | 検査は `make test` から回り、CI はそれと同じターゲットを呼ぶ（`docs/rules/commands.md`）。CI にだけある検査を作らない |
| 6-2 | AC-1 の期待値表を、一致する側と一致しない側の両方について持つ（AC-1 の注記）。AC-2・AC-3・AC-5・6-7 の期待値表の各行も fixture が持つ（ただし AC-3 は「スクリプトの出力」列で）。期待値表に行の無い要求を散文だけで残さない |
| 6-3 | P-2 の判定のうち PR 本文の検査（spec-link の 3）を除くもの — `no-spec` の判定、チェッカ自身の2つのパスの実在の確認、チェッカ自身の変更の分岐、除外軸の一致と、それらの順序 — は `.github/scripts/check-exclusion-axis.sh` が持つ（D-5）。その fixture は `.github/scripts/test-check-exclusion-axis.sh` とし、`make test-scripts` から呼ぶ（`make test-scripts` の外に新しいターゲットを作らない） |
| 6-4 | `spec-link.yml` と `review-trail.yml` は、いずれも判定に `check-exclusion-axis.sh` を呼び、`run:` に除外軸の判定（`services` `apps` `infra` を並べた正規表現など）を自前で持たない。fixture は2つの yml の字面を読んでこれを確かめる。**呼び出しの読み出しが0件のとき緑にしない。** また、どちらか一方の yml の写しに自前の除外軸の判定を書き足すと落ちることを、同じ fixture で確かめる（陽性対照）。2つの yml の片方だけを読んで満たしたと扱わない（AC-4-2）。6-3 でスクリプトへ移した他の分岐（`no-spec` の判定・パスの実在の確認・チェッカ自身の変更の分岐）も、yml の `run:` に重ねて書かない。ただし字面で確かめるのは除外軸の判定だけである（AC-8-7） |
| 6-5 | AC-1 の 1-1〜1-15 の一覧を、検査の名前 `spec-link` と `review-trail` の**両方**について、`PR_LABELS` を空文字とし、カレントディレクトリにチェッカ自身の2つのパスを置いて `check-exclusion-axis.sh` に与える。AC-1 で「一致する」の一覧では `CHECK`、「一致しない」の一覧では `SKIP` を返すことを fixture が確かめる（1-1〜1-15 はチェッカ自身の変更の分岐に当たらない） |
| 6-6 | `check-exclusion-axis.sh` は、判定できなかったこと（6-7 の「判定できない」）を、`SKIP` とも `CHECK` とも同じ結果で返さない。また「失敗」（P-2 の review-trail の 2）とも終了コードで区別する。2つのワークフローは、スクリプトが終了コード 0 で標準出力に `SKIP` を出したときだけ SKIP し、終了コード 0 で `CHECK` を出したときだけ次の検査（PR 本文の検査／往復証跡の検査）へ進む。それ以外のすべて（終了コード 1・2 を含む 0 以外、スクリプトが実在しない、終了コード 0 でも標準出力が `SKIP` `CHECK` のどちらでもない）では失敗する。根拠は `docs/harness/verification-loop.md` の「検査対象を読めなかったことは、違反が無いことを意味しない」。これを要求として残すことは、2026-11-18 に人間が明示選択した（D-6）。前段（スクリプトの性質）は 6-7 の期待値表として fixture が確かめ、後段（ワークフローの振る舞い）は AC-8-6 の限界に入る |
| 6-7 | スクリプトの入力と出力の形、判定の順序を、下の「6-7 の定め」のとおりとする。fixture は下の「6-7 の期待値」の各行を持つ |

- スクリプトの実装の細部（シェルの書き方・正規表現の字面）は本仕様が定めない。「判定の定義」・P-2 の順序・下の 6-7 の定めと、各期待値表が正である
- 6-7 の形（引数・環境変数の名前・`SKIP` `CHECK` の字句・終了コードの割り当て・パスの解決のしかた）は、D-5 を満たすために仕様工程が定めたものであり、人間の決定ではない

#### 6-7 の定め

**呼び出し**: `bash .github/scripts/check-exclusion-axis.sh <検査の名前>`

| 入力 | 渡し方 | 形 | 読めないとき |
|---|---|---|---|
| 検査の名前 | 第1引数。引数はちょうど1つ | `spec-link` か `review-trail` のどちらか。完全一致で比べ、大文字と小文字を区別する | 引数が無い・2つ以上ある・2つの名前のどちらでもない → 判定できない |
| ラベル集合 | 環境変数 `PR_LABELS` | ラベル名を1行に1つ。各行を、行全体の完全一致でラベル名と比べる（前後の空白を除かない。部分一致しない）。空文字はラベルが1つも無いことを表す | 未設定 → 判定できない（空文字とは区別する） |
| 変更ファイルの一覧 | 環境変数 `CHANGED_FILE_LIST` に、一覧を書いたファイルのパスを入れる | ファイルの中身は「対象」表の「判定の入力」の形（1行に1パス）。空のファイルは空の一覧（AC-1 の 1-15）を表す。末尾に改行の無い最後の行も1行として読む | 未設定・空文字・実在しないパス・ディレクトリ → 判定できない |

- **実在を確かめるパス**（P-2 の review-trail の 2）は、`.github/scripts/check-review-trail.sh` と `.github/workflows/review-trail.yml` の2つに固定し、スクリプトが自分で持つ。引数・環境変数で差し替えない。2つは**カレントディレクトリからの相対パス**として確かめ、種類を問わず（ファイルでもディレクトリでも）そのパスに何かがあれば実在とする。ワークフローはリポジトリのルートをカレントディレクトリとして呼ぶ。fixture は一時ディレクトリへ移り、2つのパスを置く・置かないことで実在の有無を作る。スクリプトはスクリプト自身の置き場所を基準にしない
- スクリプトは、`CHANGED_FILE_LIST` のファイルと上の2つのパスの実在のほかに、リポジトリのファイルを読まない（fixture がリポジトリの外の一時ディレクトリで回せるようにする）

**出力**:

| 結果 | 終了コード | 標準出力 |
|---|---|---|
| SKIP | 0 | `SKIP` の1語だけ（末尾の改行1つを除き、他の文字を出さない） |
| 検査する | 0 | `CHECK` の1語だけ（同上） |
| 失敗（P-2 の review-trail の 2: チェッカ自身のパスが実在しない） | 1 | 何も出さない |
| 判定できない（上の表の「読めないとき」） | 2 | 何も出さない |

- 理由の文（SKIP・失敗・判定できないの理由）は標準エラー出力へ出す。標準出力へ混ぜない。除外軸で SKIP するときの理由の文は AC-7-1 に従う
- 「失敗」と「判定できない」の違い: **失敗**は、入力を読めたうえで P-2 の判定が「失敗」に至った結果である（リポジトリ側のパスがずれた＝除外軸の劣化）。**判定できない**は、スクリプトへの入力そのものが欠けて判定に至らなかったことである。どちらもワークフローを失敗させるが（6-6）、fixture がどちらの分岐を通ったかを見分けられるよう、終了コードを分ける

**判定の順序**（P-2 の順序に、入力を読む時点を加えたもの）:

1. 検査の名前と `PR_LABELS` を読む。読めなければ判定できない
2. ラベル集合に `no-spec` があれば SKIP する。このとき変更ファイルの一覧を読まない（`CHANGED_FILE_LIST` が読めなくても SKIP する）
3. 検査の名前が `review-trail` のときだけ、2つのパスの実在を確かめ、どちらかが無ければ失敗する。この確認は変更ファイルの一覧を読む前に行う
4. `CHANGED_FILE_LIST` を読む。読めなければ判定できない
5. 検査の名前が `review-trail` のときだけ、P-2 の review-trail の 3 の正規表現に一致する行があれば検査する
6. 除外軸に一致すれば検査する。一致しなければ SKIP する

#### 6-7 の期待値

特記の無い列は次のとおりとする: `PR_LABELS` は空文字、`CHANGED_FILE_LIST` は `infra/terraform/main.tf` の1行（末尾に改行あり）を書いたファイル、カレントディレクトリにはチェッカ自身の2つのパスがどちらもある。

| # | 検査の名前（引数） | `PR_LABELS` | `CHANGED_FILE_LIST` | 2つのパス | 期待値 |
|---|---|---|---|---|---|
| 6-7-1 | （引数なし） | | | | 判定できない |
| 6-7-2 | `ci` | | | | 判定できない |
| 6-7-3 | `Spec-link` | | | | 判定できない（大文字と小文字を区別する） |
| 6-7-4 | `spec-link` と `review-trail` の2つ | | | | 判定できない |
| 6-7-5 | `review-trail` | 未設定 | | | 判定できない |
| 6-7-6 | `spec-link` | 未設定 | | | 判定できない |
| 6-7-7 | `review-trail` | | 未設定 | | 判定できない |
| 6-7-8 | `spec-link` | | 空文字 | | 判定できない |
| 6-7-9 | `spec-link` | | 実在しないパス | | 判定できない |
| 6-7-10 | `review-trail` | | ディレクトリのパス | | 判定できない |
| 6-7-11 | `review-trail` | | 空のファイル | | SKIP（空の一覧は読めた入力であり、一致しない。AC-1 の 1-15） |
| 6-7-12 | `spec-link` | | 空のファイル | | SKIP（同上） |
| 6-7-13 | `review-trail` | `no-spec` | 実在しないパス | | SKIP（判定の順序の 2） |
| 6-7-14 | `review-trail` | | 実在しないパス | `.github/scripts/check-review-trail.sh` が無い | 失敗する（判定の順序の 3 が 4 より先） |
| 6-7-15 | `spec-link` | | `infra/terraform/main.tf` の1行（末尾に改行なし） | | 検査する |
| 6-7-16 | `spec-link` | | | 2つとも無い | 検査する（spec-link は実在を確かめない。P-2） |

- 「判定できない」は終了コード 2 で標準出力が空、「失敗する」は終了コード 1 で標準出力が空、「SKIP」「検査する」は終了コード 0 で標準出力がそれぞれ `SKIP` `CHECK` であることを、fixture が終了コードと標準出力の両方で確かめる
- 6-7-7〜6-7-10 と 6-7-11・6-7-12 は対で持つ。前者だけでは、空の一覧まで「判定できない」とする実装が通る。後者だけでは、読めない入力を空の一覧として SKIP する実装（6-6 が禁じる偽の SKIP）が通る

### AC-7. SKIP の出力と docs の記述

| # | 条件 |
|---|---|
| 7-1 | 2つの検査が除外軸で SKIP するとき、出力する理由の文は、範囲として `services/` `apps/` `infra/` の3つを挙げる。`services/` `apps/` の2つだけを挙げる文を残さない。この理由の文は `check-exclusion-axis.sh` が標準エラー出力に出し（AC-6-7）、fixture は AC-1 の 1-5 の一覧を2つの検査の名前で与えたときの標準エラー出力が3つをすべて含むことを確かめる |
| 7-2 | `docs/harness/verification-loop.md`「例外: プロセス検査」と `docs/rules/development-process.md`「Issue と docs の責務分界」が述べる範囲は `services/` `apps/` `infra/` とし、根拠として ADR 0025 を指す |
| 7-3 | spec-link.yml・review-trail.yml の中のコメントが述べる範囲と根拠の ADR も、7-2 にそろえる |

### AC-8. 機械検査の限界

| # | 限界 |
|---|---|
| 8-1 | **判定の入力は変更ファイルの一覧であり、一覧がどう作られるかは検査しない。** 例えば、リネームを `git diff --name-only` がどちらのパスで出すかは git の設定に依る。`infra/` から外へファイルを移すだけの PR が、移した先のパスだけで判定されうる。これは本仕様の前から `services/` `apps/` にもある性質であり、本仕様で変えない |
| 8-2 | **範囲のディレクトリが実在するかは検査しない。** `infra/` を改名・移動すると、除外軸は黙って外れ、その下を変える PR は SKIP に戻る。review-trail の判定（`check-exclusion-axis.sh`）はチェッカ自身の2つのパスの実在を確かめるが、`services/` `apps/` `infra/` の実在は確かめない。これを確かめるかは本仕様の決定に含まれない |
| 8-3 | **spec-link 自身を変える PR は、spec-link の判定を実行しない。** spec-link にはチェッカ自身の変更の分岐が無いため、spec-link.yml だけを変える PR は SKIP する（AC-3-6）。D-2 で新設する `check-exclusion-axis.sh` だけを変える PR も同じく SKIP する（AC-3-7）。本 Issue の PR もこれに当たりうる。spec-link.yml とスクリプトの変更が CI で実際に動いたかは、AC-6 の fixture でしか確かめられない。spec-link に自身の変更の分岐を設けるかは本仕様の範囲の外である（「関連」） |
| 8-4 | **CI の実行そのもの（ラベルの読み取り・`git diff` の実行・`gh api` によるコメント取得）は fixture で再現しない。** これは `docs/harness/verification-loop.md`「例外: プロセス検査」のとおり、検査対象がローカルに無いことによる |
| 8-5 | **`no-spec` を付けてよい変更かどうかは検査しない。** ラベルを付ければ 2つの検査はどちらも SKIP する。`no-spec` の形骸化は ADR 0025「悪い影響」のとおり規律で見る |
| 8-6 | **fixture はワークフローを実行しない。** fixture に入るのは、スクリプトの判定（AC-1 を 6-5 の読み替えで、AC-2 の全行、AC-3 の全行を「スクリプトの出力」列で、AC-4、AC-5 の全行、6-6 の前段、6-7 の期待値の全行、AC-7-1 の理由の文）と、2つの yml がスクリプトを呼び自前の除外軸の判定を持たないという字面（AC-6-4）である。fixture の外にあるのは、(1) PR 本文の検査の OK と NG（AC-3 の 3-1・3-2・3-5 の「期待値」列。3-1 と 3-2 の対が見分けるもの）、(2) yml がスクリプトの結果を SKIP／次の検査へ進む／失敗へ振り分けること（6-6 の後段。スクリプトが実在しないときに失敗することを含む）、(3) yml が PR のラベルを `PR_LABELS` の形へ、`git diff` の出力を `CHANGED_FILE_LIST` のファイルへ正しく渡すこと、(4) 「検査する」の先の往復証跡の取得と `check-review-trail.sh` の実行（AC-2 の注記のとおり本仕様の範囲の外） |
| 8-7 | **yml が 6-3 でスクリプトへ移した分岐を重ねて持つことは、字面では検査しない。** AC-6-4 が字面で確かめるのは除外軸の判定だけである。yml が `no-spec` の判定・パスの実在の確認・チェッカ自身の変更の分岐を `run:` に重ねて書いても、fixture は落ちない。重ねて書いた分岐がスクリプトと食い違ったときの振る舞いは、レビューで見る |

---

## 決定（D）— 2026-11-18、人間の明示選択

**以下は 2026-11-18 に人間が main の AskUserQuestion で明示選択したものである（オーケストレーター経由で受領）。** 選択肢は仕様工程の起案であり、採否を決めたのは人間である。「反映先」の AC の条文は仕様工程が起草したもので、人間の決定として扱わない。**本表に無い事項を人間の決定として扱わない。**

| 論点 | 採った案（人間の明示選択） | 採らなかった案 | 反映先 |
|---|---|---|---|
| **D-1（Q-1）. `infra/` だけを変える PR で往復を回さない場合の扱い**（ADR 0025 決定6 により、`infra/` を変える PR は review-trail の検査を受ける。往復を回さずに済ませたい `infra/` の変更をどの手段で示すか） | **(a) `no-spec` で足りる。新しい手段（専用のラベル等）を設けない** | (b) review-trail だけを SKIP させるラベルを新設する／(c) 手段を設けず、`infra/` を含む PR では `no-spec` があっても review-trail を SKIP しない | P-3（変えない）・AC-5-1・5-2（欠番）・ADR 0025「本 ADR が決めないこと」 |
| **D-2（Q-2）. 除外軸の判定をどこで検査するか**（P-1 のとおり、判定はワークフローの `run:` に直接あり、どの fixture も試験していない） | **(a) 除外軸の判定を `.github/scripts/` の1本のスクリプトへ切り出し、2つのワークフロー（spec-link / review-trail）から呼ぶ。fixture は `make test-scripts` から回す** | (b) 判定をワークフローの `run:` に残し、fixture が2つの yml から除外軸の正規表現を読み出す／(c) 機械では検査せず、レビューで確かめる | 「対象」表・AC-2-9・AC-3-7・AC-4 の注記・AC-6・AC-8-3・8-6 |
| **D-3. ADR 0004 を廃止にする時点** | **人間が ADR 0025 を承認したとき、同じ PR の中で、0025 を「承認済み」に、0004 を「廃止（0025 により置換）」にする。** 承認までは、0025 は提案中、0004 は承認済みのままとする | —— | ADR 0025・ADR 0004 のステータス（AI は変えない） |
| **D-4. 範囲外の観察**（`.devcontainer/` `Makefile` `.claude/` を除外軸に入れるか／spec-link にチェッカ自身の変更で SKIP しない分岐が無いこと） | **#174 の範囲外とする。** 本仕様の要求に入れない | —— | 「関連」・AC-8-3 |
| **D-5（Q-3）. 切り出すスクリプトが持つ判定の範囲**（D-2 の字面は「除外軸の判定だけを持つスクリプト」だが、AC-2・AC-3・AC-5 の期待値は `no-spec` の判定・チェッカ自身の変更の分岐・パスの実在の確認・PR 本文の検査の順序にも依る） | **(ii) `check-exclusion-axis.sh` は各検査の「SKIP／検査する／失敗」の判定までを持つ。入力は検査の名前（spec-link / review-trail）・ラベル集合・変更ファイルの一覧。PR 本文の検査（OK／NG）と往復証跡の検査はワークフロー（と `check-review-trail.sh`）に残す。P-2 の判定の順序とパスの実在の確認はスクリプトへ移す** | (i) 除外軸の一致だけを持つ（ラベル・順序・実在の確認・本文の検査はワークフローの `run:` に残す）／(iii) (i) に加え、ラベルと順序の分岐は `run:` に残し、fixture が yml の字面を読んで確かめる | 「対象」表・AC-2〜AC-5 の「fixture での与え方」と読み替え・AC-6-3〜6-7・AC-7-1・AC-8-2・8-6・8-7 |
| **D-6. AC-6-6 を要求として残すか**（判定できなかったことを SKIP と同じ結果で返さず、そのときワークフローは失敗する。仕様工程が `docs/harness/verification-loop.md` から導いて起草した条文で、それまでの人間の回答に含まれていなかった） | **(a) 要求として残す** | 受領した内容に含まれていない | AC-6-6・6-7 |

- スクリプトのパス名（`check-exclusion-axis.sh`・`test-check-exclusion-axis.sh`）は、Q-2 = (a) を満たすために仕様工程が既存の `check-*.sh` / `test-check-*.sh` の命名に合わせて定めたものであり、人間の決定ではない
- AC-6-7 の入出力の形（引数・`PR_LABELS`・`CHANGED_FILE_LIST`・`SKIP` `CHECK` の字句・終了コード 0／1／2 の割り当て・カレントディレクトリからの相対パスでの実在の確認・判定の順序の中で入力を読む時点）は、Q-3 = (ii) を満たすために仕様工程が定めたものであり、人間の決定ではない
- Q-3 = (ii) の字面は「順序と実在の確認をスクリプトへ移す」である。移した分岐を yml に重ねて書かないこと（AC-6-4）は仕様工程がこの字面から読んだもので、字面による機械検査は除外軸の判定に留める（AC-8-7）
- Q-1 = (a) により、「`infra/` だけを変える PR で往復を回さない場合の扱い」は未決ではなくなった。往復を回さない `infra/` の変更には `no-spec` を付け、そのとき spec-link も SKIP する（AC-5-1・5-3）

---

## 関連

- `docs/adr/0025-reference-model-with-infra-in-exclusion-axis.md`: 決定の正解（ADR 0004 を置き換える。提案中）
- `docs/adr/0004-issue-docs-reference-model.md`: 置き換えられる ADR
- `docs/harness/verification-loop.md`「例外: プロセス検査」: 2つのプロセス検査の位置づけと、除外軸が偽の Green になりうること
- `docs/rules/development-process.md`「Issue と docs の責務分界」
- discussion #167: 論点と人間の選択の出所（Issue のコメントは docs より優先されない — ADR 0025 決定1）
- **範囲外**（D 節。#174 では扱わない）: `.devcontainer/` `Makefile` `.claude/` を除外軸に入れるか／spec-link にチェッカ自身の変更で SKIP しない分岐を設けるか
