# apps/web のスキャフォールドとハーネスの SKIP 解消

Issue #9 の仕様。**`apps/web` を最小構成で立ち上げ、`make lint-web` / `make test-web` が「SKIP」ではなく実際に走る状態にする。**

**画面は作らない。** 画面/UI の受け入れ条件は `docs/specs/work-month-screen-ui.md` ほかに既にあり、本仕様はそれらの実装を含まない。本仕様が定めるのは**足場（scaffold）とハーネスの有効化**の受け入れ条件である。

## 人間が確定した決定（2026-08-14）

**以下3件は人間の決定であり、AI の仮置きではない。実装・レビュー工程で再検討しない。** 詳細は各項へ。

| 論点 | 決定 | 本文 |
|---|---|---|
| レート制限のパラメータ | **案1** — `src/lib/rate-limit.ts` を本 Issue から**外す**。a〜e は最初の Route Handler を作る Issue でセットで決める | U-1 / AC-8-5 / AC-9 |
| コンポーネントの配置先 | **承認** — `apps/web/src/components/`。`ui_kits/` は採らない。本 Issue では位置を決めるだけでコンポーネントは作らない | AC-1-5 |
| 対象が無い場合の扱い | **承認** — `test-web` / `lint-web` は対象が無ければ SKIP ではなく**失敗**。`lint-tf`（infra）の SKIP は据え置く | AC-4-2 / AC-4-5 |

## なぜ仕様を起こすか

Issue #9 の本文は「この Issue は `docs/specs/` に対応する仕様を持たない」と述べるが、**その記述は Issue 側の申告であって、仕様の要否の決定ではない**（ADR 0004: 正解は常に `docs/`）。本仕様を起こすのは次の理由による。

1. **`apps/` を変更する。** SDD は「`docs/specs/` に仕様書がない機能は実装しない」と定め、例外は PR の `no-spec` ラベル（`docs/rules/development-process.md`）。本 Issue は `apps/web` の新設に加え、**Makefile・CI・`docs/harness/verification-loop.md`・README を同時に書き換える**。受け入れ条件が無いと、何をもって完了かをテストに落とせない
2. **ハーネス側の仕様には前例がある。** `docs/specs/skills.md` / `orchestrator-entry-hook.md` / `issue-command.md` / `devcontainer-go-module-policy.md` / `security-rules.md` / `notation-rules.md` はいずれも画面ではなくハーネス・インフラの仕様である。SDD は画面に限定されていない
3. **ADR 0007 §5 が「設定は #9 の範囲とする」と明示的に委譲している**（Vitest）。委譲された決定の置き場所が要る
4. **SKIP を外すことはハーネスの検査対象を変える操作である。** 「対象が無いとき SKIP するのか失敗するのか」は verification-loop の中核の論点であり、記録なしに変えない

---

## スコープ

- `apps/web` の最小スキャフォールド（Next.js App Router + TypeScript + Tailwind CSS）
- `package.json` の scripts 契約（Makefile が呼ぶもの）
- テスト基盤 Vitest の導入（ADR 0007 §5 が本 Issue へ委譲したもの）
- ハーネスの SKIP 解消（Makefile / `ci.yml` / `docs/harness/verification-loop.md` / README）
- **レート制限（U-1）の扱いの固定** — 2026-08-14 に人間が**案1**（本 Issue から外す）を決定。本 Issue が持つのは「外した」ことと「次にどこで決めるか」の記録だけであり、レート制限の実装は含まない

### 非スコープ

- **`src/lib/lambda-client.ts`（SigV4 署名）** — 呼び出す Lambda が存在してから（ADR 0003 / 0014）
- **エンドユーザー認証の実装**（Cognito トークンの BFF 終端・検証）— ADR 0016、認証基盤の Terraform の後
- **画面・コンポーネントの実装** — `docs/specs/*-ui.md` は揃っているが本 Issue では着手しない
- **OpenNext の導入・ビルド / デプロイ設定** — ADR 0013、Terraform の Issue
- **Claude Design / DesignSync の同期運用** — ADR 0015。本仕様が決めるのは配置先だけ（AC-1-5）
- **`src/lib/rate-limit.ts`（Route Handler のレート制限）** — 2026-08-14 の決定（U-1 / 案1）により本 Issue から外した。**スタブ・TODO・既定値の仮置きも置かない**（AC-8-5）
- **Route Handler（`src/app/api/**`）の実装** — レート制限のパラメータ（U-1 の a〜e）が未決であり、最初の Route Handler を作る Issue でセットで決める（AC-9）
- **Node.js の版の変更** — 24 に固定済み（P-2）
- **`.devcontainer/` の変更**（allowlist / Dockerfile）— `docs/specs/devcontainer-go-module-policy.md` の非スコープであり、必要になったら人間の決定が要る（P-3）

---

## 前提（P）

### P-1. ハーネスの現況（実測 2026-08-14）

| 場所 | 現況 |
|---|---|
| `Makefile` `test-web` / `lint-web` | `apps/web/package.json` が無ければ `SKIP: ...（Next.js 未スキャフォールド）` を出して素通りする |
| `Makefile` `lint-web` の実体 | `cd apps/web && npm run lint && npx tsc --noEmit` |
| `Makefile` `test-web` の実体 | `cd apps/web && npm test` |
| `.github/workflows/ci.yml` `web` ジョブ | `name: Web (lint / test)`。`apps/web/package.json` があれば `npm ci`、無ければ SKIP を echo する分岐を持つ |
| `docs/harness/verification-loop.md` | 「未スキャフォールドのレイヤーは『SKIP』を明示する」節が `apps/web` と `infra/terraform` を挙げ、`test-web` の SKIP 出力を例示している |
| `README.md` | 「Web（`apps/web`）| 未スキャフォールド」（現況表）、「Next.js / TypeScript | 未定」（版の表）、SKIP の例示に `test-web` を使用 |

**`lint-web` / `test-web` が呼ぶコマンドは Makefile 側が既に確定している。** 本仕様はこの呼び出しを変えず、`apps/web` 側をそれに合わせる（AC-2）。

### P-2. Node.js の版は 24 で固定済み

`.devcontainer/Dockerfile`（`FROM node:24`）と `.github/workflows/ci.yml`（`node-version: 24`）の2箇所で固定され、README の「バージョンと固定箇所」表に載っている。**片方だけ上げると「ローカルで通るが CI で落ちる」が起きる**（`docs/rules/commands.md`）。本仕様は版を変えない。

### P-3. devcontainer 内で npm の依存取得が成立するかは未実測

`.devcontainer/init-firewall.sh` は `registry.npmjs.org` を allowlist に持つが、**追加されるのは起動時に `dig` が返した A レコードだけ**である。この方式は Go モジュールで実際に破綻しており（`docs/specs/devcontainer-go-module-policy.md` P-2）、同仕様 AC-10-6 は「npm 等は対象外。スキャフォールド時に同種の問題が再発しうる」と明記している。

- **npm の取得が devcontainer 内で成立するかは、本仕様の時点で未実測である**（Go と違い `registry.npmjs.org` は複数 A レコードを返しうるため、同じ結論になるとは限らない）
- **成立しなかった場合、allowlist や `.devcontainer/Dockerfile` を変更して解決してはならない。** それは `devcontainer-go-module-policy.md` の非スコープであり、**人間の決定**が要る（同 AC-2）。実装工程は停止して人間へ上げる

### P-4. `.gitignore` は Node/Next.js 節を既に持つ

`node_modules/` `.next/` `out/` `build/` `dist/` `*.tsbuildinfo` `next-env.d.ts` `coverage/` などが登録済み。**`package-lock.json` は無視されていない**（コミットできる）。**`next-env.d.ts` は無視されている**ため、clone 直後の作業ツリーには存在しない（AC-6-2 の前提）。

### P-5. ADR 0007 の Go 向け制約は Web に自動適用されない

ADR 0007 の §1〜§4（testify を採らない / go-cmp のみ許す / モックライブラリを入れない / `check-domain-deps` の強化）は **Go のテストと `services/api/internal/domain` を対象**にした決定である。Web に及ぶのは §5「`apps/web` のテストは Vitest とする。導入はスキャフォールド（#9）の時点で行う。本 ADR は選定のみを決め、設定は #9 の範囲とする」のみ。

**`make check-domain-deps` は Web を検査しない。** Web 側の依存規律に相当する機械検査は存在せず、本仕様でも作らない（AC-10-2 に限界として固定する）。

### P-6. デプロイ形態は OpenNext（ADR 0013）

本 Issue はアダプタを導入しないが、**OpenNext と非互換になる構成を採らない**。具体的には静的エクスポート（`output: 'export'`）を採らない。Route Handler / SSR がサーバー実行を要することは ADR 0013 が「素の S3 + CloudFront」を却下した理由そのものである。

---

## 受け入れ条件（AC）

| AC | 主題 | 主に効く工程 |
|---|---|---|
| AC-1 | スキャフォールドの構成と配置 | implementer・reviewer |
| AC-2 | `package.json` の scripts 契約（Makefile が呼ぶ） | tester・implementer |
| AC-3 | テスト基盤は Vitest。0件で緑にしない | tester・implementer |
| AC-4 | Makefile — SKIP を外し、対象が無ければ失敗させる | tester・implementer |
| AC-5 | CI の `web` ジョブが実際に検査していること | implementer・reviewer |
| AC-6 | clone 直後から決定的に走ること | tester・implementer |
| AC-7 | docs / README の同期（実装コミットと同時） | implementer・reviewer |
| AC-8 | セキュリティ・コスト・依存の最小化 | implementer・reviewer |
| AC-9 | 本 Issue で実装しないものの固定 | reviewer |
| AC-10 | 限界 — 緑が意味しないこと | reviewer |
| U-1 | **決定済み（2026-08-14 / 案1）**: レート制限は本 Issue から外す。パラメータ a〜e は次 Issue の入力として残す | 次 Issue の specifier |

### AC-1. スキャフォールドの構成と配置

| # | 要求 |
|---|---|
| 1-1 | アプリの実体を **`apps/web`** に置く（Makefile の `WEB_DIR` と一致させる）。**リポジトリ直下に `package.json` / npm workspaces を作らない。** Makefile は `cd apps/web && npm ...` を呼ぶ（P-1）ため、ルートに寄せるとハーネスの前提が変わる |
| 1-2 | **App Router**（`apps/web/src/app/`）を使う。Pages Router（`pages/`）を併設しない。ADR 0013 / 0016 / 各 UI 仕様が App Router と Route Handler を前提にしている |
| 1-3 | **TypeScript**。`tsconfig.json` の `strict` を `true` にする。`npx tsc --noEmit`（`lint-web` が呼ぶ）で型エラーが赤くなること |
| 1-4 | **Tailwind CSS** を導入し、少なくとも1つのページ（`/`）で実際に適用されていること。ページの中身は最小でよい（画面仕様の実装を始めない。AC-9） |
| 1-5 | **ローカル成果物（コンポーネント）の配置先は `apps/web/src/components/`。2026-08-14 に人間が承認した決定であり、AI の仮置きではない。** ADR 0015 が「`apps/web` スキャフォールド時に別途確定する」として委譲した先が本仕様のこの項であり、**これをもって当該委譲は解消する**（以後 ADR 0015 の「未確定」は本項を指す）。リポジトリ直下の `ui_kits/` は**採らない**（Next の同一デプロイに収める。ADR 0013。`docs/rules/architecture.md` が名指しする `apps/web/src/lib/` と同じ階層規則に揃える）。**本 Issue で決めるのはディレクトリの位置だけで、コンポーネントは作らない**（AC-9） |
| 1-6 | **`apps/web/src/lib/` を用意する。** `lambda-client.ts` の置き場所として `docs/rules/architecture.md` と ADR 0003 / 0014 がパスを名指ししている。**`lambda-client.ts` 自体は本 Issue で作らない**（AC-9） |
| 1-7 | **`apps/web/.gitignore` を新設しない。** ルートの `.gitignore` が Node/Next 節を持つ（P-4）。スキャフォールドツールが生成した場合は削除する（二重管理を作らない） |
| 1-8 | スキャフォールドツールが生成する定型ファイルのうち、**内容が事実に反するものを残さない。** とくに生成 README の「Deploy on Vercel」導線は ADR 0013（Vercel から離脱）と矛盾するため削除するか、AWS ネイティブである旨に置き換える |
| 1-9 | `next.config` で **`output: 'export'`（静的エクスポート）を採らない**（P-6） |

### AC-2. `package.json` の scripts 契約

**Makefile 側のコマンドを変えずに通ること**が要件である（P-1）。

| # | 要求 |
|---|---|
| 2-1 | `apps/web/package.json` の `scripts` に **`lint`** と **`test`** が存在する。`make lint-web` は `npm run lint && npx tsc --noEmit` を、`make test-web` は `npm test` を呼ぶ |
| 2-2 | いずれも **watch せず、非対話で終了する。** 成功で終了コード 0、失敗で非 0 を返す。Vitest の既定（`vitest`）は watch モードで CI がハングするため **`vitest run`** を使う |
| 2-3 | **`package-lock.json` をコミットする。** CI の `web` ジョブは `npm ci` を使う（P-1）ため、lockfile が無いとジョブが落ちる |
| 2-4 | scripts 名を変える場合は **`Makefile` と `.github/workflows/ci.yml` を同時に直す。** 片方だけ変えない（ハーネスの前提が壊れる） |
| 2-5 | `lint` の実体（ESLint 等）は実装工程が選んでよいが、**`--fix` を既定にしない**（検査が自動修正で緑になると、何が違反だったか記録に残らない） |

### AC-3. テスト基盤は Vitest

| # | 要求 |
|---|---|
| 3-1 | テストランナーは **Vitest**（ADR 0007 §5）。Jest を入れない |
| 3-2 | **テストが0件のとき緑にしない。** `passWithNoTests` を有効にしない。0件で成功する設定は、`check-domain-deps` の「検査対象が無い場合は失敗させる」と同じ理由で偽の Green にあたる |
| 3-3 | **意味のあるテストを最低1件置く。** `expect(true).toBe(true)` のような、対象が壊れても落ちないテストを置かない（`docs/harness/verification-loop.md`「Red を踏んでいないテストは、対象が壊れても落ちない可能性を排除できていない」） |
| 3-4 | そのテストが **Red を踏めること**を tester 工程が確認する。実装（またはテスト対象の値）を壊すと落ちることを実測する |
| 3-5 | **アサーション / モックのライブラリを追加しない。** Vitest 標準の `expect` を使う。ADR 0007 の Go 向け制約は Web に自動適用されない（P-5）が、**「入れる」判断も本仕様の範囲外**であり、必要になった Issue で判断する。本 Issue はコンポーネントを作らないため `@testing-library/*` も要らない |
| 3-6 | テスト設定（`vitest.config.ts` 等）は `apps/web` 配下に置く。リポジトリ直下へ出さない |

### AC-4. Makefile — SKIP を外し、対象が無ければ失敗させる

| # | 要求 |
|---|---|
| 4-1 | `test-web` / `lint-web` から **`apps/web/package.json` が無ければ SKIP する分岐を削除する。** SKIP 分岐を残すと、`apps/web` を消す・壊す変更が緑のまま通る |
| 4-2 | **対象（`apps/web/package.json`）が無い場合は成功ではなく失敗を返す。2026-08-14 に人間が承認した決定。** `check-domain-deps` の「検査対象が無い場合は失敗させる」と同じ扱い（`docs/harness/verification-loop.md`）。スキャフォールド済みである以上、`package.json` の不在は「未着手」ではなく「壊れている」を意味する |
| 4-3 | 失敗時は**黙って落ちない**。何が無いのか（`apps/web/package.json`）と、それが失敗である理由をログに出す |
| 4-4 | **`make lint-web` / `make test-web` の出力に `SKIP` の語が現れないこと。** これが Issue の完了条件「SKIP ではなく実際に走る」の機械的な確認手段である |
| 4-5 | **`lint-tf`（`infra/terraform`）の SKIP は据え置く（2026-08-14 の決定に含む）。** infra は未着手であり、本 Issue の対象ではない。**web の扱い（4-2）を infra へ波及させない** |
| 4-6 | **web 向けの新しい make ターゲットを増やさない。** `verify` / `test` / `lint` の依存関係を変えない（`docs/rules/commands.md` の表を増やさない＝毎回読まれるトークンを増やさない） |

### AC-5. CI の `web` ジョブが実際に検査していること

| # | 要求 |
|---|---|
| 5-1 | `.github/workflows/ci.yml` の `web` ジョブの依存インストール step から **SKIP の else 分岐を削除**し、`cd apps/web && npm ci` を無条件に実行する。`package.json` が無ければジョブが落ちる（AC-4-2 と同じ理由） |
| 5-2 | **ジョブの `name:`（`Web (lint / test)`）を変えない。新規ジョブを起こさない。** `name:` の変更は ruleset `protect-main` の必須チェックを静かに外す（`docs/harness/verification-loop.md`「必須チェックへの登録」）。人間のブラウザ操作を発生させない |
| 5-3 | **`node-version: 24` を変えない**（P-2）。変えるなら `.devcontainer/Dockerfile` と同時 |
| 5-4 | `actions/setup-node` の npm キャッシュを足すかは任意。足す場合は `cache-dependency-path` を `apps/web/package-lock.json` に指定し、**キャッシュの有無で結果が変わらない**こと |
| 5-5 | **CI 側にだけ存在する検査を作らない。** `web` ジョブが呼ぶのは `make lint-web` / `make test-web` のみに保つ |

### AC-6. clone 直後から決定的に走ること

| # | 要求 |
|---|---|
| 6-1 | **生成物が一切無い作業ツリー**（`git clone` 直後相当）で `npm ci` を行っただけの状態から、`make lint-web` と `make test-web` が成功すること。**`next build` / `next dev` を先に走らせる必要があってはならない**（CI はそれらを実行しない） |
| 6-2 | とくに **`next-env.d.ts` は `.gitignore` 済みで clone 直後に存在しない**（P-4）。`npx tsc --noEmit` がその不在で落ちない構成にすること。手段は実装工程が選んでよい（例: `tsconfig.json` の `types` / `include` の調整、生成物由来の型に依存しない）。**既定では `.gitignore` から `next-env.d.ts` を外して生成物をコミットする案を採らない。** 採るなら本仕様を更新して理由を残す |
| 6-3 | `npm ci` 以降、**lint / test がネットワークアクセスを要求しない**こと。テストから外部へ通信しない（devcontainer の allowlist に依存する検査を作らない） |
| 6-4 | **決定的であること。** 現在時刻・タイムゾーン・ロケール・乱数に依存して結果が変わるテストを置かない（`docs/harness/verification-loop.md` のハーネスの性質1） |
| 6-5 | `make verify` が通ること（`lint` + `test` + `check-domain-deps` + `check-skills` + `check-go-module-pins` + `scan-secrets`） |

### AC-7. docs / README の同期 — 実装コミットと同時に行う

**ターゲットや構成が変わっていない時点で docs を書き換えると docs が虚偽になる**（`docs/specs/devcontainer-go-module-policy.md` AC-9-4 と同じ扱い）。

| # | 要求 |
|---|---|
| 7-1 | `docs/harness/verification-loop.md`「未スキャフォールドのレイヤーは『SKIP』を明示する」節から **`apps/web` を外す**（`infra/terraform` は残す）。例示している SKIP 出力（`==> test-web`）を、実際に SKIP が残る側（`lint-tf`）の例へ差し替える |
| 7-2 | 同 doc に、**web は対象が無ければ失敗する**（AC-4-2）ことを書く。「対象が無い場合に失敗させるのはドメイン層の依存検査だけ」という現行の記述が、実態と食い違ったまま残らないようにする |
| 7-3 | README の次の3点を実態へ合わせる: (a) 現況表「Web（`apps/web`）| 未スキャフォールド（ハーネスは `SKIP` を明示）」、(b) 「バージョンと固定箇所」表の「Next.js / TypeScript | 未定」を実際に入った版と固定箇所（`apps/web/package.json` / `package-lock.json`）へ、(c) SKIP を例示している箇所（`test-web` を使っている） |
| 7-4 | **同期を口実に未完了を隠さない**（`docs/specs/readme-adr-sync.md` AC-3-4）。画面・BFF（`lambda-client.ts`）・Cognito 認証・デモURL・インフラは未完了のままであり、その表現を残す。**「Web が動く」と読める記述にしない** |
| 7-5 | **`docs/rules/` を変更しない。** 本 Issue はコマンドもガードレールも増やさない（毎回読まれるトークンを増やさない。ADR 0010 §B） |
| 7-6 | **本仕様が構成の決定の正解の置き場所である。** 決定の理由を Issue コメント・PR 本文・コミットメッセージへ書き写さない（ADR 0004）。PR 本文には本仕様へのリンクを置く（CI の `spec-link`） |

### AC-8. セキュリティ・コスト・依存の最小化

| # | 要求 |
|---|---|
| 8-1 | **`.env` 系ファイルを追加しない。** 必要になったら `.env.example`（中身は必ずダミー値）のみをコミットし、**コミット前に** `.gitignore` を確認する（`docs/rules/security.md`） |
| 8-2 | 生成・作成するファイルに、実在の URL・AWS アカウント ID・接続文字列・鍵を含めない |
| 8-3 | **`make scan-secrets` が `apps/web/node_modules` の存在する作業ツリーで通ること。** 通らない、または実用的な時間で終わらない場合、**gitleaks の除外設定を黙って足さずに停止して人間へ上げる**。検査の弱体化と改善は AI 自身から区別できない（ADR 0010 §A） |
| 8-4 | **依存を最小限にする。** AC-1〜AC-6 を満たすのに不要なパッケージを入れない。UI ライブラリ・状態管理・HTTP クライアント・認証ライブラリを先回りして入れない |
| 8-5 | **`src/lib/rate-limit.ts` を本 Issue で作らない**（U-1 を 2026-08-14 に案1で決定＝本 Issue から外した）。**素通しの実装・TODO だけの空実装・既定値の仮置きを置かない。** 「あるように見えて効いていないガードレール」は、無いことより悪い。ファイルが存在しないことが正である |

### AC-9. 本 Issue で実装しないもの（対象外の固定）

**以下が無いことは欠落ではなく仕様である。** レビューで「足りない」と指摘しない。

| 対象 | いつ / どこで | 根拠 |
|---|---|---|
| `src/lib/lambda-client.ts`（実行ロールでの SigV4 署名） | 呼び出すドメイン API Lambda が存在してから（Terraform の後） | ADR 0003 / 0014、`docs/rules/architecture.md` |
| Cognito トークンの BFF 終端・検証、ログイン3方式 | 認証基盤の Terraform の後 | ADR 0016、`docs/specs/login-ui.md` |
| 画面・コンポーネントの実装（`src/components/` の中身） | 後続の画面 Issue。本 Issue は置き場所のみ確定（AC-1-5） | `docs/specs/work-month-screen-ui.md` ほか、ADR 0015 |
| **`src/lib/rate-limit.ts`（レート制限）** | **最初の Route Handler を作る Issue。** U-1 を 2026-08-14 に**案1**で決定し、本 Issue から外した。**スタブも置かない**（AC-8-5） | U-1、`docs/rules/cost-guardrails.md`、ADR 0010 §A |
| Route Handler（`src/app/api/**`） | **U-1 が決まってから。** 最初の Route Handler を作る Issue は、レート制限の決定（U-1 の a〜e をセットで）なしに着手できない。**この着手条件は案1の決定後も外れない** | `docs/rules/cost-guardrails.md`、CLAUDE.md（ガードレールを「後で入れる」ことを許容しない） |
| OpenNext の導入・ビルド / デプロイ設定 | Terraform の Issue | ADR 0013 |
| Claude Design / DesignSync の同期運用 | 画面 Issue | ADR 0015（本仕様は配置先のみ確定。AC-1-5） |
| ドメイン API の HTTP 呼び出し実装 | `lambda-client.ts` と同時 | `docs/specs/domain-api-http-contract.md` |

### AC-10. 限界 — 緑が意味しないこと

**以下は仕様どおりの限界であり、欠陥ではない。**

| # | 内容 |
|---|---|
| 10-1 | **「SKIP が出ない」は「意味のある検査をした」を意味しない。** `passWithNoTests` の禁止（AC-3-2）で0件は防げるが、テストの中身が薄いことは防げない。緑はテストが主張した範囲の意味しか持たない |
| 10-2 | **Web には `check-domain-deps` に相当する依存検査が無い。** 「アサーション / モックライブラリを入れない」（AC-3-5）は機械検査されず、規律で守る。ADR 0007 の許可リストは Go の `domain` にしか効かない（P-5） |
| 10-3 | **`npm ci` の再現性は lockfile に依存する。** lockfile を更新した PR で依存が何に入れ替わったかを、この検査は意味的に評価しない |
| 10-4 | **CI と devcontainer で非対称がある。** GitHub runner に egress 制限は無く、`npm ci` は必ず通る。**CI が緑であることは、devcontainer で `npm ci` できることを意味しない**（P-3。`docs/specs/devcontainer-go-module-policy.md` AC-10-9 と同型） |
| 10-5 | **本 Issue の完了は「ハーネスが web を実際に検査している」ことであり、アプリが動く・デプロイできることを意味しない。** OpenNext も Terraform も未着手である |
| 10-6 | **コストガードレールの1枚（Route Handler のレート制限）は本 Issue 完了時点で未実装のままである。これは 2026-08-14 の決定（U-1 / 案1）による意図した状態であり、欠落ではない。** 本 Issue にレート制限の対象（Route Handler）が存在しないことと整合する。「後で入れる」で流さないための担保は、対象を作る Issue の着手条件として U-1 を AC-9 に固定したこと**のみ**であり、**機械検査は無い**（規律で守る） |
| 10-7 | **Tailwind が「適用されている」ことは機械検査しない**（AC-1-4）。ビルドと型検査は通るが、見た目の妥当性はハーネスの外にある |

---

## U-1. レート制限のパラメータ — **決定済み（2026-08-14 / 案1）**

**決定**: 本 Issue #9 から `src/lib/rate-limit.ts` を**外す**。レート制限のパラメータ（下記 a〜e）は、**最初の Route Handler を作る Issue でセットで決める**。#9 はスキャフォールドとハーネスの SKIP 解消に限定する。

| 項目 | 内容 |
|---|---|
| 決定日 | 2026-08-14 |
| 決定者 | 人間（AI の仮置きではない） |
| 採った案 | **案1**（下表） |
| #9 への影響 | **完了条件は変わらない**（`make lint-web` / `make test-web` が SKIP でなく走る・CI の `web` ジョブが実際に検査する・`make verify` が通る）。レート制限は元から完了条件に含まれていない |
| 実装への拘束 | **素通しの実装・TODO スタブ・既定値の仮置きを置かない**（AC-8-5）。「ガードレールがあるように見えて効いていない」状態を作らない |
| 引き継ぎ先 | 最初の Route Handler を作る Issue。着手条件は AC-9 に固定済み（案1の決定後も外れない） |

### なぜ #9 で決めなかったか（記録）

以下は決定前の争点であり、**削除せず残す**。次の Issue が a〜e を決めるための入力である。

| 根拠 | 内容 |
|---|---|
| `docs/adr/0010-harness-engineering.md` §A | 「改変してはならない（人間の承認が要る）」に **検査の閾値**と**コストガードレール**が列挙されている。列挙されていない改変は既定で禁止 |
| `docs/rules/responsibility.md` | 業務ルール・ガードレールは「必ず人間に確認を取る」側。判断に迷ったら実装を進めず**質問して止まる** |
| `docs/ai-collaboration.md`「AIの停止条件」 | 仕様に書かれていない判断が必要になったら停止する。**推測で埋めることを禁止する** |
| `docs/rules/cost-guardrails.md` | 「Route Handler | レート制限」とあるだけで、**上限・ウィンドウ・キー・超過時の応答のいずれも決まっていない** |

### 決めるべき項目（**次の Issue へ持ち越し。a〜e はセットで決める**）

**以下はいずれも未決のままである。** 案1は「#9 で決めない」ことを決めたのであって、a〜e の中身を決めたのではない。**部分的に先取りして実装しない。**

| # | 項目 | なぜ AI が決められないか | 選択肢の例 |
|---|---|---|---|
| a | **キーの取り方** | 誰を単位に絞るかで防御対象が変わる。CloudFront 経由のため `X-Forwarded-For` の左端は詐称可能 | (1) クライアント IP（`CloudFront-Viewer-Address` を信頼する）／(2) セッション・Cognito `sub`（**ゲスト閲覧は未認証なので効かない**）／(3) キー無し（インスタンス単位の総量） |
| b | **上限リクエスト数とウィンドウ** | 数値そのものが「検査の閾値」であり、正規のデモ利用を 429 にする境界を決める | 例: N req / M 秒 / キー。**値は人間が決める** |
| c | **カウンタの置き場所** | Lambda はインスタンスごとにメモリが分かれる。予約同時実行数 5（`docs/rules/cost-guardrails.md`）のため、**インメモリだと実効上限が最大5倍になる** | (1) インメモリ（近似。5倍を受け入れる）／(2) 外部ストア（DynamoDB 等。**新規リソースはコストと使用禁止リストの再検討が要る**） |
| d | **超過時の応答** | 挙動が利用者に見える | 例: 429 + `Retry-After`。ADR 0013 は同時実行超過による 429 を「意図的な挙動でありバグではない」としており、整合を取る必要がある |
| e | **適用範囲** | CloudFront 帯域の一次防御として効かせるなら SSR ページも対象になるが、**画面ロード自体が 429 になる** | (1) Route Handler のみ／(2) SSR ページを含む全リクエスト |

### 提示した選択肢と、採った案（記録）

| 案 | 内容 | 帰結 | 判定 |
|---|---|---|---|
| **案1** | **本 Issue から `src/lib/rate-limit.ts` を外し、最初の Route Handler を作る Issue で a〜e とセットで決める。** 本 Issue はスキャフォールドとハーネスの SKIP 解消に限定する | Issue #9 の「やること」を1つ減らす判断。**Issue の完了条件（SKIP 解消 / CI / `make verify`）はレート制限を含まないため、完了判定は変わらない。** レート制限の対象（Route Handler）が本 Issue に存在しないことと整合する | **採用（2026-08-14）** |
| **案2** | いま a〜e を人間が決め、本仕様に AC-11 として追記してから tester 工程へ渡す | 本 Issue の中で実装できる。ただしレート制限の対象（Route Handler）が無い状態で関数だけが入るため、**実際に効いているかを本 Issue では検証できない**（AC-3-3 の「対象が壊れても落ちないテスト」に近づく） | 採らない |
| **案3** | AI が既定値を仮置きし、後で調整する | ADR 0010 §A に反する。かつ「ガードレールがあるように見えて効いていない」状態を作り、CLAUDE.md の「コストとセキュリティのガードレールは機能要件と同格」に反する | 採らない |

**決定の正解はこの仕様にある。** Issue コメント・PR コメントに書き写して終わらせない（ADR 0004）。**a〜e が決まるまで、tester / implementer は rate-limit に着手しない**（AC-8-5 / AC-9）。

---

## 関連

- `docs/harness/verification-loop.md`: SKIP の扱い／対象ゼロを成功にしない／必須チェックへの登録／ハーネスの性質（AC-4 / AC-5 / AC-6 / AC-7）
- `docs/rules/development-process.md`: SDD と `no-spec` の位置づけ（本仕様を起こす根拠）／Web は Vitest
- `docs/rules/commands.md`: `make verify` の中身、版の固定箇所（P-2 / AC-6-5）
- `docs/rules/architecture.md`: BFF 経由の一方通行、`apps/web/src/lib/lambda-client.ts`（AC-1-6 / AC-9）
- `docs/rules/cost-guardrails.md`: Route Handler のレート制限（U-1。本 Issue から外す決定 = 2026-08-14 / 案1）
- `docs/rules/security.md`: `.env` と機密ファイルの扱い（AC-8-1）
- `docs/adr/0007-testing-with-stdlib-and-go-cmp.md` §5: Vitest の選定と #9 への委譲（P-5 / AC-3）
- `docs/adr/0010-harness-engineering.md` §A: 独断してよい範囲／承認が要る範囲（U-1 / AC-8-3）。§B: `docs/rules/` の肥大化（AC-7-5）
- `docs/adr/0013-aws-native-hosting-over-vercel.md`: OpenNext、CloudFront の従量リスク、429 の扱い（P-6 / AC-1-8 / U-1）
- `docs/adr/0014-execution-role-over-vercel-oidc.md` / `0003`: SigV4 は実行ロール（AC-9）
- `docs/adr/0015-claude-design-for-design-system.md`: 配置先を「スキャフォールド時に確定する」と本仕様へ委譲。**その委譲は AC-1-5（`apps/web/src/components/`。2026-08-14 に人間が承認）で解消済み**
- `docs/adr/0016-cognito-end-user-authentication.md`: 認証は BFF で終端（AC-9）
- `docs/specs/devcontainer-go-module-policy.md`: allowlist の非スコープ、npm への波及（P-3 / AC-10-4）
- `docs/specs/readme-adr-sync.md`: 同期を口実に未完了を隠さない（AC-7-4）
- `docs/specs/work-month-screen-ui.md` ほか UI 仕様: 本 Issue の非スコープ（AC-9）
