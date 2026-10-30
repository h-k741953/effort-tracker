# Web 画面 実装設計 — 仕様

`apps/web` の画面について、**どのルートにどのページを置き、既存のどの部品・lib を使い、どの状態をサーバーとクライアントのどちらが持ち、どの AC をどのテストファイルが担うか**を定める（Issue #55 の完了条件「コンポーネント構成・状態管理の実装設計を `docs/specs/` に残す」）。

**要求（受け入れ条件）はこの仕様へ書き写さない。** 正解は各 UI 仕様（`login-ui.md` ほか）と `design-system.md`・`bff-auth-termination.md` にあり、本仕様は AC 番号で参照するだけである（ADR 0004）。本仕様が決めるのは実装設計に限る。

- **決定者**: 実装設計の各決定は仕様工程（AI 起案。**人間の承認前**）。ただし文言・ログインの入口の行き先・ログイン失敗の担い手・区分の解決が失敗したときの扱い（本仕様の Q-1〜Q-4）と、画面2 の絞り込みの置き場・ハンドオフに無い文言・閲覧系の3画面のパス・「今月の入力」で契約が複数あるときの形（Q-5〜Q-8）、初回入力の部品の置き場（Q-9）、「今月の入力」と初回入力の行き先を年月を含まない経路にすること（Q-10）、テストの差し替えの方針（T-0）は、**2026-10-29 に人間が明示選択で決めた**。今月の入力の経路の置き場と、本人の契約を確かめる場所（Q-11）、画面3 が本人かを知る手段（Q-12）、勤務月の取得が成り立たないときの扱い（Q-13）、操作が弾かれたときの文言のまとめ方（Q-14）、祝日データの用意（Q-15）は、**2026-10-30 に人間が明示選択で決めた**。ただし Q-13・Q-14 の文言そのもの、Q-12 の真偽のキー名（仮の名前）と真偽が応答に無いときの扱いは AI 起案で人間の承認前である。決定は該当の節の本文に、出所「2026-10-29（または 2026-10-30）人間の明示選択（Q-n = …）」を添えて置く。ここでいう Q-n は**本仕様の論点の番号**であり、P-1 が引く `cognito-auth-infra.md` の Q-1 とは別物である
- **日付**: 2026-10-29（画面1 の節を起こし、同日に Q-1〜Q-4 の決定を反映した。同日、画面2 の節＝W 節を足し、同日に Q-5〜Q-8 とテストの差し替えの方針の決定を反映した。Q-8 に伴い `design-system.md` 6-13 を改定した。同日、Q-9・Q-10 の決定を反映し、月の境目に合わせたタイマーで描き直す設計を取り下げた。Q-10 のうち経路の置き場と本人の契約の確かめ方は Q-11 として残した。2026-10-30、Q-11 = (e) の決定を C-2・W-1・W-7・T-2 へ反映し、W-8（画面2 の関数の形と props）を足し、T-0-3 の是正が済んだことを L-1・C-3・T-0・T-1 へ反映した。W 節のうち出所を書いていない決定は AI 起案で人間の承認前である。同日、画面3 の節＝M 節と T-3 を足し、W-8 の `ThisMonthEntry` を実装の形へ改めた。同日、Q-12 = (b)・Q-13 = (a)・Q-14 = (b)・Q-15 = (a) の決定を M 節（M-1〜M-4・M-9〜M-12）と T-3 へ反映し、論点から外した。勤務月の表現に足す真偽は #55 とは別の Issue（#155）が入れるものであり、本仕様はそのキー名を仮の名前として扱う（M-3）。同日、その別の Issue が人間の承認を得て #155 として起票されたことを M-3・M-11・T-3・論点の節へ反映した）
- **前提の仕様**: `login-ui.md`／`design-system.md`（AC-5・AC-6（6-13 は 2026-10-29 改定後）・AC-8・AC-12）／`bff-auth-termination.md`（AC-5-5・AC-5-11・AC-11-2）／`work-month-listing-ui.md`（AC-1〜AC-10。W 節）／`bff-wiring.md` AC-6 の R-2〜R-4・R-10／`domain-api-http-contract.md` AC-3・AC-10-2・AC-12。M 節はこれに加えて `work-month-screen-ui.md`（AC-1〜AC-8）／`daily-record-entry-ui.md`（AC-1〜AC-7・AC-9・AC-10）／`monthly-closing-ui.md`／`approval-ui.md`／`bff-wiring.md` AC-6 の R-1・R-5〜R-9・AC-8 の 8-7／`domain-api-http-contract.md` D-7・D-12・AC-2・AC-4〜AC-9・AC-10-1・AC-10-3

---

## スコープ

- **対象**: 画面1（ログイン画面・デモ用ロール切替・ヘッダー）と、画面1 が最初に要する**画面間で共有する土台**（C 節: ルート表のうち既定の画面、利用者の区分の解決、開ける画面の判定、置き場所の規則）。画面2（本人の勤務月一覧・承認待ち一覧・デモ勤務月群と、ログイン画面を除く各画面に共通のサイドバー。W 節）。画面3（勤務月画面。稼働実績の入力・編集・削除、締め、承認・差戻しの操作と、その出し分け。M 節）
- **非対象**:
  - 受け入れ条件そのもの（各 UI 仕様が持つ）
  - BFF の Route Handler（`/api/auth/*`・業務の経路）とドメイン API。既存のものを使うだけで、本仕様では変えない（`bff-auth-termination.md`・`bff-wiring.md`・`domain-api-http-contract.md`）
  - ヘルプ・契約一覧・稼働の推移の画面。C-4 の判定で画面の識別子として名前が出ることと、C-2 でパスを定めること（サイドバーの項目の行き先として要るため）だけで、ページの設計は持たない
  - ユーザー・ロール管理画面（非スコープ。`docs/rules/scope.md`・`login-ui.md` AC-8-1）
  - サインアウト（要求する仕様が無い。`bff-auth-termination.md` の非対象表）

---

## 前提（P）— 既存の決定と実装（参照のみ）

| # | 前提 | 出典 |
|---|---|---|
| P-1 | 自前のログイン画面は3つの入口の提示に留まり、メール+パスワード・MFA(TOTP)・Google の画面は Cognito のホストされたサインイン画面が持つ（Q-1 = (a)） | `cognito-auth-infra.md` Q-1・11-11 |
| P-2 | サインインの開始は `GET /api/auth/sign-in`、戻りは `GET /api/auth/callback`。戻りが成功すると `/` へ 302 で戻す（`auth-handlers.ts` の `SIGN_IN_LANDING_PATH`） | `bff-auth-termination.md` AC-11-2 (i)(ii) |
| P-3 | ロール切替は `POST /api/auth/role`。要求からロールを読まず、検証した現在のロールを反転させる。成功は 204、未認証は 401（Cookie を破棄）、構成欠落は 5xx、レート制限は 429 | 同 AC-5-5・AC-5-11・AC-11-2 (iii)・AC-11-12 |
| P-4 | テーマは `et_theme` Cookie（`theme-cookie.ts`）。`layout.tsx` が最初の描画から `<html data-theme>` を出す | `design-system.md` AC-12-3・12-6 |
| P-5 | 部品 `AppHeader`・`RoleSwitcher`・`ThemeToggle`・`ErrorBanner` は既存。いずれも保存・通信・ロールの判定をしない | `design-system.md` 6-3・6-7・6-11・6-14 |
| P-6 | `apps/web/src/components/` 直下の `*.tsx` は台帳（`design-system.md` AC-5-1）と過不足なく一致しなければならない。画面固有の合成は台帳に載らない | `design-system.md` AC-5-1〜5-3・`component-inventory.test.ts` |
| P-7 | URL・ルートは UI 仕様で固定されていない | `design-system.md` 8-4・`login-ui.md` 8-6 |
| P-8 | 新しい npm 依存を入れない（状態管理ライブラリを含む） | Issue #55 の前提 |

**デザインのハンドオフとの関係** —— `design/handoff/2026-10-theme-b/` のログイン画面は、自前のメール+パスワードのフォーム・2段階認証の状態・失敗の状態を描いている。これは P-1（Q-1 = (a)）と両立しないため、本設計はフォームを作らず、入口を Cognito のホストされた画面への導線として置く（L-2）。ハンドオフから採るのは入口の文言だけである（L-2。2026-10-29 人間の明示選択（Q-1 = (a)、ただし「・新規登録」を付けない））。2段階認証の状態・失敗の状態はログイン画面に描かない（L-2。2026-10-29 人間の明示選択（Q-3 = (a)））。

---

## C. 画面間で共有する土台

### C-1. 置き場所の規則

| 対象 | 置き場所 | 理由 |
|---|---|---|
| ページ | `apps/web/src/app/` 配下の `page.tsx`（ルートは C-2） | App Router |
| 画面固有の合成（ヘッダーの右側の組み立て・ログインの入口など） | `apps/web/src/app/_components/`（`_` 始まりはルートにならない） | P-6。`src/components/` に置くと台帳と食い違う |
| 純粋な判定・通信の関数 | `apps/web/src/lib/` | 既存の `auth-handlers.ts` 等と同じ。テストから直接呼べる |
| サーバー専用の取得（`next/headers` を読むもの） | `apps/web/src/app/_lib/` | `src/lib/` を Next の実行時に依存させない |

- クライアント部品（`"use client"`）は、`role-cookie.ts`・`viewer.ts`・`auth-config.ts` を**値として**輸入しない（`node:crypto` と署名鍵の読み取りをブラウザへ持ち込まない）。型 `Role` は `import type` に限る（`role-switcher.tsx` と同じ形）
- ページ・`_components` は `lambda-client` を輸入しない（`lambda-client-boundary.test.ts` が検査済み。`login-ui.md` AC-5-1）

### C-2. ルート表（画面1 が定める分）

| 画面 | パス | 置くファイル | 定めた節 |
|---|---|---|---|
| ログイン画面 | `/` | `app/page.tsx` | L-1 |
| 本人の勤務月一覧（技術者の既定の画面。`login-ui.md` 6-7-a） | `/work-months` | 画面2 の節で置く | C-3 の遷移先として本節で決める |
| 承認待ち一覧（承認者の既定の画面） | `/pending-approvals` | 同上 | 同上 |
| デモ勤務月群（ゲストの既定の画面） | `/demo` | 同上 | 同上 |
| 勤務月画面 | `/work-months/[contractId]/[yearMonth]`（`yearMonth` は `YYYY-MM`） | `app/work-months/[contractId]/[yearMonth]/page.tsx`（薄い層。本体は `app/_components/work-month-screen.tsx`。M-1） | W-1（画面2 の行のリンク・直近・今月の入力が行き先として要するため、本節で決める） |
| ヘルプ | `/help` | 本仕様では置かない（ページが入るまで 404） | W-6 のサイドバーの項目の `href` として要るため本節で決める。出所: 2026-10-29 人間の明示選択（Q-7 = (a)） |
| 契約一覧 | `/contracts` | 同上 | 同上 |
| 稼働の推移 | `/work-trends` | 同上 | 同上 |
| 今月の入力の経路（画面ではない。何も表示せず、送り先へ回すだけ） | `/this-month?contract=<契約ID>` | `app/this-month/page.tsx`（`redirect()` するだけのページ。本体は `app/_components/this-month-screen.tsx`） | W-7。経路を置くことは 2026-10-29 人間の明示選択（Q-10）、`redirect()` するだけのページに置くことは 2026-10-30 人間の明示選択（Q-11 = (e)）。**パスとクエリの名前は AI の起案** |

- **勤務月画面のパスを画面3 ではなく本節で決める理由**: 画面2 の4つの導線（行＝`work-month-listing-ui.md` AC-5-1、直近＝AC-6-1、初回入力＝AC-7-1、今月の入力＝AC-10-9）がいずれもこのパスを要し、画面2 のテストの期待値になるため。画面3 の節はページを置くだけで、パスを変えない。形は BFF の R-1 のパス（`/api` を除いたもの）に揃えた。**AI の起案であり、人間の承認前である**（次項と同じ扱い）。`screen-routes.ts` に `workMonthPath(contractId, yearMonth)` として持ち、各セグメントを `encodeURIComponent` で符号化する
- **`/work-months`・`/pending-approvals`・`/demo` のパスの文字列は AI の起案であり、人間の承認前である。** 2026-10-29 の明示選択（Q-1〜Q-8）はこれらのパスと勤務月画面のパスを決めていない（Q-7 が決めたのはヘルプ・契約一覧・稼働の推移の3つだけである）。**レビューで人間が確かめる対象として残す**（パスは UI 仕様で固定されていない＝P-7。変えるときは `screen-routes.ts` の定数と本表を改める）。ログイン画面を `/` に置くことは、P-2 の戻り先からの帰結である（次項）
- **ログイン画面を `/` に置く理由**: サインインの戻りは `/` へ着く（P-2）。`/` をログイン画面にすると、ログイン済みの利用者は 6-10 によりそのロールの既定の画面へ移り、`design-system.md` 8-1-b（ログイン後に本人の勤務月一覧）が `auth-handlers.ts` を変えずに成り立つ。ゲストが `/` を開いたときはログイン画面が出る（6-10-a）
- **ヘルプ・契約一覧・稼働の推移のパス `/help`・`/contracts`・`/work-trends` は、AI の起案ではなく人間の決定である。** 出所: 2026-10-29 人間の明示選択（Q-7 = (a) パスは `/help`・`/contracts`・`/work-trends`。サイドバーには仕様（`work-month-listing-ui.md` 10-2〜10-4）どおり項目を出し、ページが入るまで行き先が 404 になることを受け入れる）。ページは本仕様の対象外であり、各画面の Issue が置く。404 の期間は限界として T-2 に記録する
- **今月の入力の経路のパス `/this-month` とクエリの名前 `contract` は AI の起案であり、人間の承認前である。** 2026-10-29 の明示選択（Q-10）が決めたのは「年月を含まない経路を行き先にし、押された時点でサーバーが JST の当月を計算して、その契約 × 当月の勤務月画面へ 302 で送る」ことであり、2026-10-30 の明示選択（Q-11 = (e)）が決めたのは「`redirect()` するだけのページに置く」ことである。いずれもパスの文字列は決めていない（`/this-month?contract=<契約ID>` は Q-10 を諮った際の例である）。レビューで人間が確かめる対象として残す。`screen-routes.ts` に `thisMonthPath(contractId)` として持ち、契約 ID をクエリの値として符号化する
- パスの文字列は `apps/web/src/lib/screen-routes.ts` に定数として1か所で持ち、ページ・判定・リンクはそれを参照する
- 画面2 が入るまで `/work-months`・`/pending-approvals`・`/demo` はページを持たない（遷移先が 404 になる期間がある）。同じブランチで画面2 が入ることを前提とする

### C-3. 利用者の区分の解決（`apps/web/src/lib/viewer.ts`）

```ts
export type Viewer = { kind: "guest" } | { kind: "signed-in"; role: Role };
export type ResolveViewerResult = Viewer | { kind: "misconfigured" };

export function resolveViewer(params: {
  cookieHeader: string | null;
  now: Date;
  environment?: Environment;
  publicKeyResolver?: PublicKeyResolver;
}): Promise<ResolveViewerResult>;
```

- 判定の順は `work-month-handlers.ts` の `handle()` の順2〜順4 と同じにする —— (1) 認証の構成が欠けていれば `misconfigured`、(2) `resolveVerifiedSession` が成立しなければ `guest`、(3) 成立したのに署名鍵が欠けていれば `misconfigured`、(4) 成立すれば `resolveEffectiveRole` で決めたロールの `signed-in`（`login-ui.md` AC-5-2・`bff-auth-termination.md` AC-5-11）
- ゲストにロールを与えない（`bff-auth-termination.md` 5-11 (v)・`login-ui.md` 6-5）
- **セッション Cookie の検証に失敗したとき**: ページは**ゲストとして描く**（上の順 (2)）。`resolveViewer` もページも Cookie を書き換えない（ページの描画からは Cookie を書けない）。失敗した Cookie の破棄（`bff-auth-termination.md` 5-8 (v)）は、**次に Route Handler を通る要求に任せる**。要求ごとに検証して破棄する `proxy.ts` は**足さない**。出所: 2026-10-29 人間の明示選択（Q-4 (i) = (a)）
- **認証の構成・署名鍵が欠けているとき（`misconfigured`）**: ページは**エラーページにする**。ゲストや既定の画面へ**黙って落とさない**（`bff-auth-termination.md` 7-5 と同じ思想）。実装はページが `misconfigured` を受けたら例外を投げ、Next のエラーの描画に任せる（`decideScreen`・`decideLoginScreen` は `Viewer` だけを受け、`misconfigured` を受けない）。エラーページの文言は本仕様で定めない（決定に無いため。Next の既定の描画のまま）。出所: 2026-10-29 人間の明示選択（Q-4 (ii) = (a)）
- 区分は `app/_lib/current-viewer.ts` の `getCurrentViewer()` で解決する。これは `headers()` の `cookie` と現在時刻で `resolveViewer` を呼ぶだけの薄い層で、React の `cache` で1要求1回にまとめる（ヘッダーとページが別々に解決しない）。画面の本体は `getCurrentViewer()` を直接呼ばず、注入された `deps.getCurrentViewer()` を呼ぶ。本物の依存 `serverScreenDeps`（T-0-2）が `getCurrentViewer` を束ねる（React の `cache` による1要求1回は保たれる）

### C-4. 開ける画面の判定（`apps/web/src/lib/screen-access.ts`）

```ts
export type ScreenId =
  | "own-work-months" | "pending-approvals" | "demo-work-months"
  | "work-month" | "help" | "contracts" | "work-trends";
export type ScreenDecision = { kind: "show" } | { kind: "redirect"; to: string };

export function defaultScreenPath(viewer: Viewer): string;               // 6-7-a
export function decideScreen(screen: ScreenId, viewer: Viewer): ScreenDecision; // 6-7・6-8・6-8-a・6-9・6-9-a
export function decideLoginScreen(viewer: Viewer): ScreenDecision;        // 6-10・6-10-a
```

- `ScreenId` は 6-7 が列挙する画面の識別子である（ログイン画面は 6-7 の外なので含めない）。`help`・`contracts`・`work-trends` は判定の表に現れるだけで、本仕様はそれらのページを設計しない
- 関数は純粋で、Cookie・状態・稼働実績に触れない（6-9-b）
- **どのページも（本体が）、描画の最初に `deps.getCurrentViewer()` と `decideScreen`（ログイン画面は `decideLoginScreen`）を呼び、`redirect` なら `next/navigation` の `redirect()` で移す**。ロールを切り替えたときは L-4 が `router.refresh()` でサーバーの描画をやり直すため、同じ判定が働いて 6-8・6-8-a が成り立つ。URL を直接開いたとき（6-9）も同じ経路である。切替のためだけの別の遷移処理をクライアントに持たない
- 移した後の履歴の扱いは固定しない（`login-ui.md` 8-6）。`redirect()` の既定に従う
- 判定は画面の出し分けであり認可ではない（`login-ui.md` 8-7）。データの取得の可否は BFF とドメイン API が持つ

---

## L. 画面1 — ログイン画面・デモ用ロール切替・ヘッダー

### L-1. 構成

```mermaid
flowchart TD
  Layout["app/layout.tsx（既存。data-theme を出す）"] --> LoginPage["app/page.tsx（薄い層）"]
  LoginPage -->|"serverScreenDeps を渡す"| LoginScreen["_components/login-screen.tsx（本体）"]
  LoginScreen -->|"deps.getCurrentViewer + decideLoginScreen"| Decide{"show / redirect"}
  Decide -->|redirect| Default["既定の画面（C-2）"]
  Decide -->|show| SiteHeader["_components/site-header.tsx（サーバー）"]
  Decide -->|show| Entries["_components/login-entries.tsx（サーバー）"]
  SiteHeader --> AppHeader["components/app-header.tsx（既存）"]
  SiteHeader --> Controls["_components/header-controls.tsx"]
  Controls --> RoleCtl["_components/role-switch-control.tsx（クライアント）"]
  Controls --> ThemeCtl["_components/theme-control.tsx（クライアント）"]
  Controls --> LoginLink["ログインへのリンク"]
  RoleCtl --> RoleSwitcher["components/role-switcher.tsx（既存）"]
  RoleCtl --> ErrorBanner["components/error-banner.tsx（既存）"]
  ThemeCtl --> ThemeToggle["components/theme-toggle.tsx（既存）"]
```

| ファイル | 種別 | 責務 | 主に担う AC |
|---|---|---|---|
| `app/page.tsx` | サーバー（async。薄い層） | `loginScreen(serverScreenDeps)` を返すだけ（T-0-2）。判定も描画の組み立ても持たない | — |
| `app/_components/login-screen.tsx` | サーバー（async 関数。依存を引数で受ける） | `loginScreen(deps: ScreenDeps)`。`deps.getCurrentViewer()` が `misconfigured` なら例外を投げる（C-3。Q-4 (ii) = (a)）。それ以外は `decideLoginScreen` が `redirect` なら移し、`show` なら `SiteHeader`（`screen="login"`）と `LoginEntries` を描く | 6-10・6-10-a・9-1 |
| `app/_components/login-entries.tsx` | サーバー | 3つの入口を出す（L-2） | 1-1・1-2・2-1・2-3・3-1・3-2・4-1・4-3 |
| `app/_components/site-header.tsx` | サーバー（同期。値は props で受ける） | `AppHeader` の `right` に `HeaderControls` を差し込む。props は `viewer: Viewer`・`theme: Theme`・`screen: "login" \| "app"` | 9-1 |
| `app/_components/header-controls.tsx` | サーバー | 右側に何を・どの順に出すかを決めて並べる（L-3） | 6-1・6-5・9-2・9-3・9-4・9-6・9-7 |
| `app/_components/role-switch-control.tsx` | クライアント | ロール切替の通信と画面の更新（L-4） | 6-1・6-2・6-6・6-9-b・7-2・9-5 |
| `app/_components/theme-control.tsx` | クライアント | テーマの保存と `<html>` の属性の付け替え（L-5） | 9-3・9-4・9-5・`design-system.md` 12-3 |
| `lib/role-switch-client.ts` | 純粋（ブラウザで動く） | `POST /api/auth/role` を送り、結果を3つに分ける（L-4） | 6-9-b・7-2 |
| `lib/viewer.ts`・`lib/screen-access.ts`・`lib/screen-routes.ts` | 純粋 | C-3・C-4・C-2 | 5-2・6-5・6-7〜6-10-a |
| `app/_lib/current-viewer.ts`・`app/_lib/screen-deps.ts` | サーバー専用 | C-3 の薄い層と、`ScreenDeps`・`serverScreenDeps`（T-0-2） | — |

`SiteHeader` を async にせず値を props で受けるのは、テストで描画できるようにするためである（async のサーバー部品は Testing Library で直接描けない）。値を取るのは本体の側（`deps.getCurrentViewer()` と `deps.getTheme()`。本物は `getCurrentViewer()` と `cookies()` → `resolveTheme`）。画面2 以降のページ（`screen="app"`）も同じ部品を使う。

### L-2. ログインの入口（`login-entries.tsx`）

| 入口 | 要素 | 文言（アクセシブル名） | 行き先 | 備考 |
|---|---|---|---|---|
| (a) ゲストとして続行 | `next/link` のリンク | 「ゲストとして続行（閲覧のみ）」。リンクに添えて注記「入力・締め・承認はできません。」を出す | `/demo`（C-2） | Route Handler を経由しない・要求を送らない（`bff-auth-termination.md` 11-4・`login-ui.md` 4-1） |
| (b) メール+パスワード | `<a>`（全体の遷移。`next/link` を使わない＝外部へ 302 する経路のため） | 「メールアドレスでログイン」 | `/api/auth/sign-in` | 新規登録はホストされた画面の側が持つ（P-1・2-1）。文言に「・新規登録」を**付けない** |
| (c) Google 連携 | `<a>`（(b) と同じ理由） | 「Google でログイン」 | `/api/auth/sign-in`（(b) と同じ） | ホストされた画面が Google の選択肢を出す。利用者は Google まで2回押す |

- **文言**の出所: 2026-10-29 人間の明示選択（Q-1 = (a) ハンドオフどおり。ただし (b) に「・新規登録」を付けない）。テストはこの文言でアクセシブル名を引く
- **(c) の行き先**の出所: 2026-10-29 人間の明示選択（Q-2 = (a) Google の入口も `/api/auth/sign-in` を通す）。**BFF の仕様と実装は変えない**（`bff-auth-termination.md` AC-11-2 (i)・`cognito-oidc.ts` に `identity_provider` を足さない）。Google まで2回押すことは受け入れた
- **帰結 —— (b) と (c) は行き先で区別できない。** 両者は同じ `href` を持つため、テストは入口を**アクセシブル名で区別する**（「メールアドレスでログイン」と「Google でログイン」がそれぞれちょうど1つあること）。`href` だけを数える・比べる形では、(c) が欠けても (b) が2つあっても見分けられない
- フォーム・`<input type="password">`・資格情報を受ける欄を置かない（P-1・2-3）。クライアント部品を持たない
- **ログイン失敗の表示（7-1）**: 資格情報・MFA の誤りの提示と再試行は、**Cognito のホストされた画面に任せる**。サインインの戻り（`/api/auth/callback`）で失敗したとき（取消・`state` の不一致等）は、BFF が返す今の 401 の素の応答（`bff-auth-termination.md` 3-4 (i)）のままとする。**ログイン画面は失敗を出さない**（`ErrorBanner` も再試行の導線も持たない）。これは T 節の限界に記録する。`bff-auth-termination.md` は変えない。出所: 2026-10-29 人間の明示選択（Q-3 = (a)）
- 通信・サーバーエラー（7-2）のうち、この画面が自分で通信するものは無い（入口はいずれも遷移である）

### L-3. ヘッダーの右側（`header-controls.tsx`）

出すものと並びを、純粋な関数 `headerSlots(viewer, screen)` で決め、その順に描く（並びは 9-7）。

| `viewer` | `screen` | 出すもの（この順） | 根拠 |
|---|---|---|---|
| ゲスト | `login` | テーマ切替 | 9-2・9-3・9-6 |
| ゲスト | `app` | テーマ切替 → ログインへのリンク | 9-3・9-6・9-7 |
| 技術者・承認者 | `login` | テーマ切替 | 9-2・9-6（実際には 6-10 で移るため描かれない） |
| 技術者・承認者 | `app` | ロール切替 → テーマ切替 | 6-1・9-2・9-3・9-7 |

- ログインへのリンクの行き先は `/`（C-2）。文言は「ログイン」（出所: 2026-10-29 人間の明示選択（Q-1 = (a)））
- `headerSlots` は `header-controls.tsx` から輸出し、表をそのままテストの入出力にする

### L-4. ロール切替（`role-switch-control.tsx`・`role-switch-client.ts`）

| 状態 | 持ち主 | 内容 |
|---|---|---|
| 現在のロール | **サーバー**（署名済み Cookie。`resolveViewer` が読む） | props `role` で受け、`RoleSwitcher` の `role` にそのまま渡す。**クライアントでロールを持たない・先回りして書き換えない** |
| 通信中 | クライアント（`useState`） | 通信中は次の `onChange` を無視する。切替は反転なので（P-3）、二重に送ると元へ戻るため |
| 失敗 | クライアント（`useState`） | 失敗したときだけ `ErrorBanner` を出す。理由文は「ロールを切り替えられませんでした。」（出所: 2026-10-29 人間の明示選択（Q-1 = (a)）） |

`requestRoleSwitch(fetchImpl = fetch): Promise<"switched" | "signed-out" | "failed">` は `POST /api/auth/role` を本文なしで送る（ロールを申告しない。`bff-auth-termination.md` 5-5 (ii) と食い違う要求を作らない）。応答の対応は 204 → `switched`、401 → `signed-out`、それ以外の応答・例外 → `failed`。

```mermaid
sequenceDiagram
  participant U as 利用者
  participant C as RoleSwitchControl
  participant B as BFF /api/auth/role
  participant P as ページ（サーバー）
  U->>C: RoleSwitcher の onChange
  C->>B: POST（本文なし）
  alt 204（switched）
    C->>P: router.refresh()
    P->>P: getCurrentViewer + decideScreen（C-4）
    P-->>U: 同じ画面のまま描き直す（6-6・6-8-a）か、既定の画面へ redirect（6-8）
  else 401（signed-out。BFF が Cookie を破棄済み）
    C->>P: router.refresh()
    P-->>U: ゲストとして判定し直す（C-4）
  else それ以外・例外（failed）
    C-->>U: ErrorBanner。ロールの表示は変えない・refresh しない（7-2）
  end
```

- `router.refresh()` 以外の遷移（`push`・`replace`）をクライアントから呼ばない（移すかどうかはサーバーの判定だけが決める。C-4）
- テーマの Cookie に触れない（9-5）

### L-5. テーマ切替（`theme-control.tsx`）

| 状態 | 持ち主 | 内容 |
|---|---|---|
| 保存されたテーマ | **サーバーが読む Cookie**（`et_theme`） | ページが `cookies()` → `resolveTheme` で読み、props `initialTheme` で渡す（`layout.tsx` と同じ値になる） |
| 表示中のテーマ | クライアント（`useState`。初期値 `initialTheme`） | `ThemeToggle` の `onChange` で更新する |

`onChange(theme)` で、(1) `document.cookie` に `serializeThemeCookie(theme)` を書き、(2) `document.documentElement` の `data-theme` を `theme` に付け替え、(3) 表示中のテーマを更新する。再読み込みしない・サーバーへ要求を送らない（`design-system.md` 12-3。部品の `ThemeToggle` は副作用を持たないまま＝6-14-v）。ゲストにも同じ部品を出す（9-4）。

---

## W. 画面2 — 勤務月一覧（本人一覧・承認待ち一覧・デモ勤務月群）とサイドバー

本節で AC 番号だけを書いたものは `work-month-listing-ui.md` の AC である。**本節の決定は、出所「2026-10-29（または 2026-10-30）人間の明示選択（Q-n = …）」を添えたものを除き、AI 起案であり人間の承認前である。**文言のうち出所を「ハンドオフ」と書いたものは `design/handoff/2026-10-theme-b/` の該当ファイルの字面をそのまま採ったもので、本節では作っていない。

### W-1. 構成

```mermaid
flowchart TD
  OwnPage["app/work-months/page.tsx（薄い層）"] -->|"serverScreenDeps を渡す"| OwnScreen["_components/own-work-months-screen.tsx"]
  PendPage["app/pending-approvals/page.tsx（薄い層）"] -->|同上| PendScreen["_components/pending-approvals-screen.tsx"]
  DemoPage["app/demo/page.tsx（薄い層）"] -->|同上| DemoScreen["_components/demo-work-months-screen.tsx"]
  OwnScreen -->|"deps.getCurrentViewer + decideScreen"| Shell
  PendScreen -->|同上| Shell
  DemoScreen -->|同上| Shell
  Shell["_components/app-shell.tsx（サーバー）"] --> SiteHeader["_components/site-header.tsx（既存。screen=app）"]
  Shell --> AppSideNav["_components/app-side-nav.tsx（クライアント）"]
  Shell --> View["一覧の中身（クライアント）"]
  AppSideNav --> SideNav["components/side-nav.tsx（既存）"]
  AppSideNav -->|"技術者のとき GET /api/contracts（R-10）"| BFF["BFF Route Handler（既存）"]
  View --> OwnView["_components/own-work-months-view.tsx"]
  View --> PendView["_components/pending-approvals-view.tsx"]
  View --> DemoView["_components/demo-work-months-view.tsx"]
  OwnView -->|"R-2・R-10"| BFF
  PendView -->|"R-3"| BFF
  DemoView -->|"R-4"| BFF
  OwnView --> Table["_components/work-month-table.tsx"]
  PendView --> Table
  DemoView --> Table
  OwnView --> Filters["_components/work-month-filters.tsx"]
  PendView --> Filters
  OwnView -->|"0件のとき"| Initial["_components/initial-entry-action.tsx"]
  AppSideNav --> Entry["lib/this-month-entry.ts"]
  Initial --> Entry
  Entry -.->|"リンクの行き先（押されたとき）"| ThisMonthPage["app/this-month/page.tsx（薄い層。force-dynamic）"]
  ThisMonthPage -->|"serverThisMonthDeps と searchParams を渡す"| ThisMonthScreen["_components/this-month-screen.tsx（本体。何も描かない）"]
  ThisMonthScreen -->|"技術者・契約 ID あり: redirect（その契約 × JST の当月）"| WorkMonth["勤務月画面（画面3）"]
  ThisMonthScreen -->|"それ以外: redirect（区分の既定の画面）"| DefaultScreen["本人の勤務月一覧／承認待ち一覧／デモ勤務月群"]
```

| ファイル | 種別 | 責務 | 主に担う AC |
|---|---|---|---|
| `app/work-months/page.tsx`・`app/pending-approvals/page.tsx`・`app/demo/page.tsx` | サーバー（async。薄い層） | 対応する画面の本体へ `serverScreenDeps`（T-0-2）と、Next が渡す `searchParams` を待った値を渡し、その戻り値を返すだけ。判定も描画の組み立ても持たない（T-0-2） | — |
| `app/_components/own-work-months-screen.tsx` | サーバー（async 関数。依存を引数で受ける） | `ownWorkMonthsScreen(deps, searchParams)`。`deps.getCurrentViewer()` が `misconfigured` なら例外（C-3）。`decideScreen("own-work-months")` が `redirect` なら移す。`show` なら `AppShell`（選択中＝本人の勤務月一覧）と `OwnWorkMonthsView` を描く。`searchParams` の `contract` が文字列1つのときだけそれを `initialContractId` として渡し、それ以外（無い・配列）は渡さない（W-4） | 2-4・3-3・9-12、`login-ui.md` 6-9 |
| `app/_components/pending-approvals-screen.tsx` | 同上 | `pendingApprovalsScreen(deps)`。同上。画面は `pending-approvals`、中身は `PendingApprovalsView` | 2-4・3-3 |
| `app/_components/demo-work-months-screen.tsx` | 同上 | `demoWorkMonthsScreen(deps)`。同上。画面は `demo-work-months`、中身は `DemoWorkMonthsView` | 3-1・3-3 |
| `app/_lib/screen-deps.ts` | サーバー専用 | 型 `ScreenDeps` と本物の依存 `serverScreenDeps`（T-0-2）。型 `ThisMonthDeps` と本物の依存 `serverThisMonthDeps`（W-7） | — |
| `app/_components/app-shell.tsx` | サーバー（同期。値は props で受ける） | `SiteHeader`（`screen="app"`）・`AppSideNav`・`<main>` を並べる。ログイン画面からは使わない。画面3 も使う | 10-1 |
| `app/_components/app-side-nav.tsx` | クライアント | 技術者のとき自分の契約を取得し、`thisMonthEntry` と `sideNavItems`（W-6）で項目を決めて `SideNav` へ渡す | 10-1〜10-9-g |
| `app/_components/own-work-months-view.tsx` | クライアント | 本人一覧の取得・絞り込み・ページ・表示（W-4） | AC-1・AC-4・AC-5・AC-6・7-1・7-3・AC-9 |
| `app/_components/pending-approvals-view.tsx` | クライアント | 承認待ち一覧（W-5） | AC-2・AC-4・AC-5・7-2・7-3・AC-9 |
| `app/_components/demo-work-months-view.tsx` | クライアント | デモ勤務月群（W-5） | AC-3・5-1・7-3・9-3 |
| `app/_components/work-month-table.tsx` | フックを持たない（どちらからも使える） | 行の表。各行に契約表示名・対象年月・`StatusBadge`・勤務月画面へのリンク | 1-2・2-3・5-1 |
| `app/_components/work-month-filters.tsx` | フックを持たない（値とハンドラを props で受ける） | `SelectField` で条件を出し、条件の解除の操作を出す。出す条件の集合を props で受ける | 9-1・9-2・9-11・9-14 |
| `lib/work-month-list.ts` | 純粋 | 行の型、絞り込み、選択肢、ページ分け、直近、年月の表示、1ページの件数の定数（W-3・W-4） | 4-1・4-2・6-3・9-4〜9-9・9-11 |
| `lib/work-month-list-client.ts` | 純粋（ブラウザで動く） | R-2〜R-4・R-10 を呼び、結果を分類する（W-2・W-3） | 1-1・7-3 |
| `lib/side-nav-items.ts` | 純粋 | サイドバーの項目（W-6） | 10-2〜10-6・10-9-a〜10-9-d |
| `lib/this-month-entry.ts` | 純粋 | `thisMonthEntry(contracts)`。自分の契約の件数で、当月の入力の導線を `hidden` / `link` / `choice` に分ける（W-6）。行き先は年月を含まない `thisMonthPath(contractId)`（W-7）で、年月を受け取らない。サイドバーの「今月の入力」と、本人一覧の0件の初回入力（W-4）が同じものを使う。出所: 2026-10-29 人間の明示選択（Q-9 = (a) 共有するのは出し分けの関数と開いたときの見え方だけ）・（Q-10 = 年月を含まない経路を行き先にする） | 6-2・7-1・10-9-a〜10-9-d |
| `app/_components/initial-entry-action.tsx` | クライアント | 本人一覧の0件の `EmptyState` の `action`。`thisMonthEntry` の結果で、何も出さない／リンク／選択肢を開く操作を出す（W-4）。画面固有の部品であり、`design-system.md` の台帳（AC-5-1）に載せない。出所: 2026-10-29 人間の明示選択（Q-9 = (a) 初回入力の部品は画面固有の `app/_components/initial-entry-action.tsx` に置く。台帳は変えない） | 6-2・7-1 |
| `lib/jst-month.ts` | 純粋 | `currentJstYearMonth(now: Date): string`。与えた時刻の JST の年月（`YYYY-MM`）を返す。W-7 の経路が当月を決めるのと、R-10 の `yearMonth`（W-3）に使う | 10-9 |
| `app/this-month/page.tsx` | サーバー（async。薄い層） | `export const dynamic = "force-dynamic"` を持ち、`thisMonthScreen(serverThisMonthDeps, await searchParams)` を呼ぶだけ（W-7・T-0-2）。出所: 2026-10-30 人間の明示選択（Q-11 = (e)）。`force-dynamic` は AI 起案（W-7 の先読み） | — |
| `app/_components/this-month-screen.tsx` | サーバー（async 関数。依存を引数で受ける） | `thisMonthScreen(deps, searchParams)`。区分と押された時点の JST の当月から送り先を決め、`redirect()` するだけで何も描かない（W-7）。ドメイン API も `lambda-client` も BFF も呼ばない | 10-9・10-9-a・10-9-c・10-9-e・10-9-g |
| `lib/screen-routes.ts`（既存へ追加） | 純粋 | `workMonthPath(contractId, yearMonth)`（C-2）、`thisMonthPath(contractId)`（C-2・W-7）、`ownWorkMonthsPath({ contractId? })`（9-12 の受け口のクエリを組み立てる） | 5-1・9-12・10-9 |

- 表の各ファイルが輸出する関数の形と props は W-8 に置く（今月の入力の経路は W-7）
- `_components` に置くのは P-6 のため（`src/components/` の台帳を増やさない）。既存部品で使うのは `SideNav`・`Card`・`StatusBadge`・`SelectField`・`Button`・`Pagination`・`EmptyState`・`ErrorBanner`・`AppHeader`（`SiteHeader` 経由）で、**部品の props を変えない**。例外は `SideNav` で、`design-system.md` 6-13 の 2026-10-29 改定（選択肢を開く項目。6-13-v）に従って `components/side-nav.tsx` の `SideNavItem` に形を1つ足す（props のキーは変えない）。出所: 2026-10-29 人間の明示選択（Q-8）
- 遷移はいずれも `next/link` のリンクで行う（行・直近・初回入力・サイドバーの項目と、それらが開く契約の選択肢）。初回入力と「今月の入力」のリンクは勤務月画面を直接指さず、W-7 の経路を指す（Q-10）。遷移は GET だけで、勤務月の状態を変えない（5-2・9-13・10-7）

### W-2. データを取る側と状態を持つ側

**一覧のデータはブラウザ（クライアント部品）が BFF の Route Handler から取る。** サーバーのページでは取らない。理由:

1. ページ・`_components` は `lambda-client` を輸入しない（C-1。`docs/rules/architecture.md` の「Route Handler 以外から Lambda を呼ばない」）
2. サーバーのページから自分の Route Handler を HTTP で呼ぶと、オリジンの解決と Cookie の転送が要るうえ、レート制限の鍵（`rate-limit.ts` は閲覧者のアドレスのヘッダで数える）が閲覧者のものでなくなり、全員が1つの鍵を共有する
3. 絞り込みの選択肢と直近は一覧の全体を要し、絞り込みはブラウザで行う（W-3。2026-10-29 人間の明示選択（Q-5 = (a)））

| 状態 | 持ち主 | 内容 |
|---|---|---|
| 利用者の区分・ロール | **サーバー**（C-3） | ページが解決し、props で渡す。クライアントで持たない |
| 取得の結果 | クライアント（`useState`） | `loading` / `failed` / `loaded`（行の配列。本人一覧は自分の契約の配列も持つ） |
| 絞り込みの条件 | クライアント（`useState`） | `{ contractId, yearMonth, state }`。空文字が「すべて」（9-4） |
| ページ | クライアント（`useState`） | 1 始まり。条件を変えたら 1 へ戻す（9-9） |

- **条件とページを URL に書き戻さない**（`router.push`・`replace` を呼ばない）。再読み込みすると既定へ戻る。例外は 9-12 の受け口のクエリ `contract` で、ページが読んで初期値として渡す（W-4）
- 取得中は一覧の領域に `aria-busy="true"` を付け、文言を出さない（取得中の文言を定める仕様が無いため）
- 応答の分類（`work-month-list-client.ts`）: 200 → `ok`、401 と 403 → `refresh`、それ以外の応答（429 を含む）・例外 → `failed`。`refresh` は L-4 と同じく `router.refresh()` を呼び、区分の判定をサーバーに任せる（C-4。別タブでのロール切替やセッションの失効のあと、開けない画面から既定の画面へ移る）。`failed` は `ErrorBanner` に `onRetry` を付けて出し、再試行は同じ GET をやり直すだけである（7-3）

### W-3. 取得と絞り込みの置き場

**事実**: ドメイン API の E-2 が受けるクエリは `engineerId`・`state`・`limit`・`offset` だけで（`domain-api-http-contract.md` AC-3）、BFF の R-2・R-3 が渡すのも `state`・`limit`・`offset` だけである（`bff-wiring.md` AC-6 の R-2・R-3 の行、`work-month-handlers.ts` の `listParameters`）。**契約・対象年月で絞る条件は、BFF にもドメイン API にも無い。** R-4 は要求から何も取らない（`demo-data-reset.md` AC-7-6）。絞り込みの置き場は 8-6 が実装工程に委ねている。

**絞り込みはブラウザで行い、一覧は全件を取得する。BFF とドメイン API は変えない。** 出所: 2026-10-29 人間の明示選択（Q-5 = (a)）。以下はその設計である（全件の取り方・並び・選択肢・ページの分け方は AI 起案）。

- **全件の取得**: `fetchAllWorkMonths(path, fetchImpl)` は `path?offset=N` を GET し、`limit` を送らない（上限はドメイン API の実装の値であり契約で固定されていない＝同 AC-3-5。既定の件数で返った分を受け取る）。次の `offset` は「今の `offset` ＋ 受け取った `items` の件数」とし、`offset` が `total` 以上になるか `items` が空になったら止める。行は `(contractId, yearMonth)` で重複を除き、先に来たものを残す
- **並び**: 応答の並び（対象年月の降順、同じ年月は契約識別子の昇順。同 AC-3-4）を保ち、ブラウザで並べ替えない。絞り込みは並びを保つ（4-1・9-8）
- **絞り込み**: 状態を含むすべての条件をブラウザで当てる（R-2 に `state` を送らない。選択肢と直近が絞る前の全体を要するため）
- **選択肢**: 本人一覧の契約は R-10（`GET /api/contracts?yearMonth=<JST の当月>`）の `items`。E-8 は `yearMonth` を必須とするが、返す契約の集合は `yearMonth` によらない（`domain-api-http-contract.md` 12-3・12-16）。承認待ち一覧の契約と、両一覧の対象年月は取得した行から作る（9-6・9-6-a）
- **ページ**: 絞った後の行を `WORK_MONTH_LIST_PAGE_SIZE` 件ずつに分ける。値は `lib/work-month-list.ts` の定数で持ち、仕様の値ではない（4-2・8-4 が実装工程に委ねた）。テストは定数を輸入して使い、数値を書かない
- **デモ勤務月群**: R-4 を1回呼ぶだけで、ページを送らない（R-4 はクエリを読まない）。絞り込みもページも置かない（9-3。AC-4 は本人一覧と承認待ち一覧が対象）

### W-4. 本人一覧（`own-work-months-view.tsx`）

並びはハンドオフ `My Months (Engineer).dc.html` に倣う —— 見出し → (取得エラー | 0件 | 通常: 直近 → 絞り込み → (絞り込みの結果が0件 | 表と `Pagination`))。

| 要素 | 部品 | 文言・内容 | 出所 | AC |
|---|---|---|---|---|
| 見出し | `<h1>` と説明 | 「自分の勤務月」／「担当しているすべての契約の勤務月です。」 | ハンドオフ 25・26 行 | — |
| 取得エラー | `ErrorBanner`（`onRetry` 付き） | 「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」 | ハンドオフ 31 行 | 7-3 |
| 0件 | `EmptyState` | title「勤務月がまだありません」、description「稼働実績を入力すると、その月の勤務月がここに表示されます。」、action は `initial-entry-action.tsx`（下の「初回入力の導線」） | ハンドオフ 36・158 行 | 6-2・7-1 |
| 直近 | `Card` | 「直近の勤務月」の見出しに、直近の行の対象年月・契約表示名・`StatusBadge` を添え、リンク「直近の勤務月を開く」→ `workMonthPath`。直近は絞る前の全体の先頭（`latestRow`）。ハンドオフの状態ごとの補足文（122 行）は採らない（要求が無い） | ハンドオフ 46・54 行 | 6-1・6-3 |
| 絞り込み | `role="search"` の領域と `work-month-filters.tsx` | 領域の名「勤務月の絞り込み」。`SelectField` 3つ（ラベル「契約」「対象年月」「状態」、`placeholder`「すべて」＝値は空文字）。状態の選択肢の表示は `StatusBadge` と同じ語。条件が1つ以上あるときだけ `Button`「条件をクリア」 | ハンドオフ 60・63・69・75・81・160 行 | 9-1・9-4・9-6・9-6-a・9-6-b・9-11・9-14 |
| 絞り込みの結果が0件 | `EmptyState` | title「条件に合う勤務月はありません」、description「絞り込みの条件を変えるか、クリアしてください。」、action は `Button`「条件をクリア」。初回入力のリンクを出さない | ハンドオフ 88・184 行 | 9-10 |
| 表 | `Card` と `work-month-table.tsx` | 名「すべての勤務月」の `<table>`。列見出し「契約」「対象年月」「状態」と、視覚的に隠した「操作」。各行のリンク「開く」のアクセシブル名は「`<契約表示名>` × `<対象年月>`を開く」→ `workMonthPath` | ハンドオフ 94・96・103・171 行 | 1-2・5-1 |
| 対象年月の表示 | `formatYearMonth` | `2026-07` → 「2026年7月」 | ハンドオフ 155 行 | 1-2・9-6-a |
| ページ | `Pagination` | 表があるときに出す。`pageCount` は `max(1, ceil(件数 / WORK_MONTH_LIST_PAGE_SIZE))` | — | 4-2・9-9 |

- **初回入力の導線**（`initial-entry-action.tsx`）: サイドバーの「今月の入力」と同じ `thisMonthEntry(自分の契約)`（W-6）の結果で出し分ける。出所: 2026-10-29 人間の明示選択（Q-8 = 初回入力のリンクにも同じ選択の部品を使い回す。契約が0件の技術者には初回入力のリンクを出さない）
  - **共有するもの**: 出し分けの関数 `thisMonthEntry` と、開いたときの見え方（`design-system.md` 6-13-v と同じ観測可能な形）だけである。部品は `SideNav` と共有せず、画面固有の `app/_components/initial-entry-action.tsx` に置く（`SideNav` は `<nav>` を出すため `EmptyState` の中に置けない）。`design-system.md` の部品台帳（AC-5-1）と AC-6 の表は変えない。出所: 2026-10-29 人間の明示選択（Q-9 = (a)）
  - `hidden`（自分の契約が0件）: action を**出さない**。`EmptyState` の title と description だけを示し、押すと行き止まりになる導線を作らない
  - `link`（1件）: リンク「稼働実績を入力する」→ その契約の `thisMonthPath`（W-7）
  - `choice`（2件以上）: `button`「稼働実績を入力する」（`aria-expanded`）を押すと、自分の契約の選択肢（契約表示名のリンク。それぞれ その契約の `thisMonthPath`）を開く。選ぶ前に勤務月画面を開かない（10-9-b と同じ）。開いたときの見え方は `design-system.md` 6-13-v と同じ（描画した直後は閉じていて選択肢のリンクが文書に無い、押すと `aria-expanded="true"` になり選択肢が `options` の順で `button` より後に現れる、再び押すと閉じる）。出所: 2026-10-29 人間の明示選択（Q-9 = (a)）。**6-13-v の観測可能な形そのものは AI の起案である**（`design-system.md` 6-13-v）
  - 当月は描画の時点で決めない。リンクが押された時点で W-7 の経路が決める（Q-10）
- 件数の表示（ハンドオフ 183 行の「全 N 件」）は出さない（8-7 が要求しない）
- **9-12 の受け口**: クエリ `contract`（`ownWorkMonthsPath({ contractId })` が組み立てる）。自分の契約の取得が済んでから、値が契約の選択肢にあれば契約の条件の初期値とし、**無ければ無視して「すべて」とする**（`SelectField` が選択肢の外の値を表せないため。AI 起案）。クエリの名前は 9-12 が固定しない（`design-system.md` 8-4）
- 自分の契約（R-10）の取得に失敗したときも、取得エラーとして扱う（行だけを出して契約の選択肢を欠いた状態にしない）

### W-5. 承認待ち一覧・デモ勤務月群

**承認待ち一覧（`pending-approvals-view.tsx`）** —— 本人一覧から直近を除き、条件を契約・対象年月の2つにしたもの。

| 要素 | 文言・内容 | 出所 | AC |
|---|---|---|---|
| 見出し | 「承認待ちの勤務月」／「勤務月を開いて内容を確認し、承認または差し戻しをしてください。」 | ハンドオフ `Pending Approvals (Approver).dc.html` 25・26 行 | — |
| 取得エラー | `ErrorBanner`（`onRetry` 付き）。本人一覧と同じ「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」 | 2026-10-29 人間の明示選択（Q-6 = (a) 本人一覧の文言を流用） | 7-3 |
| 0件 | `EmptyState` title「承認待ちの勤務月はありません」、description「技術者が勤務月を締めると、ここに表示されます。」。action を持たない | 同 31 行 | 7-2 |
| 絞り込み | 領域の名「勤務月の絞り込み」、`SelectField`「契約」「対象年月」、「条件をクリア」。状態の条件を出さない | 同 37・40・46・52 行 | 9-2・9-6・9-6-a・9-11・9-14 |
| 絞り込みの結果が0件 | 本人一覧と同じ文言 | 同 59 行 | 9-10 |
| 表 | 名「承認待ちの勤務月」。列と行のリンクは本人一覧と同じ。状態の列に `StatusBadge`（`PendingApproval`） | 同 65・67・74 行 | 2-3・5-1 |

- R-3 が返した行をそのまま載せ、自分が技術者である勤務月を除かない（2-5）。行に技術者の識別子は無い（`domain-api-http-contract.md` AC-10-2）
- `PendingApproval` 以外を載せないことは R-3 が `state` を固定することで担保される（`bff-wiring.md` 6-c）。画面は状態で行を除かない

**デモ勤務月群（`demo-work-months-view.tsx`）**

| 要素 | 文言・内容 | 出所 | AC |
|---|---|---|---|
| 注記 | `role="note"` に「デモ用のサンプルです（閲覧のみ・毎日リセットされます）」「ゲストとして閲覧しています。入力・締め・承認はできません。」 | ハンドオフ `Guest Demo List.dc.html` 25〜27 行 | 3-2・3-5 |
| 見出し・表の名 | 「サンプル勤務月」 | 同 31・35 行 | — |
| 表 | 列は本人一覧と同じ。各行のリンクは勤務月画面へ。リンクの文言は本人一覧と同じ「開く」で、アクセシブル名は「`<契約表示名>` × `<対象年月>`を開く」（ハンドオフの「表示する」は採らない） | 2026-10-29 人間の明示選択（Q-6 = (a) 行のリンクは「開く」） | 3-1・3-2・5-1 |
| 取得エラー | `ErrorBanner`（`onRetry` 付き）。本人一覧と同じ「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」 | 2026-10-29 人間の明示選択（Q-6 = (a) 本人一覧の文言を流用） | 7-3 |
| 0件 | `EmptyState` の title「サンプル勤務月はありません」。description と action を持たない | 2026-10-29 人間の明示選択（Q-6 = (a) デモ0件は「サンプル勤務月はありません」） | — |

- 絞り込み・ページ・直近を置かない（9-3）

### W-6. サイドバー（`side-nav-items.ts`・`app-side-nav.tsx`）

`sideNavItems({ viewer, current, thisMonth })` が `SideNav` の `items` と `footerItems` を返す純粋な関数である。

- `current` は開いている画面のキー（`this-month` / `own-work-months` / `contracts` / `work-trends` / `pending-approvals` / `demo-work-months` / `help`）。一覧の3画面はそれぞれのキーを渡す。勤務月画面での選び方（10-5 の後半）は M-10 が決め、同じ関数へキーを渡す
- 項目の集合・文言・並びは 10-2〜10-4・10-6 をそのまま表にする（最下部の「ヘルプ」は `footerItems`）。承認担当者一覧の項目を作らない（10-4-a）
- `href`: 本人の勤務月一覧・承認待ち一覧・デモ勤務月群・ヘルプ・契約一覧・稼働の推移は C-2。ヘルプ・契約一覧・稼働の推移はページが入るまで行き先が 404 になるが、項目は 10-2〜10-4 どおり出す。出所: 2026-10-29 人間の明示選択（Q-7 = (a)）
- **今月の入力**（技術者だけ）: `thisMonth` は `thisMonthEntry(自分の契約)` の結果で、`{ kind: "hidden" }`・`{ kind: "link"; href }`・`{ kind: "choice"; options: { label; href }[] }` のいずれか。`app-side-nav.tsx` が R-10 で自分の契約を取り、0件なら `hidden`（10-9-d）、1件ならその契約の `thisMonthPath`（W-7）の `link`（10-9-a）、2件以上なら `choice`（10-9-b）。`choice` の `options` は R-10 の `items` の全部を応答の順のまま並べ、`label` は契約表示名、`href` はその契約の `thisMonthPath` とする（10-9-c。契約期間で絞らない）。取得中と取得の失敗のときは `hidden` とする（AI 起案）。行き先に年月を含めないのは、当月を押された時点でサーバーが決めるためである。出所: 2026-10-29 人間の明示選択（Q-10 = 「今月の入力」と初回入力のリンクは年月を含まない経路を行き先にする。`SideNav` はリンク（`href`）のまま。`design-system.md` 6-13 は再改定しない）
  - `sideNavItems` は `link` を `SideNavItem` の `{ label: "今月の入力", href, current }` に、`choice` を選択肢を開く項目 `{ label: "今月の入力", options, current }` に写す。`SideNav` が「今月の入力」の下に契約の選択肢を開く（`design-system.md` 6-13-v）。**選択の画面は挟まない**。出所: 2026-10-29 人間の明示選択（Q-8 = 契約が複数あるときは SideNav の「今月の入力」の下に契約の選択肢を開く。選択の画面を挟まないため `docs/rules/scope.md`「列挙に無い画面は加えない」に当たらない）
  - **契約が0件の技術者には「今月の入力」を出さない**（10-9-d。押すと行き止まりになる導線を作らない）。出所: 2026-10-29 人間の明示選択（Q-8 付随）
- **当月の決め方**: サイドバーと初回入力は当月を持たない。当月は W-7 の経路が、押された時点で決める。**月の境目に合わせたタイマーで描き直す設計は取り下げた**。出所: 2026-10-29 人間の明示選択（Q-10）
- 選択中は `current` に対応する項目だけ（10-5）
- ロール切替の後（10-8）は、L-4 の `router.refresh()` でページが新しい区分を props で渡し直すため、項目も描き直される。サイドバー自身はロールを持たない

### W-7. 今月の入力の経路（`/this-month`）

出所: 2026-10-29 人間の明示選択（Q-10 = 「今月の入力」と初回入力のリンクは、年月を含まない経路を行き先にする。押された時点でサーバーが JST（`Asia/Tokyo`）の当月を計算し、その契約 × 当月の勤務月画面へ 302 で送る。この経路は何も表示せず、送り先へ回すだけで画面を増やさない。「302」の扱いは下記「転送の状態コード」）と、2026-10-30 人間の明示選択（Q-11 = (e) 送るだけのページ。下記「置き場」「送り先」（`misconfigured` の行を含む）「本人の契約を確かめる場所」）。パスとクエリの名前は C-2 のとおり AI の起案である。

**Q-10 の出所のうち「契約 ID はクエリで受け取るが、それが本人の契約かをサーバー側で確かめずに送らない（他人の契約・ゲスト・未認証には送らない）」は、2026-10-30 の Q-11 = (e) により次の形に置き換わった** —— ゲスト・承認者・未認証は勤務月画面へ送らない（区分の既定の画面へ送る）。技術者については、本人の契約かを `/this-month` では確かめず、送り先の勤務月画面（画面3）とドメイン API が確かめる。

- **画面ではない**: 何も表示せず、`ScreenId`（C-4）に加えない。`login-ui.md` 6-7 の画面の集合を増やさない（`docs/rules/scope.md`「列挙に無い画面は加えない」に当たらない）
- **置き場**: `redirect()` するだけのページ `app/this-month/page.tsx` に置く。**ドメイン API も `lambda-client` も BFF の Route Handler も呼ばない**。`bff-wiring.md` と `docs/rules/architecture.md` は変えない。出所: 2026-10-30 人間の明示選択（Q-11 = (e)）。ページを薄い層と本体に分けること（T-0-2 の形）と、下記の関数の形は AI 起案
- **送り先**: 区分は画面1 の区分の解決（C-3。Cookie から）で決める。出所: 2026-10-30 人間の明示選択（Q-11 = (e)）。表の「契約 ID」の読み方（空文字・配列を「無い」とみなす）は AI 起案

| `deps.getCurrentViewer()` の結果 | クエリ `contract` | 本体がすること | 根拠 |
|---|---|---|---|
| `misconfigured` | 問わない | 例外を投げる。`redirect()` を呼ばない（ほかの画面と同じくエラーページ） | C-3。Q-4 (ii) = (a)。Q-11 = (e) |
| `guest`（未認証・セッションの検証に失敗したものを含む。C-3 の順 (2)） | 問わない | `redirect(defaultScreenPath(viewer))`（デモ勤務月群） | `login-ui.md` 6-7-a。Q-11 = (e) |
| 承認者 | 問わない | `redirect(defaultScreenPath(viewer))`（承認待ち一覧） | 同上 |
| 技術者 | 空でない文字列が1つ | `redirect(workMonthPath(契約 ID, currentJstYearMonth(deps.now())))` | 10-9・10-9-a・10-9-c。Q-10・Q-11 = (e) |
| 技術者 | 無い・空文字・配列 | `redirect(ownWorkMonthsPath({}))`（本人の勤務月一覧） | Q-11 = (e)（契約 ID が無いとき）。空文字・配列を含めるのは AI 起案 |

```mermaid
flowchart TD
  Start["thisMonthScreen(deps, searchParams)"] --> V{"deps.getCurrentViewer()"}
  V -->|misconfigured| Err["例外（エラーページ）"]
  V -->|"guest・承認者"| Def["redirect(defaultScreenPath(viewer))"]
  V -->|技術者| C{"contract が空でない文字列1つか"}
  C -->|いいえ| Own["redirect(ownWorkMonthsPath({}))"]
  C -->|はい| WM["redirect(workMonthPath(契約 ID, currentJstYearMonth(deps.now())))"]
```

- **本人の契約を確かめる場所**: `/this-month` では確かめない。送り先の勤務月画面（画面3）とドメイン API が確かめる。**URL を直接入力して勤務月画面を開いたときと同じ扱い**である。理由: 送り先は URL を直接入力しても開けるので、途中で確かめても守りは増えない。出所: 2026-10-30 人間の明示選択（Q-11 = (e)）
- **当月**: 本体が `deps.now()` を1回呼び、その時刻で `currentJstYearMonth` を呼ぶ。`/this-month` は動的に描くページなので（下記「先読み」）、`deps.now()` は要求を受けた時点＝押された時点の時刻になる。`currentJstYearMonth(now)` は `now` に 9 時間を足した UTC の年月を返す（JST は夏時間を持たない。AI 起案）。これにより、当月の判定が「選んだ時点の JST の日付による」（10-9）
- **未生成の当月**: 当月の勤務月が未生成でも送る（10-9-e。生成は勤務月画面でもしない）
- **状態を変えない**: 本体は `redirect()` を呼ぶほかに何もしない。`fetch` を呼ばず、勤務月の状態・稼働実績を変えない（10-9-g）
- **転送の状態コード**: Q-10 の「302 で送る」は、Q-11 = (e) により `redirect()` で送る形になった。`redirect()` が URL を直接開いた要求に返すのは 307（一時的な転送）である（`apps/web/node_modules/next/dist/docs/01-app/03-api-reference/04-functions/redirect.md` 12 行・215 行）。`next/link` で遷移したときの送り方は Next に任せる。本仕様もテストも状態コードを固定せず、`redirect()` に渡した送り先を読む（302 と 307 の違いは「人間の決定を待っている論点」の末尾に、確かめる対象として残す）

**関数の形（AI 起案）**

```ts
// app/_lib/screen-deps.ts（サーバー専用。既存の ScreenDeps・serverScreenDeps は変えない）
export type ThisMonthDeps = {
  getCurrentViewer: () => Promise<ResolveViewerResult>;
  now: () => Date;
};
export const serverThisMonthDeps: ThisMonthDeps; // getCurrentViewer は serverScreenDeps と同じもの、now は () => new Date()

// app/_components/this-month-screen.tsx（本体。screen-deps.ts は import type に限る）
export type ThisMonthSearchParams = { [key: string]: string | string[] | undefined };
export function thisMonthScreen(deps: ThisMonthDeps, searchParams: ThisMonthSearchParams): Promise<never>;

// app/this-month/page.tsx（薄い層）
export const dynamic = "force-dynamic";
export default async function ThisMonthPage(props: { searchParams: Promise<ThisMonthSearchParams> }): Promise<never>;
```

- 時刻の注入は `ScreenDeps` に足さず、別の型 `ThisMonthDeps` で行う。`ScreenDeps` に `now` を足すと、時刻を使わないほかの画面の本体とテストの Fake（`login-screen.test.tsx`・画面2 の `*-screen.test.tsx`）まで形が変わるため。`ThisMonthDeps` はテーマを持たない（何も描かないため）
- 本体はどの行でも `redirect()` を**ちょうど1回**呼ぶか例外を投げて終わり、JSX を返さない（戻り値の型 `Promise<never>`）

**先読み（AI 起案。人間が「先読みを止めるか、動的な描画であることを確かめ、テストか設計で担保する」ことを求めた）**

`next/link` の先読みが `/this-month` の本体を走らせると、先読みした時点の月で送り先が決まり、「押された時点」（10-9）から外れるおそれがある。`SideNav`（`components/side-nav.tsx`。`next/link` を使う）の props を変えて先読みを止めると `design-system.md` 6-13 の再改定になり、Q-10 の「6-13 は再改定しない」とぶつかる。そこで、**`/this-month` を先読みされない動的なページに保つ**:

- 根拠: Cache Components を使わない構成では、動的なページは `loading.js` の境界が無い限り先読みされない（`apps/web/node_modules/next/dist/docs/01-app/02-guides/prefetching.md` 31〜35 行）。`<Link>` の `prefetch` が既定（`"auto"`／`null`）のとき、動的なルートは最も近い `loading.js` の境界までの一部だけが先読みされる。`prefetch` が `true` なら動的なルートも全体が先読みされる。Partial Prefetching を有効にすると既定が変わる（`apps/web/node_modules/next/dist/docs/01-app/03-api-reference/02-components/link.md` 302〜306 行）。`partialPrefetching` は `cacheComponents` を要する（`.../05-config/01-next-config-js/partialPrefetching.md` 36 行）
- 担保の形:
  1. `app/this-month/page.tsx` は `export const dynamic = "force-dynamic"` を持つ
  2. `app/this-month/` とその上（`app/`）に `loading.*` を置かない
  3. `next.config.ts` で `cacheComponents`・`partialPrefetching` を有効にしない
  4. `/this-month` を指すリンクを描く部品（`components/side-nav.tsx`・`app/_components/initial-entry-action.tsx`）は `next/link` に `prefetch` を渡さない（既定のまま）
- これらを静的に読むテストで担保する（T-2）。Next が実際に先読みしないことは本番の構成でしか起きず（先読みは本番だけで働く。同 `link.md` 298 行）、テストでは読まない（T-2 の限界）

**画面3 への申し送り**: `/this-month` は本人の契約かを確かめずに勤務月画面へ送る（上記）。したがって画面3 の設計とテストは、**技術者が他人の契約の勤務月を開いたとき（URL の直接入力と `/this-month` 経由の両方が同じ経路になる）の扱いを、必ず条文とテストに入れる**。出所: 2026-10-30 人間の明示選択（Q-11 = (e)）。扱いの中身は画面3 の節で決める（本節では決めない）。受けた先は M-10・T-3

### W-8. 画面2 の関数の形と props（AI 起案）

T-2 の表が求めるテストを書ける粒度で、輸出する名前と型を定める。**本節はすべて AI 起案であり、人間の承認前である。** 行・契約の形は `domain-api-http-contract.md` AC-10-2（E-2 の行。R-2〜R-4 が中継する）・AC-10-4（E-8 の行。R-10 が中継する）のキー名に従い、画面2 が読むキーだけを持つ。

```ts
// lib/work-month-list.ts（純粋）
export type WorkMonthState = "Draft" | "PendingApproval" | "Approved";
export interface WorkMonthRow {          // AC-10-2 の items[] の1行
  contractId: string;
  contractDisplayName: string;
  yearMonth: string;                     // YYYY-MM
  state: WorkMonthState;
}
export interface ContractSummary {       // AC-10-4 の items[] のうち画面2 が読むキー
  contractId: string;
  contractDisplayName: string;
}
export interface WorkMonthFilter {       // 空文字が「すべて」（9-4）
  contractId: string;
  yearMonth: string;
  state: "" | WorkMonthState;
}
export interface FilterOption { value: string; label: string } // SelectField の options の1つ
export const EMPTY_WORK_MONTH_FILTER: WorkMonthFilter;          // 3つとも空文字
export const WORK_MONTH_LIST_PAGE_SIZE: number;                 // 1 以上の整数（W-3）

export function filterWorkMonths(rows: readonly WorkMonthRow[], filter: WorkMonthFilter): WorkMonthRow[]; // 9-4・9-5・9-8
export function hasActiveFilter(filter: WorkMonthFilter): boolean;               // 9-11（1つ以上が空文字でない）
export function contractOptionsFromContracts(contracts: readonly ContractSummary[]): FilterOption[]; // 本人一覧（9-6）。入力の順
export function contractOptionsFromRows(rows: readonly WorkMonthRow[]): FilterOption[];             // 承認待ち一覧（9-6）。重複なし・最初に現れた順
export function yearMonthOptions(rows: readonly WorkMonthRow[]): FilterOption[];                    // 9-6-a。重複なし・降順。label は formatYearMonth
export function stateOptions(): FilterOption[];  // 9-6-b。Draft・PendingApproval・Approved の3つ。label は StatusBadge の描く語と同じ
export function pageCount(rowCount: number): number;                              // max(1, ceil(rowCount / WORK_MONTH_LIST_PAGE_SIZE))
export function pageRows<T>(rows: readonly T[], page: number): T[];               // 1 始まり。page 番目の WORK_MONTH_LIST_PAGE_SIZE 件
export function latestRow(rows: readonly WorkMonthRow[]): WorkMonthRow | undefined; // 6-3。先頭の行（同じ参照）。0件は undefined
export function formatYearMonth(yearMonth: string): string;                       // "2026-07" → "2026年7月"
```

- `FilterOption` の `value` は、契約は `contractId`、対象年月は `YYYY-MM`、状態は状態の値。`label` は契約表示名、`formatYearMonth` の結果、状態の表示の語
- `stateOptions` の `label` は lib が持つ（`StatusBadge` の表示の語は輸出されておらず、部品を変えないため）。一致はテストが `StatusBadge` を描いて比べる（T-2）
- 入力の配列を書き換えない（いずれの関数も新しい配列を返す。`latestRow` は要素の参照を返す）

```ts
// lib/work-month-list-client.ts（純粋。ブラウザで動く）
export type FetchResult<T> = { kind: "ok"; value: T } | { kind: "refresh" } | { kind: "failed" };

export function fetchAllWorkMonths(path: string, fetchImpl?: typeof fetch): Promise<FetchResult<WorkMonthRow[]>>;
export function fetchOwnWorkMonths(fetchImpl?: typeof fetch): Promise<FetchResult<WorkMonthRow[]>>;              // R-2 のパスで fetchAllWorkMonths
export function fetchPendingApprovalWorkMonths(fetchImpl?: typeof fetch): Promise<FetchResult<WorkMonthRow[]>>; // R-3 のパスで fetchAllWorkMonths
export function fetchDemoWorkMonths(fetchImpl?: typeof fetch): Promise<FetchResult<WorkMonthRow[]>>;            // R-4 を1回だけ。クエリを付けない
export function fetchOwnContracts(now: Date, fetchImpl?: typeof fetch): Promise<FetchResult<ContractSummary[]>>; // R-10。yearMonth = currentJstYearMonth(now)
```

- パスは `bff-wiring.md` AC-6 の R-2・R-3・R-4・R-10 の行のとおり（本仕様に書き写さない）。要求はいずれも GET で、本文を持たない
- `fetchImpl` を省いたときは、**呼び出しの時点の**大域の `fetch` を使う（モジュールの読み込み時に束ねない。テストが `globalThis.fetch` を差し替えるため＝T-0-1）
- 分類（W-2）: 200 → `ok`、401・403 → `refresh`、それ以外の応答（429 を含む）・例外・本文が JSON として読めない／`items` が配列でない → `failed`
- `fetchAllWorkMonths` は W-3 の「全件の取得」のとおり `offset=0` から送る。途中の要求が `ok` でなければ、そこで止めてその分類を返す（受け取った行は捨てる）
- `fetchOwnContracts` の `now` は、呼ぶ側が取得の時点で `new Date()` を渡す。`OwnWorkMonthsView` と `AppSideNav` はそれぞれ R-10 を呼ぶ（結果を共有しない）

```ts
// lib/this-month-entry.ts（純粋）
export type ThisMonthEntry =
  | { kind: "hidden" }
  | { kind: "link"; href: string }
  | { kind: "choice"; options: readonly { label: string; href: string }[] };
export function thisMonthEntry(contracts: readonly ContractSummary[]): ThisMonthEntry; // 引数は1つ（年月を受けない。Q-10）

// lib/side-nav-items.ts（純粋）
export type SideNavCurrent =
  | "this-month" | "own-work-months" | "contracts" | "work-trends"
  | "pending-approvals" | "demo-work-months" | "help";
export function sideNavItems(params: {
  viewer: Viewer;
  current: SideNavCurrent;
  thisMonth: ThisMonthEntry;
}): { items: SideNavItem[]; footerItems: SideNavItem[] }; // SideNavItem は components/side-nav.tsx（design-system.md 6-13 の2つの形）
```

```ts
// app/_components の props
export function AppShell(props: { viewer: Viewer; theme: Theme; current: SideNavCurrent; children: ReactNode }): JSX.Element;
export function AppSideNav(props: { viewer: Viewer; current: SideNavCurrent }): JSX.Element; // "use client"。技術者のときだけ fetchOwnContracts
export function OwnWorkMonthsView(props: { initialContractId?: string }): JSX.Element;       // "use client"
export function PendingApprovalsView(): JSX.Element;                                          // "use client"
export function DemoWorkMonthsView(): JSX.Element;                                            // "use client"
export function WorkMonthTable(props: { label: string; rows: readonly WorkMonthRow[] }): JSX.Element; // label は <table> のアクセシブル名
export function WorkMonthFilters(props: {
  filter: WorkMonthFilter;
  contractOptions: readonly FilterOption[];
  yearMonthOptions: readonly FilterOption[];
  stateOptions?: readonly FilterOption[];  // 省くと状態の条件を出さない（承認待ち一覧。9-2）
  onChange: (filter: WorkMonthFilter) => void;
  onClear: () => void;
}): JSX.Element;
export function InitialEntryAction(props: { entry: ThisMonthEntry }): JSX.Element | null;  // "use client"。hidden で null
```

- `ThisMonthEntry` の `choice` の `options` は readonly の配列とする（2026-10-30 に実装 `lib/this-month-entry.ts` に合わせて改めた。`side-nav-items.test.ts` が `as const` の値を渡すため。要求は変わらない）
- `AppSideNav` の `viewer` はサーバーが解決した `Viewer`（`misconfigured` を含まない）。クライアント部品なので `Viewer` は `import type` に限る（C-1）。`AppShell`・`AppSideNav` の `current` は、画面3 のために `SideNavCurrent | WorkMonthLocation` へ広げる（M-11。一覧の3画面が渡す値は変わらない）
- 3つの一覧の中身は、BFF の応答を `fetch` で取るため、データの props を受けない（W-2）。`router.refresh()` は `next/navigation` の `useRouter` から取る
- 本体（`*-screen.tsx`）の形は W-1 の表のとおり（`ownWorkMonthsScreen(deps: ScreenDeps, searchParams: { [key: string]: string | string[] | undefined })`、`pendingApprovalsScreen(deps: ScreenDeps)`、`demoWorkMonthsScreen(deps: ScreenDeps)`）

---

## M. 画面3 — 勤務月画面

本節で AC 番号だけを書いたものは `work-month-screen-ui.md` の AC である。ほかは「入力 UI」＝`daily-record-entry-ui.md`、「締め UI」＝`monthly-closing-ui.md`、「承認 UI」＝`approval-ui.md`、「契約」＝`domain-api-http-contract.md`、「一覧 UI」＝`work-month-listing-ui.md` と略す。**本節の決定は、出所を添えたものを除き、AI 起案であり人間の承認前である。** 文言のうち出所を「ハンドオフ N 行」と書いたものは `design/handoff/2026-10-theme-b/WorkMonthScreen.dc.html` の該当行の字面をそのまま採ったもので、本節では作っていない。

**論点 Q-12〜Q-15 は 2026-10-30 に人間が明示選択で決めた**（Q-12 = (b) → M-3、Q-13 = (a) と Q-14 = (b) → M-9、Q-15 = (a) → M-6・M-12）。決定に伴って残った AI 起案（真偽のキー名・真偽が応答に無いときの扱い・M-9 の文言・祝日データの形式）は、各節に「AI 起案」と書き、末尾「起案のまま残るもの」にも挙げる。

**#155 への依存**: M-3 の真偽は、勤務月の表現（契約 AC-10-1）へ足すものであり、契約 AC-2-5・AC-10-1・AC-10-3 と `workmonth-implementation-design.md` の出力 DTO の改定、ドメイン API の実装の変更は **#55 とは別の Issue である #155** が行う。#55 では `services/` と `domain-api-http-contract.md` を変えない。出所: 2026-10-30 人間の明示選択（Q-12 = (b)。ドメイン API の変更は別の Issue）。#155 の起票は 2026-10-30 に人間が承認した。

### M-1. 構成

```mermaid
flowchart TD
  Page["app/work-months/[contractId]/[yearMonth]/page.tsx（薄い層）"] -->|"serverScreenDeps・params・WorkMonthHolidayView を渡す"| Screen["_components/work-month-screen.tsx（本体）"]
  Screen -->|"deps.getCurrentViewer + decideScreen('work-month')"| Shell["_components/app-shell.tsx（既存。current は WorkMonthLocation）"]
  Shell --> AppSideNav["_components/app-side-nav.tsx（既存へ追加。選択中を M-10 で決める）"]
  Shell -->|"引数で受けた view を描く"| HolidayView["_components/work-month-holiday-view.tsx（クライアント。M-12 の段）"]
  HolidayView -->|"isHoliday を props で渡す"| View["_components/work-month-view.tsx（クライアント）"]
  HolidayView --> Holidays
  View -->|"R-1・R-5〜R-9"| BFF["BFF Route Handler（既存。応答をそのまま中継）"]
  View -->|"表現の真偽を読む"| Own["readOwnership（M-3）"]
  View --> Summary["_components/work-month-summary.tsx"]
  View --> Calendar["_components/work-month-calendar.tsx"]
  View --> Form["_components/daily-record-form.tsx（クライアント）"]
  View --> Ops["_components/work-month-operations.tsx（クライアント）"]
  View --> Actions["lib/work-month-actions.ts（出し分け）"]
  View --> Feedback["lib/work-month-feedback.ts（M-9）"]
  Calendar --> Cells["lib/work-month-calendar.ts（isHoliday は引数）"]
  Holidays["lib/holidays/（M-12 の段）"]
  Form --> Entry["lib/time-entry.ts"]
  Entry --> WH["lib/working-hours.ts（既存。computeWorkingHours）"]
  View --> Client["lib/work-month-client.ts"]
```

`lib/work-month-calendar.ts`・`WorkMonthView` は `lib/holidays/` を輸入しない。`lib/holidays/` を輸入するのは `work-month-holiday-view.tsx` だけであり、`page.tsx`（薄い層）がそれを本体へ渡す。これにより、祝日データの同梱（M-12）を画面3 の最後の段へ回しても、それより前の段のファイルとテストは祝日データに依存しない（出所: 2026-10-30 人間の明示選択（Q-15 = (a)。祝日のデータと `lib/holidays/` は画面3 の最後の段へ回す）。分け方は AI 起案）。

| ファイル | 種別 | 責務 | 主に担う AC |
|---|---|---|---|
| `app/work-months/[contractId]/[yearMonth]/page.tsx` | サーバー（async。薄い層） | `workMonthScreen(serverScreenDeps, await params, WorkMonthHolidayView)` を返すだけ（T-0-2）。M-12 の段より前は、`WorkMonthHolidayView` の代わりに渡すものが無いため、本ページは M-12 の段で置く（T-3） | — |
| `app/_components/work-month-screen.tsx` | サーバー（async 関数。依存を引数で受ける） | `workMonthScreen(deps, params, view)`。`misconfigured` なら例外（C-3）。`decideScreen("work-month", viewer)` が `redirect` なら移す（区分3種はいずれも開ける＝`login-ui.md` 6-7）。`show` なら `AppShell`（`current` は `{ kind: "work-month", contractId, yearMonth }`）の子に、引数の `view` を `viewer`・`contractId`・`yearMonth` で描く。`fetch` を呼ばない。`lib/holidays/` を輸入しない | `login-ui.md` 6-7・6-9、1-8・1-9 |
| `app/_components/work-month-holiday-view.tsx` | クライアント（M-12 の段） | `lib/holidays/` の `isHoliday` を `WorkMonthView` の props に渡すだけ | 入力 UI AC-9 |
| `app/_components/work-month-view.tsx` | クライアント | 勤務月の取得・本人かの読み取り（M-3）・操作・再取得・失敗の表示（M-2〜M-9）。祝日の判定は props の `isHoliday` で受ける | AC-1〜AC-7、入力 UI AC-2〜AC-7、締め UI AC-1〜AC-6、承認 UI AC-1〜AC-5・AC-10 |
| `app/_components/work-month-summary.tsx` | フックを持たない | 総稼働時間・精算幅・超過／不足（M-5） | 1-3〜1-5・AC-4 |
| `app/_components/work-month-calendar.tsx` | フックを持たない | カレンダー（M-6）。編集できるときは日を選ぶ `button`、それ以外は選べない要素 | 1-6、入力 UI AC-1・AC-9 |
| `app/_components/daily-record-form.tsx` | クライアント | 1日分の入力フォーム（M-7）。入力中の値を持つ | 入力 UI AC-3〜AC-5・AC-6 |
| `app/_components/work-month-operations.tsx` | クライアント | 締め（`CloseAction`）・承認と差戻し（`ReviewActions`）の区画と確認ステップ（M-8） | 締め UI AC-1・AC-2・AC-4、承認 UI AC-1〜AC-3・AC-10 |
| `lib/work-month.ts` | 純粋 | 勤務月の表現の型（契約 AC-10-1 のキー）、本人かの真偽のキー名（仮の名前）の唯一の持ち主と `readOwnership`（M-3）、時間・時刻の表記（M-5） | 1-3・1-4・AC-3、入力 UI 1-1・1-6 |
| `lib/work-month-client.ts` | 純粋（ブラウザで動く） | R-1・R-5〜R-9 を呼び、結果を分類する（M-2） | AC-6 |
| `lib/work-month-feedback.ts` | 純粋 | 取得・操作の失敗の分類と文言（M-9） | AC-6、入力 UI 4-1-a・4-3、締め UI 6-1、承認 UI 10-8 |
| `lib/work-month-actions.ts` | 純粋 | 区分×状態×本人かで、出す操作と注記を決める（M-4・M-5） | AC-3・AC-5 |
| `lib/work-month-calendar.ts` | 純粋 | カレンダーのマス・曜日の印・未来の日・日のアクセシブル名（M-6） | 入力 UI 1-4・1-5・4-2・AC-9 |
| `lib/time-entry.ts` | 純粋 | フォームの値の読み取り・弾く理由の分類・保存前の稼働時間（M-7） | 入力 UI 3-4・3-6〜3-6-b・3-6-f・4-1・4-5〜4-9 |
| `lib/holidays/`（新規。M-12 の段） | 純粋 | 同梱データと `isHoliday(date)`（入力 UI 10-3。M-12） | 入力 UI AC-9・AC-10 |
| `lib/jst-month.ts`（既存へ追加） | 純粋 | `currentJstDate(now)`（JST の当日 `YYYY-MM-DD`） | 入力 UI 4-2 |
| `lib/side-nav-items.ts`（既存へ追加） | 純粋 | `workMonthSideNavCurrent`（M-10） | 一覧 UI 10-5 |
| `app/_components/app-side-nav.tsx`・`app-shell.tsx`（既存へ追加） | — | `current` に `WorkMonthLocation` を受ける（M-10・M-11） | 一覧 UI 10-5 |

- `_components` に置くのは P-6 のため。既存部品で使うのは `Card`・`StatusBadge`・`SelectField`・`Button`・`EmptyState`・`ErrorBanner`・`FieldError`・`ConfirmDialog`（と `AppShell` 経由の `SideNav`・`AppHeader`）で、**部品の props を変えない**。可否は呼び出し側の条件レンダリングで表し、部品に状態・操作者種別を渡さない（`design-system.md` 7-2）
- ページ・`_components` は `lambda-client` を輸入しない（C-1）
- パスは C-2 のまま変えない（AI 起案・人間の承認前のまま）

### M-2. データを取る側と状態を持つ側

**勤務月のデータはブラウザ（`WorkMonthView`）が BFF の R-1 から取り、操作は R-5〜R-9 へ送る。** サーバーのページでは取らない。理由は W-2 の 1・2 と同じ。

| 状態 | 持ち主 | 内容 |
|---|---|---|
| 利用者の区分・ロール | **サーバー**（C-3） | props `viewer` で受ける。ロール切替の後は L-4 の `router.refresh()` で渡し直される |
| 勤務月 | クライアント（`useState`） | `loading` / `failed`（M-9 の取得の失敗）/ `missing`（M-9 の見つからない・不正）/ `loaded`（`WorkMonthDetail` と `Ownership`） |
| 本人か | 勤務月と同じ `useState`（`loaded` が持つ） | `Ownership`（`"own"` / `"other"` / `"unknown"`）。最新の表現から `readOwnership` で読む（M-3）。別に状態を持たない。`loaded` でないときは `"unknown"` と同じに扱う |
| 選んだ日 | クライアント（`useState`） | `YYYY-MM-DD` または `null`。初期値は `null`（ハンドオフ 377 行の既定の選択は採らない） |
| 入力中の値・保存を押したか | `DailyRecordForm`（`useState`） | `TimeEntryForm` と `tried`。日を選び直すと初期化する（`key` に日付を渡す） |
| 通信中 | クライアント（`useState`） | 通信中は次の操作（保存・削除・締め・承認・差戻し）を受けない（二重送信を防ぐ） |
| 操作の失敗 | クライアント（`useState`） | 失敗の種別（M-9）。次の操作を始めたら消す |
| 締めた直後 | クライアント（`useState`） | 締めが成功したら真。注記（M-5）にだけ使う |
| 当日 | 描画の時点で `currentJstDate(new Date())` | 未来の日の判定（入力 UI 4-2）に使う |

- **示す値は、ドメイン API が返した最新の表現だけから描く。** 操作の成功の応答は更新後の勤務月の表現である（契約 D-6）ため、それで置き換え、R-1 を取り直さない。保存の前に画面が示していた稼働時間で置き換えない（入力 UI 3-6-c）
- **409 を受けたときだけ R-1 を取り直す**（6-4・`bff-wiring.md` 8-6「再取得は画面の責務」）。取り直す間は `loading` とし（操作要素を出さない）、取り直した表現で操作要素を出し直すため、古い操作は成立しない。取り直しが `ok` でなければ、M-9 の取得の失敗・見つからないの表示へ移る（AI 起案）
- **`viewer` の区分かロールが変わったら R-1 を取り直す**（ロール切替の後の `router.refresh()` で props が変わったとき）。本人かの真偽は操作者ごとに決まる値であり、ロールに依るかは #155 が決める（M-3）ため、古い表現の真偽を新しいロールで使い回さない（AI 起案）
- URL に状態を書き戻さない。遷移は `next/link` と `router.refresh()` だけ
- 取得中は内容の領域に `aria-busy="true"` を付け、文言を出さない（W-2 と同じ）
- **応答の分類**（`work-month-client.ts`。一覧の分類 W-2 と異なり、403 は業務の拒否として扱う）:

| 応答 | 分類 | 画面がすること |
|---|---|---|
| 200 で、本文が JSON の object で `contractId` が文字列・`dailyRecords` が配列 | `ok`（`ownership` は `readOwnership(本文)`。真偽のキーの有無で `ok` か否かを変えない＝M-3） | 表現で置き換える |
| 401（本文を問わない） | `refresh` | `router.refresh()` をちょうど1回呼ぶ（区分の判定をサーバーに任せる。L-4 と同じ） |
| 400・403・404・409 で、本文が `{"error":{"code": 文字列}}` | `rejected`（`status` と `code` を持つ） | M-9 |
| それ以外（429・5xx・上の形でない 4xx・JSON でない本文・例外） | `failed` | M-9 |

### M-3. 本人の判定（勤務月の表現の真偽を読む）

**決定**: 勤務月の表現（E-1・E-3〜E-7）に「操作者がその勤務月の技術者本人か」を表す真偽を足し、画面はそれを読んで AC-3 の ①〜④ を出し分ける。技術者の識別子そのものは表現に載せない。**AC-3 は緩めない。** 出所: 2026-10-30 人間の明示選択（Q-12 = (b)）。

- **担い手は #155**: 真偽を足す契約の改定（契約 AC-2-5・AC-10-1・AC-10-3）、`workmonth-implementation-design.md` の出力 DTO の改定、ドメイン API の実装の変更は、#55 とは別の Issue である #155 が行う。#55 は `services/` と `domain-api-http-contract.md` を変えない。出所: 同上（#155 の起票は 2026-10-30 に人間が承認した）
- **キー名は仮の名前**: 真偽のキー名は #155 の仕様工程が決める。本仕様は仮の名前 `operatorIsOwner` で設計し、**キー名は #155 の決定に従う**。出所: 同上（仮の名前の字面は AI 起案）
- **R-10 を本人の判定に使わない**。`WorkMonthView` は R-10 を呼ばない（サイドバーの選択中のために `AppSideNav` が R-10 を呼ぶことは M-10 のとおりで、本人の判定とは別である）

決定の前提だった事実（論点の起票の時点。2026-10-30）: 既存の勤務月の表現は技術者の識別子を持たず（契約 AC-10-3 の 10-3-1）、操作者によらず同じ表現を返す（契約 D-12・AC-2-5）。R-10（E-8）はロールが `Approver` の要求に 403 を返す（契約 D-20・AC-12-13）。このため、既存の BFF とドメイン API ではロールが `Approver` のときの ③／④ を区別できなかった。

**BFF の変更は要らない。** 根拠は `bff-wiring.md` の既存の条文である —— 契約の成功は「上流のステータスと、上流の JSON をそのまま返す（値を足さず、削らず、組み替えない）」（AC-8 の 8-7 (i)。D 節 U-2 (i)「応答はそのまま中継する」）。成功の分類は「2xx で本文が JSON として読める」ことだけで、表現のキーを数えない（8-1 (i)）。したがって、ドメイン API が表現に真偽を足せば、BFF は変更なしにそれをブラウザへ渡す。`bff-wiring.md` の検査が表現のキーを固定で列挙しているかは本仕様では読んでおらず、#155 の仕様工程が確かめる（申し送り）。

**キー名の持ち方**（AI 起案。#155 でキー名が決まったときの変更を1か所に留めるため）:

- 字面 `operatorIsOwner` を持つのは `lib/work-month.ts` の定数 `OPERATOR_OWNERSHIP_KEY` **だけ**とする。`apps/web/src` の他のファイル（テストを含む）は定数を輸入して使い、字面を書かない（T-3 がファイル走査で読む）
- `readOwnership(body: unknown): Ownership` —— `body` が object で、`body[OPERATOR_OWNERSHIP_KEY]` が `true` なら `"own"`、`false` なら `"other"`、それ以外（キーが無い・真偽でない・`body` が object でない）なら `"unknown"`。`work-month-client.ts` は `ok` の分類のときにこれを呼び、結果を `ownership` として返す
- #155 が真偽の向き（本人なら真、の逆）を変えて決めた場合も、改めるのは `lib/work-month.ts` の定数と `readOwnership` だけである

**真偽が応答に無いとき**（#155 が入るまで。**AI 起案・レビューで人間が確かめる対象**）:

- `"unknown"` として扱い、ゲスト以外でも**操作要素を1つも出さない**（M-4 の `"unknown"` の行。「提供しない」は非表示＝AC-3 前文）。勤務月の識別・状態・総稼働時間・精算幅・超過／不足・カレンダーは出す（取得の失敗として扱わない）
- 安全側である理由: ①〜④ のどれかに推測で寄せると、出してはならない操作（②への入力・締め、④への承認・差戻し）を出しうる。出さない側は AC-3 の「提供しない」と同じ見え方であり、誤って提供することが無い
- R-10 で補う形（ロールが `Engineer` のときだけ R-10 で ①／② を判定する）は採らない。Q-12 = (b) は R-10 を本人の判定に使わない選択であり、補う形は #155 が入った後に消すコードになる
- **帰結**: #155 がドメイン API に入るまで、本物の環境では画面3 はどのロールでも操作要素を出さず、閲覧だけになる。画面2 の初回入力の導線（W-4）・`/this-month`（W-7）の行き先でも入力できない。AC-3 の条文は変わらず、真偽が届けば画面の変更なしに満たす
- これを停止に当たる業務判断としなかった理由: 要求（AC-3）を緩めず、既存の画面（画面1・画面2）の挙動を変えず、画面3 の規則も起票の時点から置いていた「本人かが分からないうちは操作を出さない」と同じだからである。ただし、**#55 を #155 より先にマージ・デプロイしてよいか**（上の帰結を受け入れるか）は人間の判断であり、レビューで確かめる対象として「起案のまま残るもの」に挙げる

本節のその他（AI 起案）:

- 取得中・取得の失敗・見つからないとき（`loaded` でない）は `"unknown"` と同じに扱う
- ゲストは本人かを使わない（⑤は本人かによらず操作が無い）。真偽を読んでも M-4 の結果は変わらない

### M-4. 操作の出し分け（`lib/work-month-actions.ts`）

`workMonthActions({ state, generated, operator })` が4つの操作の可否を返す純粋な関数である。表は AC-3 のマトリクスをそのまま入出力にしたもので、要求は AC-3・AC-5 が持つ。

| `operator` | 状態 | 入力・編集・削除 | 締め | 承認 | 差戻し | 根拠 |
|---|---|---|---|---|---|---|
| ゲスト | 全状態 | — | — | — | — | AC-3 ⑤ |
| 本人か `"unknown"`（ロールを問わない。真偽が応答に無いときを含む） | 全状態 | — | — | — | — | M-3（AI 起案） |
| `Engineer`・`"own"`（①） | `Draft` | 出す | `generated` が真のときだけ出す | — | — | AC-3 ①、締め UI 1-1・5-1・5-2、契約 AC-6 の申し送り |
| `Engineer`・`"other"`（②） | `Draft` | — | — | — | — | AC-3 ② |
| `Approver`・`"other"`（③） | `Draft` | — | — | — | — | AC-3 ③ |
| `Approver`・`"own"`（④） | `Draft` | 出す | `generated` が真のときだけ出す | — | — | AC-3 ④ |
| ① | `PendingApproval` | — | — | — | — | AC-3 ①・5-1、締め UI 1-3・4-1 |
| ② | `PendingApproval` | — | — | — | — | AC-3 ② |
| ③ | `PendingApproval` | — | — | 出す | 出す | AC-3 ③、承認 UI 1-1・3-2 |
| ④ | `PendingApproval` | — | — | — | — | AC-3 ④、承認 UI 3-1 |
| ①〜④ | `Approved` | — | — | — | — | AC-3、5-2、承認 UI 1-4・4-2 |

- **未生成（`generated` が偽）の月に締めを出さない**のは、契約 AC-6 の申し送り・`bff-wiring.md` 6-5 による（画面だけが担う）。入力は出す（初回の入力で暗黙生成される＝入力 UI 2-3）。未生成の月の状態は `Draft` である（契約 AC-2-2）
- 空月（`generated` が真で稼働実績が0件）には締めを出す（締め UI 5-2）。**ハンドオフ 484 行の「稼働実績が1件以上のときだけ締めを出す」は採らない**（締め UI 5-2 と食い違うため）
- **ハンドオフ 396 行の「ロールが `Engineer` のときだけ入力できる」は採らない**（AC-3 ④ `Draft` と食い違うため）
- 出し分けは画面の表示であり認可ではない。弾く判定はドメイン API が持つ（契約 AC-4〜AC-8。`bff-wiring.md` 6-5）

### M-5. 画面の要素と文言（`work-month-view.tsx`・`work-month-summary.tsx`）

並びはハンドオフに倣う —— (ゲストの注記) → 見出しと状態バッジ → 注記 → 総稼働時間・精算幅・超過／不足 → (カレンダー ／ 入力の区画・締めの区画・承認の区画)。ハンドオフの現在地表示（29〜35 行）は採らない（8-6）。

| 要素 | 部品 | 文言・内容 | 出所 | AC |
|---|---|---|---|---|
| ゲストの注記 | `role="note"` | 「デモ用のサンプルです（閲覧のみ・毎日リセットされます）」。ゲストのときだけ | ハンドオフ 25 行 | 一覧 UI 3-2・3-5 |
| 見出し | `<h1>` | 「`<契約表示名>` × `<対象年月>`」。対象年月は `formatYearMonth`（W-8） | ハンドオフ 463 行 | 1-1 |
| 状態バッジ | `StatusBadge` | 表現の `state` | ハンドオフ 38〜40 行 | 1-2・AC-2 |
| 注記 | `<p>` | `workMonthNote`（M-11）が返す1文。編集できる → 「下書きの勤務月です。カレンダーの日を選んで、開始・終了・休憩を入力してください。」／締めた直後で `PendingApproval` → 「締めました。承認されるまで稼働実績は編集できません。」／ゲスト → 「ゲストとして閲覧しています。」／`Approved` → 「承認済の勤務月です。」／承認を出す → 「技術者が締めた勤務月です。」／`Approver`・`"own"` → 「ご自身の勤務月のため、承認・差戻しはできません。」／`PendingApproval` → 「締め済のため編集できません。」／それ以外 → 「下書きの勤務月です。」（この順に最初に当たったもの） | ハンドオフ 433〜439 行 | — |
| 総稼働時間 | `Card` | 見出し「総稼働時間」、値 `formatHours(totalHours)` と単位「時間」 | ハンドオフ 49・50 行 | 1-3・4-1〜4-3 |
| 精算幅 | `Card` | 見出し「精算幅」、値「`formatHours(lowerBound)`〜`formatHours(upperBound)`」と単位「時間」。表現の `settlementRange` をそのまま使う | ハンドオフ 57・58 行 | 1-4 |
| 超過／不足（`Draft`） | `Card` | 見出し「超過／不足」、文「締め時に確定します」。数値を出さない | ハンドオフ 66・67 行 | 4-1・4-5 |
| 超過・不足（`PendingApproval`・`Approved`） | `Card` | 見出し「超過」「不足」、それぞれ `formatHours` と単位「時間」。**0 でない側の数値と見出しの文字に `text-danger`**（`--danger`）を付け、0 の側・総稼働時間・精算幅・「締め時に確定します」には付けない | ハンドオフ 77・81 行／4-6 | 4-2・4-3・4-6 |
| カレンダー | `Card` と `work-month-calendar.tsx` | M-6 | ハンドオフ 94 行 | 1-6 |
| 実績0件の空状態 | `EmptyState` | 入力・編集を出し、稼働実績が0件で日を選んでいないとき。title「まだ稼働実績がありません」、description「カレンダーの日を選んで、稼働時間を入力してください。最初の入力でこの勤務月が作られます。」、action は `Button`（`variant="primary"`）「稼働実績を入力する」で、押すと `firstInputDay`（M-6）の日を選ぶ | ハンドオフ 142・454・455 行 | 入力 UI 2-1・2-2 |
| 入力の案内 | `Card` | 入力・編集を出し、稼働実績が1件以上で日を選んでいないとき。見出し「稼働実績の入力」、文「カレンダーの日を選ぶと、ここで稼働時間を入力・編集・削除できます。」 | ハンドオフ 149・150・467 行 | — |
| 入力フォーム | `daily-record-form.tsx` | 入力・編集を出し、日を選んでいるとき（M-7） | ハンドオフ 155〜243 行 | 入力 UI AC-3〜AC-6 |
| 締めの区画 | `CloseAction` | M-8 | ハンドオフ 251〜255 行 | 締め UI AC-1・AC-2 |
| 承認の区画 | `ReviewActions` | M-8 | ハンドオフ 266〜271 行 | 承認 UI AC-1・AC-10 |
| 失敗の表示 | `ErrorBanner`・`EmptyState` | M-9 | — | AC-6 |

- **時間の表記** `formatHours({ hours, minutes })`: `hours + minutes / 60` を小数第2位までに丸め（`Math.round(x * 100) / 100` を `toFixed(2)`）、末尾が `0` なら1字だけ落とす。例: `{9,30}` → `9.5`、`{10,0}` → `10.0`、`{7,45}` → `7.75`、`{0,15}` → `0.25`、`{180,15}` → `180.25`。出所: ハンドオフ 297 行（`fmt`）
- **時刻の表記** `formatTimeOfDay({ hour, minute })` → `9:00`・`17:45`（時は0で埋めない、分は2桁）。範囲は「`開始`〜`終了`」。出所: ハンドオフ 316・413 行
- 「誰が締めた・承認したか」・差戻しの理由と回数・精算金額を出さない（締め UI 7-2・7-3、承認 UI 4-4・5-3・5-4。表現が持たない＝契約 AC-10-3）

### M-6. カレンダーと祝日（`work-month-calendar.ts`・`work-month-calendar.tsx`）

- 見出し「日次の稼働実績」（ハンドオフ 94 行）。曜日の見出しは「月」「火」「水」「木」「金」「土」「日」の7つ（ハンドオフ 96 行。週の始まりは月曜＝入力 UI 1-5）
- `calendarCells({ yearMonth, records, isHoliday, today })` がマスの列を返す。先頭に「1日の曜日の月曜からの日数」個の空きマス（ハンドオフ 403 行）、続けて1日〜末日、末尾に7の倍数になるまで空きマスを置く
- 日のマスは `mark` を持つ: 祝日なら `hol`（土曜・日曜でも優先する＝入力 UI 9-3）、そうでなく土曜なら `sat`、日曜なら `sun`、ほかは空文字。描画は要素の `data-dow` 属性にこの値を出し（空文字なら属性を出さない）、色は `design-system.md` 2-1-x〜2-1-ab のトークンを当てる（出所: ハンドオフ 411 行の `data-dow`）
- 祝日のマスには札「祝」を出す（入力 UI 9-8。ハンドオフ 104 行）。祝日の判定は `isHoliday` だけを引く（入力 UI 10-2）。`calendarCells` と `WorkMonthView` は `isHoliday` を引数・props で受け、`lib/holidays/` を輸入しない（M-1）。本物の `isHoliday` は `lib/holidays/` が持ち、`work-month-holiday-view.tsx` が渡す（データの用意は M-12。2026-10-30 人間の明示選択（Q-15 = (a)）。祝日の判定を引数で差し替える形は AI 起案のまま保つ）。テストは `isHoliday` を引数で差し替える
- 実績のある日は、`formatHours(workingHours)` と「`formatTimeOfDay(startTime)`〜`formatTimeOfDay(endTime)`」を出す（入力 UI 1-1・1-6。ハンドオフ 107・108 行）。休憩の時刻は出さない。各日の `workingHours` は時刻の粒度により `roundedWorkingHours` と一致し、日の値の和が総稼働時間と一致する（入力 UI 1-2・契約 D-5。テストは表現の例で和を読む）
- **編集できるとき**（M-4 の入力・編集が出る）: 日のマスは `button`（`aria-pressed` が選んだ日なら `true`）で、アクセシブル名は `dayLabel`（下記）。押すとその日を選ぶ。未来の日も選べる（選ぶとフォームが未来の日の理由を出す＝M-7）
- **編集できないとき**: 日のマスは `button` にせず、押せない要素で同じ内容を出す（ハンドオフ 116〜133 行）
- `dayLabel(cell)`: 「`M`月`D`日（`曜日`）」に、実績があれば「 `formatHours` 時間（`範囲`）」、無ければ「 未入力」、未来の日ならさらに「（未来の日付）」を続ける。祝日は曜日の代わりに「祝」。例: 「10月2日（金） 9.75 時間（9:00〜19:45）」「10月12日（祝） 未入力」。出所: ハンドオフ 400・410 行
- `firstInputDay(cells)`: 祝日・土曜・日曜でなく未来でもない最初の日。無ければその月の1日。出所: ハンドオフ 454 行
- 未来の日の判定は `date > today`（文字列の比較。`today` は `currentJstDate(new Date())`。入力 UI 4-2 の「JST 当日より後」）

### M-7. 入力フォーム（`daily-record-form.tsx`・`time-entry.ts`）

| 要素 | 内容 | 出所 | 入力 UI |
|---|---|---|---|
| 見出し | 「稼働実績の入力」 | ハンドオフ 159 行 | — |
| 対象日 | ラベル「対象日」と、選んだ日の「`M`月`D`日（`曜日`）」（祝日は「祝」） | ハンドオフ 161・400 行 | 3-1 |
| 未来の日 | 選んだ日が未来なら、`FieldError`「未来の日付には入力できません」を出し、時刻の選択・休憩・保存・削除を出さず、「閉じる」だけを出す | ハンドオフ 164・236〜240 行 | 4-2（時刻の選択と保存を出さずに理由だけを示す形を選んだ） |
| 時刻の選択 | 「勤務時間」の区切り（`fieldset`）に「開始」「終了」、「休憩」の区切りに「休憩開始」「休憩終了」。それぞれ時と分の `SelectField` 2つ。アクセシブル名は「開始（時）」「開始（分）」「終了（時）」…「休憩終了（分）」の8つ（`hideLabel`）。未選択の表示 `placeholder` は「--」（値は空文字） | ハンドオフ 169・175・192・317・318・428・472・473 行 | 3-1・3-2・3-2-b・3-2-c |
| 選択肢 | 時は `daily-record-entry.md` D-10 の時の値域の昇順、分は同 D-10 の4値の昇順。表示は時が数字そのまま、分が2桁（`00`・`15`・`30`・`45`）。値域は本仕様へ書き写さず、`lib/time-entry.ts` の定数 `HOUR_OPTIONS`・`MINUTE_OPTIONS` が持つ | ハンドオフ 317・318 行 | 3-2-a |
| 休憩の有無 | チェックボックス「休憩あり」。外すと休憩開始・休憩終了の選択を描かず、送る値の `break` を `null` にする | ハンドオフ 195 行 | 3-1-a・3-4 |
| 保存前の稼働時間 | `aria-live="polite"` の領域に、ラベル「稼働時間」、値 `formatHours(previewWorkingHours(form))` と単位「時間」。値が無いときは「—」。値があるときの補足は「`開始`〜`終了`（休憩 `H:MM`）」または「`開始`〜`終了`（休憩なし）」、無いときは「開始・終了・休憩から自動で計算します」 | ハンドオフ 220〜226・349・475・476 行 | 3-6・3-6-a・3-6-b・3-6-f |
| 案内 | 「時は 0〜23、分は 15 分単位で選んでください。」 | ハンドオフ 227 行 | — |
| 操作 | `Button`「保存」、実績のある日だけ `Button`「削除」、`Button`「閉じる」（選んだ日を外す） | ハンドオフ 229〜233 行 | 3-3・AC-5・AC-6 |

- **保存前の稼働時間は `computeWorkingHours` の戻り値だけから得る**（入力 UI 3-6-f）。`previewWorkingHours(form)` は、未選択が1つでも残るなら関数を呼ばずに `null`、そうでなければ `computeWorkingHours` の戻り値をそのまま返す。補足の休憩の長さ（`H:MM`）は休憩終了と休憩開始の差で、稼働時間ではない
- **弾く理由**（入力 UI 4-1・4-5〜4-9。互いに異なる文）。`timeEntryProblems(form, tried)` が返し、`FieldError` を該当の行の下に出し、その行の `SelectField` に `invalid` と `errorId` を渡す。出所: ハンドオフ 333・343〜348 行

| 理由 | 出す行 | 文言 | 出す時機 |
|---|---|---|---|
| 時と分がどちらも未選択 | その行 | 「時刻を選んでください」 | 保存を押した後（`tried`） |
| 時だけ未選択 | その行 | 「時を選んでください」 | 同上 |
| 分だけ未選択 | その行 | 「分を選んでください」 | 同上 |
| 終了が開始より後でない | 終了 | 「終了は開始より後の時刻にしてください」 | 開始と終了が選ばれた時点から |
| 休憩終了が休憩開始より後でない | 休憩終了 | 「休憩の終了は休憩の開始より後にしてください」 | 休憩ありで休憩の両時刻が選ばれた時点から（上の行に当たらないとき） |
| 休憩が開始〜終了に収まらない | 休憩終了 | 「休憩は勤務時間（開始〜終了）の中に収めてください」 | 4つすべて選ばれた時点から（上の2行に当たらないとき） |
| 稼働時間が 0 になる | 休憩終了 | 「稼働時間が 0 になります。休憩の時刻を見直してください」 | 4つすべて選ばれた時点から（上の3行に当たらないとき） |

  - 組み合わせの4つの理由は、4つの時刻（休憩なしなら2つ）がすべて選ばれているとき、**理由が1つも無いことと `computeWorkingHours` が `null` を返さないことが一致する**（AI 起案の不変条件。T-3 が `daily-record-entry.md` AC-3 の 3-B・3-C の表で読む）
- **保存**: 未選択が残るなら `tried` を立てて送らない（4-1）。理由が1つでもあれば送らない。どちらも入力した値を保つ（4-4）。それ以外は `readTimeEntry(form)` の `input`（`startTime`・`endTime`・`break`。稼働時間を含めない＝3-6-c）を R-5 へ送る
- **編集**（入力 UI 5-1）: 実績のある日を選ぶと `formFromRecord(record)` を初期値にする（休憩が `null` なら「休憩あり」を外す）。新しい日の初期値は `NEW_TIME_ENTRY_FORM`（開始・終了は未選択、休憩あり 12:00〜13:00）。出所: ハンドオフ 315 行（入力 UI 3-5 は初期値を固定しない）
- 保存の成功の後も同じ日を選んだままにし、フォームは保存した値のまま `tried` を戻す。削除の成功の後は `NEW_TIME_ENTRY_FORM` へ戻す。出所: ハンドオフ 480・482 行
- 削除に確認ステップを置かない（入力 UI AC-6 が要求せず、ハンドオフ 231 行も置かない）

### M-8. 締め・承認・差戻し（`work-month-operations.tsx`）

| 要素 | 文言・内容 | 出所 | AC |
|---|---|---|---|
| 締めの区画 | `Card`。見出し「勤務月を締める」、文「入力が済んだら締めてください。締めると承認されるまで編集できません。」、`Button`（`primary`）「締める」 | ハンドオフ 251・252・255 行 | 締め UI 1-1・1-4・5-3（未入力の警告を出さない） |
| 締めの確認 | `ConfirmDialog`。`title`「`<YYYY>`年`<M>`月の勤務月を締めますか？」、`description` は2段落「締めると稼働実績は編集できなくなり、ご自身で取り消すことはできません。」「下書きに戻す方法は、承認者による差戻しだけです。」、`confirmLabel`「締める」、`confirmVariant`「primary」 | ハンドオフ 443〜446 行 | 締め UI 2-1 |
| 承認の区画 | `Card`。見出し「承認」、文「内容を確認して、承認または差戻しをしてください。」、`Button`（`danger`）「差戻し」と `Button`（`primary`）「承認」をこの順に | ハンドオフ 266・267・270・271 行 | 承認 UI 1-1 |
| 承認の確認 | 承認 UI 10-9-a の行のとおり（`confirmVariant`「primary」＝10-6）。**ハンドオフ 447 行の文言は採らない**（承認 UI 10-9 が持つ） | 承認 UI 10-9-a | 承認 UI 10-1・10-1-a |
| 差戻しの確認 | 承認 UI 10-9-b の行のとおり（`confirmVariant`「danger」）。理由の入力欄を置かない | 承認 UI 10-9-b | 承認 UI 10-2・10-2-a・10-5・2-1 |

- 確認で承諾すると、締めは R-7、承認は R-8、差戻しは R-9 を**ちょうど1回**送る（本文なし）。取消（部品の取消の操作・Esc）なら何も送らず、状態を変えない（締め UI 2-3、承認 UI 10-4）
- 締めを取り消す・下書きへ戻す操作を、どの状態・区分にも置かない（締め UI AC-4）
- 成功の応答で置き換えると、状態バッジ・超過／不足・出す操作が M-4・M-5 のとおり変わる（締め UI AC-3、承認 UI AC-4・AC-5）。締めの成功の後は「締めた直後」を真にする（注記だけに効く）

### M-9. 操作の結果と失敗

分類と文言は `lib/work-month-feedback.ts`（M-11）が持つ。`WorkMonthView` はその戻り値で `ErrorBanner`・`EmptyState` を描くだけにする。

**操作（保存・削除・締め・承認・差戻し）の結果** —— 弾かれたときの文言は種類ごとに4つの文にまとめる。出所: 2026-10-30 人間の明示選択（Q-14 = (b)）。**4つの文の字面は AI 起案であり、レビューで人間が承認する対象（未承認）である。**

| 結果 | 種類（`OperationFailureKind`） | 画面がすること | 文言（AI 起案・未承認） |
|---|---|---|---|
| `ok` | — | 表現で置き換える（M-2） | — |
| `refresh`（401） | — | `router.refresh()` をちょうど1回。表現を変えない | — |
| `rejected` で `status` が 400（保存の `TIME_OF_DAY_OUT_OF_RANGE`・`INVALID_TIME_COMBINATION`・`FUTURE_DATE_NOT_ALLOWED`・`DATE_OUT_OF_WORK_MONTH`・`INVALID_REQUEST`。`code` を問わない） | `invalid-input` | 理由を出し、入力中の値を保つ（6-1・6-2、入力 UI 4-1-a・4-3・4-4）。表示値を変えない（入力 UI 4-10） | 「入力した内容を受け付けられませんでした。内容を見直して、もう一度保存してください。」 |
| `rejected` で 403・404（`code` を問わない） | `not-allowed` | 理由を出す。表示値を変えない | 「この勤務月では、この操作はできません。」 |
| `rejected` で 409（`code` を問わない） | `state-changed` | 理由を出し、R-1 を取り直して置き換える（6-4、締め UI 6-1、承認 UI 10-8。M-2）。入力中の値は保つ | 「勤務月の状態が変わっていたため、最新の内容に更新しました。」 |
| `failed`（429・5xx・通信の失敗・契約の形でない応答） | `communication` | 失敗を出す。表示値を変えず、締め・承認が成立したように見せない（6-3）。自動で送り直さない | 「通信に失敗しました。時間をおいて、もう一度お試しください。」 |

- 4つの文は `ErrorBanner`（`role="alert"`）の本文として出し、`onRetry` を渡さない（操作の送り直しは利用者が同じ操作をもう一度押すことで行う。自動で送り直さない＝6-3）。AI 起案
- 次の操作を始めたら消す（M-2）。409 の文は、取り直しが `ok` のときだけ出したまま残す。取り直しが `ok` でなければ、下の「勤務月の取得の結果」の表示に替わる（M-2。AI 起案）
- 4つの文は互いに異なり、M-7 の事前の検査の7つの文とも異なる（入力 UI 4-9 の趣旨。AI 起案）
- 契約は `INVALID_TIME_COMBINATION` の理由を区別しない（契約 AC-11-11）。画面が理由を区別して示すのは M-7 の事前の検査による。400 を返されたときの文は種類で1つである（Q-14 = (b) の帰結）

**勤務月の取得（R-1）の結果** —— 出所: 2026-10-30 人間の明示選択（Q-13 = (a)）。**`EmptyState` の文言は AI 起案であり、レビューで人間が承認する対象（未承認）である。**

| 結果 | 種類（`LoadFailureKind`） | 画面がすること | 文言 |
|---|---|---|---|
| `ok` | — | 表現を示す | — |
| `refresh`（401） | — | `router.refresh()` をちょうど1回 | — |
| `rejected` で 404・400（`CONTRACT_NOT_FOUND`・`INVALID_REQUEST`。`code` を問わない） | `missing` | 1つの `EmptyState` にまとめ、区分の既定の画面（C-4 の `defaultScreenPath(viewer)`）へのリンクを `action` に置く。再試行を出さない。見出し（契約表示名 × 対象年月）・状態バッジ・カレンダーを出さない | title「勤務月を表示できません」、description「契約が見つからないか、URL の形式が正しくありません。」、リンク「勤務月の一覧を開く」→ `defaultScreenPath(viewer)`（**AI 起案・未承認**） |
| `rejected` で 403・409、または `failed` | `failed` | `ErrorBanner`（`onRetry` 付き）。再試行で R-1 を再び GET する。見出し・状態バッジ・カレンダーを出さない | 「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」（W-4 の取得エラーの文言を流用。出所: Q-13 = (a)） |

- R-1 の 403・409 は契約の E-1 が返す組に当たらないため、取得の失敗として扱う（AI 起案）
- リンクの行き先は `defaultScreenPath(viewer)`（技術者は本人一覧、承認者は承認待ち一覧、ゲストはデモ勤務月群＝`login-ui.md` 6-7-a）。3つとも勤務月の一覧であるため、リンクの文言は区分によらず1つとする（AI 起案）。`screen-access.ts` は純粋であり（C-4）、クライアントの `WorkMonthView` から輸入してよい
- `/this-month` に存在しない契約 ID を付けたとき（W-7）と、勤務月画面の URL を直接入力したとき（M-10）は、どちらもこの表のとおりになる

### M-10. 他人の契約・未生成の当月・サイドバーの選択中・ゲスト

**他人の契約の勤務月を開いたとき**（W-7 の申し送り。2026-10-30 人間の明示選択（Q-11 = (e)）が求めた条文）:

- `/this-month?contract=<他人の契約>` を経た場合も、勤務月画面の URL を直接入力した場合も、**同じ経路**（`workMonthScreen` → `WorkMonthView`）を通る。区別しない
- **応答は既存の条文で決まっている**: R-1 は任意の `contractId` を受け（`bff-wiring.md` 13-4）、E-1 は操作者で絞らず、契約が実在すれば 200 で勤務月の表現を返す（契約 D-12・AC-2-5。未生成なら `generated: false`＝契約 AC-2-2）。これは人間が確定した仕様である（契約 AC-11-3）
- **画面の扱いも既存の条文で決まっている**: 操作者は ②他の技術者（ロールが `Approver` なら③）であり、操作要素を1つも出さない（AC-3 ②・③ `Draft` 行。`PendingApproval` の③は承認・差戻しを出す）。勤務月の識別・状態・総稼働時間・精算幅・超過／不足・カレンダーは閲覧できる（1-1〜1-6・5-3）
- **画面が ②・③ と判定する手段は M-3 による**（表現の真偽が偽。2026-10-30 人間の明示選択（Q-12 = (b)））。真偽が応答に無いあいだは `"unknown"` であり、やはり操作要素を出さない（M-3。AI 起案）
- 状態を変える要求は、操作要素を出さないので画面からは送られない。送られた場合はドメイン API が 403 で弾く（契約 AC-4-3・AC-5-4・AC-6-3。T-3 の限界）
- 実在しない契約（404）・書式の不正な年月（400）は M-9 の「勤務月の取得の結果」の `missing`

**未生成の当月を開いたとき**（一覧 UI 10-9-e）: R-1 が `generated: false` の表現を返す（契約 D-7）。画面は空状態と同じに見せ（入力 UI 2-2。本人で編集できれば M-5 の実績0件の空状態）、**勤務月を生成する要求を送らない**（開いただけでは R-5〜R-9 を呼ばない）。「月を開く」等の操作を置かない。締めを出さない（M-4）。最初の保存（R-5）の応答は `generated: true` になり、以降は締めを出す（入力 UI 2-3）

**サイドバーの選択中**（一覧 UI 10-5 の後半。W-6 が本節へ委ねた）: `workMonthSideNavCurrent({ viewer, contractId, yearMonth, ownContracts, now })` が `SideNavCurrent` を返す。`AppSideNav` は `current` が `WorkMonthLocation` のとき、自分で取った R-10 の結果（技術者のとき）と描画の時点の `new Date()` でこの関数を呼ぶ。

| `viewer` | 条件 | 戻り値 |
|---|---|---|
| ゲスト | — | `demo-work-months` |
| 承認者 | — | `pending-approvals` |
| 技術者 | `ownContracts` に `contractId` があり、かつ `yearMonth` が `currentJstYearMonth(now)` と等しい | `this-month` |
| 技術者 | それ以外（他人の契約・当月でない・`ownContracts` が `undefined`＝取得中か取得の失敗） | `own-work-months` |

- 取得中・取得の失敗を `own-work-months` とするのは AI 起案（取得の後に `this-month` へ変わりうる）。開いたまま JST の月の境目を越えても描き直さない（W-6 の Q-10 と同じ。限界）

**ゲストの閲覧**（1-8・AC-3 ⑤、一覧 UI 3-2）: ゲストの `WorkMonthView` は R-1 だけを呼び（`WorkMonthView` はどの区分でも R-10 を呼ばない＝M-3。ゲストの `AppSideNav` も R-10 を呼ばない）、操作要素を1つも出さない。ゲストの注記（M-5）を出す。ヘッダーの右側は L-3 の `app` の行（テーマ切替 → ログインへのリンク）

### M-11. 画面3 の関数の形と props（AI 起案）

T-3 の表が求めるテストを書ける粒度で、輸出する名前と型を定める。**本節はすべて AI 起案であり、人間の承認前である。** `Ownership` を勤務月の表現の真偽から得ることは 2026-10-30 人間の明示選択（Q-12 = (b)）による。

```ts
// lib/work-month.ts（純粋）。キー名は契約 AC-10-1（OPERATOR_OWNERSHIP_KEY を除く）
import type { Break, TimeOfDay, WorkingHours } from "./working-hours";
import type { WorkMonthState } from "./work-month-list";
export type Ownership = "own" | "other" | "unknown";
// 本人かの真偽のキー名。仮の名前であり、#155 の決定に従う。字面を持つのはここだけ（M-3）
export const OPERATOR_OWNERSHIP_KEY = "operatorIsOwner";
export function readOwnership(body: unknown): Ownership; // M-3。true → "own"、false → "other"、それ以外 → "unknown"
export interface DailyRecord {
  date: string;                       // YYYY-MM-DD
  startTime: TimeOfDay;
  endTime: TimeOfDay;
  break: Break | null;
  workingHours: WorkingHours;
  roundedWorkingHours: WorkingHours;
}
export interface WorkMonthDetail {
  contractId: string;
  contractDisplayName: string;
  yearMonth: string;                  // YYYY-MM
  state: WorkMonthState;
  generated: boolean;
  settlementRange: { lowerBound: WorkingHours; upperBound: WorkingHours };
  totalHours: WorkingHours;
  excess: WorkingHours | null;
  shortfall: WorkingHours | null;
  dailyRecords: DailyRecord[];
  // 本人かの真偽は WorkMonthDetail に載せない。readOwnership が本文から読み、WorkMonthResult の ownership に置く
}
export function formatHours(value: WorkingHours): string;                   // M-5
export function formatTimeOfDay(time: TimeOfDay): string;                   // "9:00"
export function formatTimeRange(start: TimeOfDay, end: TimeOfDay): string;  // "9:00〜18:00"
```

```ts
// lib/work-month-client.ts（純粋。ブラウザで動く）
export type WorkMonthResult =
  | { kind: "ok"; value: WorkMonthDetail; ownership: Ownership } // ownership は readOwnership(本文)
  | { kind: "refresh" }
  | { kind: "rejected"; status: number; code: string }
  | { kind: "failed" };
export interface DailyRecordInput { startTime: TimeOfDay; endTime: TimeOfDay; break: Break | null } // 契約 AC-4 の本文
export function workMonthApiPath(contractId: string, yearMonth: string): string; // "/api" + E-1 のパス。各セグメントを encodeURIComponent
export function fetchWorkMonth(contractId: string, yearMonth: string, fetchImpl?: typeof fetch): Promise<WorkMonthResult>;   // R-1。GET
export function saveDailyRecord(contractId: string, yearMonth: string, date: string, input: DailyRecordInput, fetchImpl?: typeof fetch): Promise<WorkMonthResult>; // R-5。PUT
export function deleteDailyRecord(contractId: string, yearMonth: string, date: string, fetchImpl?: typeof fetch): Promise<WorkMonthResult>; // R-6。DELETE
export function closeWorkMonth(contractId: string, yearMonth: string, fetchImpl?: typeof fetch): Promise<WorkMonthResult>;   // R-7。POST
export function approveWorkMonth(contractId: string, yearMonth: string, fetchImpl?: typeof fetch): Promise<WorkMonthResult>; // R-8。POST
export function rejectWorkMonth(contractId: string, yearMonth: string, fetchImpl?: typeof fetch): Promise<WorkMonthResult>;  // R-9。POST
```

- パスは `bff-wiring.md` 6-7 (i)（`/api` ＋ 契約の E のパス）。R-5・R-6 は `workMonthApiPath(...) + "/daily-records/" + encodeURIComponent(date)`、R-7〜R-9 は `+ "/close"`・`"/approve"`・`"/reject"`
- R-5 だけが本文を持つ。本文は `JSON.stringify(input)` で、キーは `startTime`・`endTime`・`break` の3つだけ（`break` を省かない＝契約 D-18。稼働時間を含めない）。`Content-Type` は `application/json`。ほかは本文を持たない
- 分類は M-2 の表。`fetchImpl` を省いたときは呼び出しの時点の `globalThis.fetch` を使う（W-8 と同じ）
- `value` は本文から `WorkMonthDetail` の型が持つキーを読んだものとし、真偽のキーの有無で `ok` か否かを変えない（M-3）

```ts
// lib/work-month-feedback.ts（純粋）。M-9 の2つの表
import type { WorkMonthResult } from "./work-month-client";
export type OperationFailureKind = "invalid-input" | "not-allowed" | "state-changed" | "communication";
export type LoadFailureKind = "missing" | "failed";
type NotOk = Extract<WorkMonthResult, { kind: "rejected" } | { kind: "failed" }>;
export const OPERATION_FAILURE_MESSAGES: Readonly<Record<OperationFailureKind, string>>; // M-9 の4つの文
export function operationFailureKind(result: NotOk): OperationFailureKind;               // M-9 の操作の表
export function loadFailureKind(result: NotOk): LoadFailureKind;                         // M-9 の取得の表
export const WORK_MONTH_LOAD_ERROR_MESSAGE: string;   // W-4 の取得エラーの文言と同じ字面
export const WORK_MONTH_MISSING: Readonly<{ title: string; description: string; linkLabel: string }>; // M-9 の EmptyState
```

- 文言の字面はこのファイルだけが持つ（画面2 の各ビューは W-4 の文言をそれぞれのファイルの定数で持っており、本仕様はそれを動かさない）

```ts
// lib/work-month-actions.ts（純粋）
import type { Role } from "./role-cookie";        // import type
import type { Ownership } from "./work-month";   // import type（M-3）
export type Operator = { kind: "guest" } | { kind: "signed-in"; role: Role; ownership: Ownership };
export interface WorkMonthActions { edit: boolean; close: boolean; approve: boolean; reject: boolean }
export function workMonthActions(params: { state: WorkMonthState; generated: boolean; operator: Operator }): WorkMonthActions; // M-4 の表
export function workMonthNote(params: { state: WorkMonthState; operator: Operator; actions: WorkMonthActions; justClosed: boolean }): string; // M-5 の注記
```

```ts
// lib/work-month-calendar.ts（純粋）
export type DayMark = "sat" | "sun" | "hol" | "";
export type CalendarCell =
  | { kind: "blank" }
  | { kind: "day"; date: string; day: number; weekday: string; mark: DayMark; holiday: boolean; future: boolean; record: DailyRecord | undefined };
export const WEEKDAY_HEADERS: readonly string[]; // ["月","火","水","木","金","土","日"]
export function calendarCells(params: {
  yearMonth: string;                          // YYYY-MM（表現の yearMonth）
  records: readonly DailyRecord[];
  isHoliday: (date: string) => boolean;
  today: string;                              // YYYY-MM-DD（JST の当日）
}): CalendarCell[];
export function dayLabel(cell: Extract<CalendarCell, { kind: "day" }>): string; // M-6
export function firstInputDay(cells: readonly CalendarCell[]): string;         // M-6。YYYY-MM-DD

// lib/jst-month.ts（既存へ追加）
export function currentJstDate(now: Date): string; // YYYY-MM-DD

// lib/holidays/data.ts（新規。M-12 の段。原本から変換して生成する。M-12）
export interface HolidayRow { date: string; name: string } // date は YYYY-MM-DD、name は原本の名称
export const HOLIDAY_ROWS: readonly HolidayRow[];          // 原本の全行を、原本の行の順で

// lib/holidays/index.ts（新規。M-12 の段）
export function isHoliday(date: string): boolean; // date は YYYY-MM-DD。HOLIDAY_ROWS の date に一致すれば真（入力 UI 10-5）
```

- `weekday` は「月」〜「日」（祝日でも曜日の字を持つ。`dayLabel` が祝日を「祝」に替える）

```ts
// lib/time-entry.ts（純粋）
export interface TimeSelection { hour: string; minute: string }   // 空文字が未選択
export interface TimeEntryForm {
  start: TimeSelection; end: TimeSelection;
  hasBreak: boolean; breakStart: TimeSelection; breakEnd: TimeSelection;
}
export type TimeField = "start" | "end" | "breakStart" | "breakEnd";
export type TimeEntryProblemKind =
  | "unselected-both" | "unselected-hour" | "unselected-minute"
  | "end-not-after-start" | "break-end-not-after-start" | "break-outside" | "zero";
export interface TimeEntryProblem { field: TimeField; kind: TimeEntryProblemKind }
export const HOUR_OPTIONS: readonly { value: string; label: string }[];   // D-10 の時の値域（昇順）
export const MINUTE_OPTIONS: readonly { value: string; label: string }[]; // D-10 の分の4値（昇順。label は2桁）
export const NEW_TIME_ENTRY_FORM: TimeEntryForm;
export const TIME_ENTRY_MESSAGES: Readonly<Record<TimeEntryProblemKind, string>>; // M-7 の文言
export function formFromRecord(record: DailyRecord): TimeEntryForm;
export function readTimeEntry(form: TimeEntryForm): { kind: "incomplete" } | { kind: "complete"; input: DailyRecordInput }; // 休憩なしなら break: null
export function timeEntryProblems(form: TimeEntryForm, tried: boolean): TimeEntryProblem[]; // M-7 の表
export function previewWorkingHours(form: TimeEntryForm): WorkingHours | null;              // computeWorkingHours の戻り値だけ
```

- `timeEntryProblems` は1つの行に理由を1つだけ返す（未選択の理由は `tried` が真のときだけ）。休憩なしのとき休憩の行を見ない（3-4）

```ts
// lib/side-nav-items.ts（既存へ追加）
export interface WorkMonthLocation { kind: "work-month"; contractId: string; yearMonth: string }
export function workMonthSideNavCurrent(params: {
  viewer: Viewer; contractId: string; yearMonth: string;
  ownContracts: readonly ContractSummary[] | undefined; now: Date;
}): SideNavCurrent; // M-10 の表
```

```tsx
// app/_components の props
export interface WorkMonthViewBaseProps { viewer: Viewer; contractId: string; yearMonth: string }
export function workMonthScreen(
  deps: ScreenDeps,
  params: { contractId: string; yearMonth: string },
  view: ComponentType<WorkMonthViewBaseProps>,           // 本物は WorkMonthHolidayView（page.tsx が渡す。M-1）
): Promise<JSX.Element>;
export default async function WorkMonthPage(props: { params: Promise<{ contractId: string; yearMonth: string }> }): Promise<JSX.Element>; // 薄い層。M-12 の段で置く
export function AppShell(props: { viewer: Viewer; theme: Theme; current: SideNavCurrent | WorkMonthLocation; children: ReactNode }): JSX.Element; // 既存の型を広げる
export function AppSideNav(props: { viewer: Viewer; current: SideNavCurrent | WorkMonthLocation }): JSX.Element;                               // 既存の型を広げる
export function WorkMonthView(props: WorkMonthViewBaseProps & { isHoliday: (date: string) => boolean }): JSX.Element;        // "use client"。lib/holidays/ を輸入しない
export function WorkMonthHolidayView(props: WorkMonthViewBaseProps): JSX.Element;                                             // "use client"。M-12 の段。<WorkMonthView {...props} isHoliday={isHoliday} /> を返すだけ
export function WorkMonthSummary(props: { workMonth: WorkMonthDetail }): JSX.Element;
export function WorkMonthCalendar(props: {
  cells: readonly CalendarCell[];
  editable: boolean;
  selectedDate: string | null;
  onSelect: (date: string) => void;   // editable が偽のときは呼ばれない
}): JSX.Element;
export function DailyRecordForm(props: {                                                     // "use client"。親が key={date} を渡す
  cell: Extract<CalendarCell, { kind: "day" }>;
  busy: boolean;
  onSave: (input: DailyRecordInput) => void;
  onDelete: () => void;
  onClose: () => void;
}): JSX.Element;
export function CloseAction(props: { yearMonth: string; busy: boolean; onConfirm: () => void }): JSX.Element;                      // "use client"
export function ReviewActions(props: { busy: boolean; onApprove: () => void; onReject: () => void }): JSX.Element;                 // "use client"
```

- `DailyRecordForm` の初期値は `cell.record` があれば `formFromRecord`、無ければ `NEW_TIME_ENTRY_FORM`。`cell.future` が真なら M-7 の未来の日の形で描く。削除は `cell.record` があるときだけ出す
- `busy` が真の間、保存・削除・締め・承認・差戻しの `onX` を呼ばない
- `WorkMonthView` の `viewer` はサーバーが解決した `Viewer`（`import type` に限る＝C-1）。`router.refresh()` は `next/navigation` の `useRouter` から取る。`viewer` の区分かロールが変わったら R-1 を取り直す（M-2）。本人かは `ok` の `ownership` だけから得る（M-3）
- `WorkMonthView` の「勤務月を表示できません」のリンクの行き先は `defaultScreenPath(viewer)`（C-4。M-9）
- `WorkMonthHolidayView` の型を `workMonthScreen` の `view` に合わせるため、`WorkMonthViewBaseProps` は `work-month-view.tsx` から輸出し、本体は `import type` で受ける（本体が `WorkMonthView` を値として輸入しない）
- `params` の値は Next が渡すまま本体へ渡し、本体も画面も書式を検査しない（書式の判定はドメイン API＝`bff-wiring.md` 7-3）

### M-12. 祝日データの同梱（画面3 の最後の段）

**決定**: 人間が原本（内閣府の `syukujitsu.csv`）を用意する。置き場所は `/workspaces/effort-tracker/syukujitsu.csv` の予定（2026-10-30 の時点では、まだ置かれていない）。祝日のデータと `lib/holidays/` は画面3 の**最後の段**に回す。原本はコミットせず、入力 UI 10-3 の形式へ変換したものだけを同梱する。出所: 2026-10-30 人間の明示選択（Q-15 = (a)）。本節のそれ以外は AI 起案である。

```mermaid
flowchart LR
  S1["段1: 祝日データに依存しないもの<br/>lib（work-month・client・feedback・actions・calendar・time-entry ほか）<br/>部品・WorkMonthView・workMonthScreen<br/>T-3 の「段1」の行"] --> Gate{"原本が置き場所にあるか"}
  Gate -->|"ある"| S2["段2（最後）: 祝日データ<br/>.gitignore・lib/holidays/・WorkMonthHolidayView・page.tsx<br/>T-3 の「段2」の行"]
  Gate -->|"無い"| Stop["段2 に入らず停止を申告する"]
```

- **段の境界**: 段1 のファイルとテストは `lib/holidays/` を輸入しない（M-1・M-6）。段2 で足すのは、`.gitignore` の1行、`lib/holidays/data.ts`・`lib/holidays/index.ts`、`app/_components/work-month-holiday-view.tsx`、`app/work-months/[contractId]/[yearMonth]/page.tsx` と、T-3 の「段2」の行のテストである。勤務月画面のルートは段2 で初めて開く（段1 の間も、画面2 の勤務月画面へのリンクと `/this-month` の行き先は、いまと同じく 404 のままである。ブランチの中だけの途中の状態である）
- **原本が無いとき**: 段2 に入らず、停止を申告する（AI は原本を取得できない＝入力 UI 10-7 の手順1）
- **原本をコミットしない**: 段2 の最初に、ルートの `.gitignore` へ `/syukujitsu.csv` を足す（原本を置いた作業ツリーで誤って追加しないため）

**変換**（入力 UI 10-3 はファイルの名前と形式を実装工程に委ねている。以下は AI 起案）:

- **入力の想定**: 文字コードは Shift_JIS、1行目は見出し、各行は「`YYYY/M/D`,名称」。**この想定は原本を読んで確かめていない**（2026-10-30 の時点で原本が無い）。段2 で原本を読み、想定と違えば（文字コード・見出し・日付の書き方・列の数）、変換を進めずに停止を申告する
- **出力**: `lib/holidays/data.ts` の `HOLIDAY_ROWS`（M-11）。各行の日付を `YYYY-MM-DD`（月と日を2桁に0で埋める）へ、名称は原本の字面のまま。見出しの行と末尾の空行を除き、**ほかの行を足さず、除かず、並べ替えない**（入力 UI 10-4・10-5）
- `data.ts` は生成物であり、手で書き換えない旨を先頭のコメントに書く。変換は上の規則で再現でき、変換のスクリプトはリポジトリに置かない（入力 UI 10-7 の手順2 の差し替えも同じ規則で行う）
- **判定**: `isHoliday(date)` は `HOLIDAY_ROWS` の `date` の集合に `date` があれば真とする。計算で補わない（入力 UI 10-5）

**利用条件と出典の表示**（入力 UI 10-8。確認と表示の形は実装工程に委ねられている。以下は AI 起案）:

- devcontainer は外向き通信を制限しており（`docs/rules/commands.md`）、AI は公表元の利用条件を読めない。そこで、**人間が原本を取得するときに、公表元のページが示す利用条件（その URL と、出典の表示について求める字面）を確かめ、原本とあわせて渡す**ことを段2 の前提とする。渡されなければ、段2 に入らず停止を申告する
- 表示の形: `data.ts` の先頭のコメントに、利用条件が求める出典の字面・取得元の URL・取得日・加工した旨（文字コードと日付の書式を改め、TypeScript の形式へ変換した）を書く。PR 本文にも同じ出典を書く（入力 UI 10-7 の手順5 の参照とあわせる）
- 利用条件が**画面への出典の表示**を求めるときは、画面の文言と置き場を本仕様も UI 仕様も定めていないため、実装を進めずに停止を申告する（AI が文言と置き場を決めない）

---

## T. テストの置き場

### T-0. 差し替えの方針（全画面）

出所: 2026-10-29 人間の明示選択（テストの差し替えの方針）。画面1〜画面3 のすべてのテストに掛かる。

- **T-0-1. `vi.mock` は Next のランタイムのモジュール（`next/headers`・`next/navigation` など）の差し替えに限って使ってよい。** 自前のモジュール（`@/lib/*`・`./_lib/*`・`./_components/*` など）を `vi.mock` で差し替えない。自前のモジュールは、手書きの Fake を引数・props で注入して差し替える（`docs/rules/development-process.md` の「テストダブルは手書きのインメモリ Fake」と同じ考え方）。ブラウザの大域（`fetch`・時計）はモジュールではないため `vi.mock` の対象にならない。手書きの Fake を `globalThis.fetch` に代入して後で戻す・`vi.useFakeTimers` で時計を進める形で差し替える（この1文は方針からの帰結として AI が書いた）。画面1 のテストのうち T-0-1 に反していたもの（旧 `apps/web/src/app/page.test.tsx`）は、T-0-3 の是正で改めた
- **T-0-2. 注入の形** —— App Router の `page.tsx` の default export は、Next が渡す `params`・`searchParams` 以外の props を受け取れない。また `page.tsx` は default export と Next が定める名前（`metadata` 等）以外を輸出できない。そこで、ページを**薄い層**と**本体**に分ける:
  - `app/_lib/screen-deps.ts`（サーバー専用）に型 `ScreenDeps = { getCurrentViewer: () => Promise<ResolveViewerResult>; getTheme: () => Promise<Theme> }` と、本物の依存 `serverScreenDeps`（`getCurrentViewer` は `current-viewer.ts` のもの、`getTheme` は `cookies()` → `resolveTheme`）を置く
  - 本体は `app/_components/<画面>-screen.tsx` に置く async 関数で、`ScreenDeps`（と、要る画面は `searchParams` を待った値）を引数で受け、判定（C-3・C-4）と描画の組み立てを持つ。本体は `screen-deps.ts` も `current-viewer.ts` も値として輸入しない（輸入すると `next/headers` がテストへ入る）。型 `ScreenDeps` 等は `import type` に限って輸入してよい（型の輸入は実行時に消える）。時刻を要する本体（W-7 の `thisMonthScreen`）は、`ScreenDeps` の代わりに `ThisMonthDeps`（W-7）を受ける
  - `page.tsx` は `serverScreenDeps` を本体へ渡して戻り値を返すだけにする。テストは本体を輸入し、手書きの Fake の `ScreenDeps` を渡して呼ぶ。`redirect` は `next/navigation` なので `vi.mock` してよい（T-0-1）
  - 薄い層（`page.tsx`・`screen-deps.ts`・`current-viewer.ts`）はテストしない。分岐を持たない配線であり、限界として記録する。ただし `app/this-month/page.tsx` の `dynamic` の輸出は、先読みの担保（W-7）として字面を静的に読む（T-2）
- **T-0-3. 画面1 の是正（済み）** —— 画面1 のテスト（旧 `apps/web/src/app/page.test.tsx`）は自前のモジュール `./_lib/current-viewer` を `vi.mock` しており、T-0-1 に反していた。是正は T-0-2 の形で、コミット 734807c（`test:`）と 77f09e9（`refactor:`）で行った。結果は次のとおりで、L-1・C-3・T-1 の本文はこの形で書いている:
  - **L-1**: ログイン画面の判定と描画は本体 `app/_components/login-screen.tsx` の `loginScreen(deps: ScreenDeps)` が持ち、`app/page.tsx` は `loginScreen(serverScreenDeps)` を返すだけの薄い層である。テストは `app/_components/login-screen.test.tsx` に置き、Fake の `ScreenDeps` を渡す。`vi.mock` するのは `next/navigation` だけである。担う AC（6-10・6-10-a・9-1）は変えていない
  - **C-3**: `getCurrentViewer()` は `current-viewer.ts` のまま変えていない。本体は `deps.getCurrentViewer()` を呼び、`serverScreenDeps` が `getCurrentViewer` を束ねる（React の `cache` による1要求1回は保たれる）

### T-1. 画面1

| テストファイル | 担う AC | 備考 |
|---|---|---|
| `apps/web/src/lib/screen-access.test.ts` | `login-ui.md` 6-7・6-7-a・6-8・6-8-a・6-9・6-9-a・6-9-b・6-10・6-10-a | `ScreenId` 7種 × 区分3種の全組と、ログイン画面 × 区分3種を表で持つ |
| `apps/web/src/lib/viewer.test.ts` | 5-2・6-5、`bff-auth-termination.md` 5-11 (i)(ii)(v) | Cookie なし／検証失敗（→ `guest`。Q-4 (i) = (a)）／ロール Cookie なし・改竄・正当（`Approver`）／構成欠落・鍵欠落（→ `misconfigured`。`guest` にならない。Q-4 (ii) = (a)）。公開鍵の解決は既存のテストと同じく差し替える |
| `apps/web/src/lib/role-switch-client.test.ts` | 6-9-b・7-2 | 送る要求（メソッド・パス・本文なし）と、204・401・429・500・例外の分類 |
| `apps/web/src/app/_components/header-controls.test.tsx` | 6-1・6-5・9-2・9-3・9-4・9-6・9-7 | L-3 の表を `headerSlots` の入出力にし、描画した DOM の並びも読む。ログインへのリンクはアクセシブル名「ログイン」・行き先 `/` で引く |
| `apps/web/src/app/_components/site-header.test.tsx` | 9-1 | ログイン画面・`app` の両方で `<header>` が出る |
| `apps/web/src/app/_components/role-switch-control.test.tsx` | 6-1・6-2・6-6・7-2・9-5 | 204 で `refresh` がちょうど1回・`push`/`replace` は0回、失敗で `role="alert"` が「ロールを切り替えられませんでした。」を持って出て `refresh` 0回、通信中の再入を無視、`et_theme` を変えない。`next/navigation` は差し替える |
| `apps/web/src/app/_components/theme-control.test.tsx` | 9-4、`design-system.md` 12-3 | 選択後に `data-theme` と保存の文字列が変わる。jsdom は既定の http の URL で `Secure` 付き Cookie を保存しないことがあるため、`document.cookie` を読むなら https の URL で動かすか、書き込みを差し替えて観測する（手段は tester が選ぶ） |
| `apps/web/src/app/_components/login-entries.test.tsx` | 1-1・1-2・2-3・3-1・4-1・4-3 | 入口がちょうど3つ、SMS を出さない、パスワード欄・フォームが無い、各入口の行き先、注記「入力・締め・承認はできません。」が出る。**入口はアクセシブル名で引いて区別する**（L-2。(b)(c) は同じ `href` のため行き先では区別できない）—— 「ゲストとして続行（閲覧のみ）」→ `/demo`、「メールアドレスでログイン」→ `/api/auth/sign-in`、「Google でログイン」→ `/api/auth/sign-in` がそれぞれちょうど1つ |
| `apps/web/src/app/_components/login-screen.test.tsx` | 6-10・6-10-a・9-1 | T-0-2 の形。本体 `loginScreen` へ手書きの Fake の `ScreenDeps` を渡し、`next/navigation` の `redirect` だけを `vi.mock` する（T-0-3）。技術者・承認者で既定の画面へ `redirect`、ゲスト（検証に失敗したセッションを含む＝`resolveViewer` が `guest` を返した場合）で描画されること、`misconfigured` で例外を投げ `redirect` も描画もしないこと（C-3。Q-4 (ii) = (a)）を読む |
| `apps/web/src/lib/session-cookie.test.ts`（既存へ追加、無ければ新規） | 9-5 | `expiredAuthCookies()` に `et_theme` が含まれない |

テストで担保しないもの（限界）: ホストされたサインイン画面の中身（2-1 の新規登録・2-2 の TOTP・3-2 のシークレット。`cognito-auth-infra.md` 11-11）、実ブラウザでの Cookie の保存と送信（`design-system.md` 11-43）、`router.refresh()` の後に Next がサーバーの `redirect()` を実際の遷移にすること（Next の振る舞い）。6-2・6-4・6-6 のうち操作要素の出し分けは画面2・3 の節が担う。

選んだ方式から生じる限界（2026-10-29 人間の明示選択に由来する。決定の時点で受け入れた帰結であり、テストで埋める対象ではない）:

- **ログイン画面はログインの失敗を出さない（7-1 の一部）。** 資格情報・MFA の誤りの提示と再試行はホストされた画面が持ち、本リポジトリのテストは見ない。サインインの戻りで失敗したとき（取消・`state` の不一致等）は BFF の 401 の素の応答が出るだけで、ログイン画面の `ErrorBanner` も再試行の導線も出ない。出所: 2026-10-29 人間の明示選択（Q-3 = (a)）
- **Google の入口は Google へ直接は届かない（3-1 の経路）。** (c) は (b) と同じ `/api/auth/sign-in` へ遷移し、Google の選択肢はホストされた画面が出す。`login-entries.test.tsx` が確かめるのは (c) が文言で区別されて存在し `/api/auth/sign-in` を指すことまでで、その先で Google が選べることは見ない。出所: 2026-10-29 人間の明示選択（Q-2 = (a)）
- **検証に失敗したセッション Cookie は、ページを開いただけでは消えない。** ページはゲストとして描くが Cookie を書けず、破棄は次に Route Handler を通る要求まで遅れる。出所: 2026-10-29 人間の明示選択（Q-4 (i) = (a)）

### T-2. 画面2

AC 番号は、ことわりの無いものは `work-month-listing-ui.md` のもの。差し替えは T-0 に従う（`vi.mock` は `next/navigation` 等の Next のランタイムのモジュールに限る。`fetch` は手書きの Fake を `globalThis.fetch` に代入し、時刻は引数で注入するか `vi.useFakeTimers` で固める）。関数の名前と型は W-7・W-8 のとおり。

| テストファイル | 担う AC | 備考 |
|---|---|---|
| `apps/web/src/lib/work-month-list.test.ts` | 4-1・4-2・6-3・9-4・9-5・9-6・9-6-a・9-6-b・9-7・9-8・9-11 | 純粋関数（W-8）を表で読む。絞り込み（`filterWorkMonths`）は `EMPTY_WORK_MONTH_FILTER` で入力と同じ行を同じ順で返す（9-4）、条件の積（9-5）、絞った後も入力の順を保つ（4-1・9-8）。対象年月の選択肢は重複なし・降順（9-6-a）、承認待ちの契約の選択肢は重複なし（9-6）。状態の選択肢は3つで、表示が `StatusBadge` の描く語と一致する（9-6-b。期待値を字面で書かず、`StatusBadge` を描いて比べる）。ページ分けは `WORK_MONTH_LIST_PAGE_SIZE` を輸入し、境界（ちょうど1ページ・1件超え・0件で `pageCount` が 1）を読む。直近は条件を指定しても絞る前の先頭（6-3）。`formatYearMonth`（`2026-07` → 「2026年7月」） |
| `apps/web/src/lib/work-month-list-client.test.ts` | 1-1・7-3 | W-8 の取得関数に手書きの Fake の `fetchImpl` を渡して読む。本人一覧のパスに `engineerId` を付けない、`limit` も `state` も送らない（W-3）、`offset` を `0` から `total` まで送る（2ページ・`items` が空で止まる・重複の除去で先に来た行を残す）、2ページ目が `ok` でなければその分類を返す、承認待ち・デモのパス（デモは1回だけ送る）、`fetchOwnContracts(now)` が R-10 に `currentJstYearMonth(now)` の `yearMonth` を付ける。200・401・403・429・500・例外・JSON でない本文の分類（`ok`／`refresh`／`failed`）。`fetchImpl` を省くと呼び出しの時点の `globalThis.fetch` を使う |
| `apps/web/src/lib/jst-month.test.ts` | 10-9 | `currentJstYearMonth` の境界: `2026-10-31T14:59:59Z` → `2026-10`、`2026-10-31T15:00:00Z` → `2026-11`、年をまたぐ `2026-12-31T14:59:59Z` → `2026-12`、`2026-12-31T15:00:00Z` → `2027-01` |
| `apps/web/src/lib/screen-routes.test.ts` | 5-1・9-12・10-9 | `workMonthPath` が各セグメントを符号化する、`ownWorkMonthsPath({ contractId })` がクエリ `contract` を持つ・指定が無ければクエリを持たない。`thisMonthPath("c-001")` が `/this-month?contract=c-001` と一致する（パス `/this-month` とクエリ `contract` だけを持ち、年月を含まない）。契約 ID に `&`・`=`・`/` を含めても、クエリ `contract` の値1つとして復号でき、別のパラメータを生じない（C-2。パスとクエリの名前は AI 起案）。ヘルプ・契約一覧・稼働の推移のパスが `/help`・`/contracts`・`/work-trends`（C-2。Q-7 = (a)） |
| `apps/web/src/lib/this-month-entry.test.ts` | 6-2・7-1・10-9-a・10-9-b・10-9-c・10-9-d | 契約0件 → `hidden`、1件 → `link`（その契約の `thisMonthPath`）、2件と3件 → `choice`（`options` の数が契約の数と同じ、並びが入力の順、`label` が契約表示名、`href` がそれぞれの契約の `thisMonthPath`）。入力の契約を除かない（10-9-c。契約期間で絞らない）。`thisMonthEntry` は年月を引数に取らない（Q-10） |
| `apps/web/src/lib/side-nav-items.test.ts` | 10-2・10-3・10-4・10-4-a・10-5（一覧の3画面）・10-6・10-9-a〜10-9-d | 区分3種 × 開いている画面のキーの表で、項目の文言・並び・`footerItems`・`current` がちょうど1つ真であることを読む。`href` は C-2 の6つ（ヘルプ・契約一覧・稼働の推移を含む）。承認者・ゲストに「今月の入力」が無い。技術者で `thisMonth` が `hidden` のとき「今月の入力」が無く、`link` のとき `href` を持つ項目、`choice` のとき `href` を持たず `options` を持つ項目になる |
| `apps/web/src/app/_components/app-side-nav.test.tsx` | 10-1・10-7・10-9-a・10-9-b・10-9-c・10-9-d・10-9-g | 技術者で契約1件 → 「今月の入力」がリンクで、行き先がその契約の `thisMonthPath`（W-7）。契約2件 → 「今月の入力」が `button` のロールで `aria-expanded="false"`、押す前に「今月の入力」の選択肢のリンク（`thisMonthPath` を指すもの）も勤務月画面へのリンクも文書に無い（10-9-b）、押すと契約表示名の `link` が契約の数だけ R-10 の順で現れ、それぞれがその契約の `thisMonthPath` を指す（10-9-c）。0件・取得の失敗 → 「今月の入力」が無い（10-9-d）。承認者・ゲストは契約を取得しない（`fetch` が呼ばれない）。開閉と描画で GET 以外を送らない（10-7・10-9-g）。描画したまま時計を JST の月の境目（`2026-10-31T15:00:00Z`）の前後へ進めても、行き先の文字列が変わらない（行き先は年月を含まない。Q-10） |
| `apps/web/src/components/side-nav.test.tsx`（既存へ追加） | `design-system.md` 6-13-iii・6-13-v（10-6-q） | 10-6-q の追記どおり。`href` を持つ項目と `options` を持つ項目を混ぜた描画で、閉じた並び・`button` の名・`aria-expanded`・開いた後の選択肢の `link` の数・順・名・`href`・位置・再び押して閉じること、`current` が真のとき `button` に `aria-current="page"` |
| `apps/web/src/components/side-nav-marker-color.test.tsx`（既存へ追加） | `design-system.md` 6-13-iv（10-3-p） | 10-3-p の追記どおり。`options` を持つ項目を1つ加えた閉じた描画で、その `button` の丸印を読み、(iii) を `link` と `button` の数の和で読む。改定前の2通りの描画は変えない |
| `apps/web/src/app/_components/work-month-table.test.tsx` | 1-2・2-3・5-1 | 各行に契約表示名・「YYYY年M月」・状態バッジ、リンクの文言「開く」とアクセシブル名「`<契約表示名>` × `<対象年月>`を開く」と行き先（`workMonthPath`） |
| `apps/web/src/app/_components/initial-entry-action.test.tsx` | 6-2・7-1・10-9-b・10-9-c | 契約0件で何も出さない（`container.innerHTML` が空）、1件でリンク「稼働実績を入力する」がその契約の `thisMonthPath` を指す。2件以上で、`button`「稼働実績を入力する」が `aria-expanded="false"` で出て、押す前に選択肢のリンクが文書に無い（`link` のロールで問い合わせられない）、押すと `aria-expanded="true"` になり、契約表示名の `link` が契約の数だけ入力の順で、文書の順で `button` より後に現れ、それぞれがその契約の `thisMonthPath` を指す、再び押すと `aria-expanded="false"` に戻り選択肢のリンクが文書から消える（W-4。Q-9 = (a)。見え方は `design-system.md` 6-13-v と同じ）。開閉で `fetch` を呼ばない |
| `apps/web/src/app/_components/this-month-screen.test.tsx` | 10-9・10-9-a・10-9-c・10-9-g、`login-ui.md` 6-7-a | T-0-2 の形。本体 `thisMonthScreen` へ手書きの Fake の `ThisMonthDeps`（`getCurrentViewer` と `now`）を渡し、`next/navigation` の `redirect` だけを `vi.mock` する（送り先を記録して例外を投げる形。画面2 の `*-screen.test.tsx` と同じ）。W-7 の送り先の表の各行を読む —— **技術者・`contract=c-001`**: `now` が `2026-10-31T14:59:59Z` → `workMonthPath("c-001", "2026-10")`、`2026-10-31T15:00:00Z` → `workMonthPath("c-001", "2026-11")`、`2026-12-31T15:00:00Z` → `workMonthPath("c-001", "2027-01")`（JST の境目で当月が切り替わる。Q-10）。**承認者・ゲスト**（`contract=c-001` を付けても）→ それぞれ `defaultScreenPath` の値（承認待ち一覧・デモ勤務月群）で、勤務月画面へ送らない。**技術者で `contract` が無い・空文字・配列（`["c-001", "c-002"]`）** → `ownWorkMonthsPath({})`。**`misconfigured`** → 例外を投げ、`redirect` を呼ばない。どの行も `redirect` はちょうど1回（`misconfigured` は0回）で、本体は JSX を返さない。どの行でも `fetch` を呼ばない（`globalThis.fetch` に呼ばれたら記録する Fake を置き、0回を読む。10-9-g）。他人の契約の ID を区別しないこと（技術者・`contract=c-other` でも `workMonthPath("c-other", 当月)` へ送る＝W-7 の「本人の契約を確かめる場所」）も1行で読む |
| `apps/web/src/app/_components/this-month-prefetch.test.ts` | 10-9（「選んだ時点」） | W-7 の先読みの担保を、ファイルを読んで静的に確かめる（Next を動かさない）—— (1) `app/this-month/page.tsx` の字面が `export const dynamic = "force-dynamic"` を持つ（ページを輸入すると `next/headers` が入るため、字面で読む）、(2) `src/app/loading.*` と `src/app/this-month/loading.*` が無い（拡張子 `tsx`・`ts`・`jsx`・`js`）、(3) `next.config.ts` の既定の輸出の `cacheComponents` と `partialPrefetching` がいずれも `true` でない、(4) `components/side-nav.tsx` と `app/_components/initial-entry-action.tsx` の字面に `prefetch` が現れない。各検査には陽性対照を添える（検査の述語へ違反の字面を与えると落ちること） |
| `apps/web/src/app/_components/own-work-months-view.test.tsx` | 1-2・1-3・4-1・4-2・5-1・6-1・6-2・7-1・7-3・9-1・9-4・9-9・9-10・9-11・9-12・9-13・9-14 | 3つの状態の行がすべて載る（1-3）、直近のリンクが絞り込みに関わらず先頭の勤務月を指す、勤務月0件で `EmptyState` が出て、契約1件なら初回入力のリンクがあり、契約0件なら初回入力の導線が無い（6-2・7-1。Q-8 付随）、絞り込みの結果0件のときは初回入力の導線が無い（9-10）、条件を変えると1ページ目へ戻る（9-9）、条件の解除の出し分け（9-11）、クエリ `contract` の初期値（選択肢にある値・無い値）、R-2 と R-10 のどちらの失敗でも `role="alert"` が「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」を持って出て、再試行で再び GET する（7-3）、401 で `refresh` がちょうど1回、どの操作でも GET 以外を送らない（9-13） |
| `apps/web/src/app/_components/pending-approvals-view.test.tsx` | 2-1・2-3・2-5・4-1・4-2・5-1・7-2・7-3・9-2・9-6・9-9・9-10・9-11 | 返った行をすべて載せる（2-5）、状態の条件が無い（9-2）、0件の文言と action が無い（7-2）、失敗で `role="alert"` が本人一覧と同じ文言を持って出て再試行で再び GET する（7-3。Q-6 = (a)） |
| `apps/web/src/app/_components/demo-work-months-view.test.tsx` | 3-1・3-2・5-1・7-3・9-3 | 3状態の行が載る、各行のリンクが「開く」でアクセシブル名に契約表示名を含み勤務月画面を指す（Q-6 = (a)）、`role="search"` も `Pagination` も無い（9-3）、注記が出る、0件で title「サンプル勤務月はありません」だけの `EmptyState`（description・action が無い）、失敗で `role="alert"` が本人一覧と同じ文言を持って出て再試行で再び GET する（7-3。Q-6 = (a)） |
| `apps/web/src/app/_components/own-work-months-screen.test.tsx`・`pending-approvals-screen.test.tsx`・`demo-work-months-screen.test.tsx` | 2-4・3-3・9-12、`login-ui.md` 6-9 | T-0-2 の形。本体へ手書きの Fake の `ScreenDeps` を渡し、`next/navigation` だけを `vi.mock` する（自前のモジュールを `vi.mock` しない）。開けない区分で既定の画面へ `redirect`、開ける区分で描く、`misconfigured` で例外を投げ `redirect` も描画もしない。本人一覧は `searchParams` の `contract` が文字列1つのときだけ `initialContractId` として渡し、無いとき・配列のときは渡さない |

テストで担保しないもの（限界）:

- 本人一覧に他の技術者の勤務月が載らないこと（1-1）と、承認待ち一覧に `PendingApproval` 以外が載らないこと（2-1・2-2・9-7）は、BFF とドメイン API が担保する（`bff-wiring.md` 6-b・6-c、`domain-api-http-contract.md` AC-3）。画面のテストが読むのは、要求に `engineerId` を付けないことまでである
- 全件の取得の途中で一覧が変わると、行の抜け・重複が起こりうる（重複は除くが、抜けは検出しない）。取得の要求は件数に比例して増え、BFF のレート制限を他の要求と分け合う。出所: 2026-10-29 人間の明示選択（Q-5 = (a)）から生じる帰結
- R-4 は要求を読まないため、デモ勤務月群はドメイン API の既定の件数までしか出ない。デモデータの勤務月はそれより少ない（`demo-data-reset.md`「デモデータの値」節）が、件数の比較はテストしない
- 条件とページを URL に残さないため、再読み込みで既定へ戻る（W-2）
- **ヘルプ・契約一覧・稼働の推移の項目は、各画面のページが入るまで 404 へ遷移する。** テストは `href` の文字列までを読み、遷移先のページの有無は読まない。出所: 2026-10-29 人間の明示選択（Q-7 = (a)。404 を受け入れた）
- `SideNav` の選択肢を開く項目（6-13-v）を、外側を押したときや `Esc` で閉じるか、ページを移った後に開閉を保つかは定めておらず、読まない（`design-system.md` 6-13-v）
- **`/this-month` が実際に先読みされないこと**は読まない。先読みは本番の構成でだけ働く Next の振る舞いであり（W-7 の「先読み」）、テストが読むのは先読みされない条件（`force-dynamic`・`loading.*` の不在・設定・`prefetch` の不在）の字面までである。Next の版を上げて先読みの規則が変わったときは、W-7 の根拠の資料を読み直す
- **`/this-month` は本人の契約かを確かめない**（W-7。2026-10-30 人間の明示選択（Q-11 = (e)））。他人の契約の勤務月が開けないことは、画面3 とドメイン API が担保し、画面2 のテストは読まない（W-7 の「画面3 への申し送り」）
- `/this-month` の転送の状態コードは読まない（W-7。`redirect()` に渡した送り先だけを読む）

### T-3. 画面3

AC 番号の略記は M 節と同じ（ことわりの無いものは `work-month-screen-ui.md`）。差し替えは T-0 に従う —— `vi.mock` は `next/navigation` 等の Next のランタイムのモジュールに限り、自前のモジュール（`@/lib/*`・`./_components/*`・`lib/holidays/` を含む）を `vi.mock` しない。`fetch` は手書きの Fake を `globalThis.fetch` に代入して後で戻し、時刻は引数で注入するか `vi.useFakeTimers` で固める。祝日は `calendarCells` の `isHoliday` 引数・`WorkMonthView` の `isHoliday` props で差し替え、画面の部品のテストでは `isHoliday` を経ずに `cells` を props で渡す。関数の名前と型は M-11 のとおり。

**段の分け方**（M-12。出所: 2026-10-30 人間の明示選択（Q-15 = (a)）。分け方は AI 起案）: テストを「段1」と「段2」の2つの表に分ける。**段1 の表のテストは `lib/holidays/` を輸入せず、祝日データの同梱に依存しない**（祝日は Fake の `isHoliday` で与える）。段2 の表のテストだけが同梱した祝日データを読む。段2 は原本が置かれるまで書かない。

**本人かの真偽のキー**（M-3）: テストは真偽のキー名の字面を書かず、`OPERATOR_OWNERSHIP_KEY` を `lib/work-month.ts` から輸入して Fake の本文を組む（例: `{ ...表現, [OPERATOR_OWNERSHIP_KEY]: true }`）。「真偽が無い」応答は、そのキーを持たない本文で表す。#155 でキー名が決まったとき、テストを改める必要は無い。

**文言の期待値**: M-9 の文言（AI 起案・未承認）は、テストに字面で書く（画面2 のテストが W-4 の文言を字面で持つのと同じ）。人間が文言を改めたら、`lib/work-month-feedback.ts` とテストの字面を改める。

#### 段1 — 祝日データに依存しない

| テストファイル | 担う AC | 備考 |
|---|---|---|
| `apps/web/src/lib/work-month-actions.test.ts` | AC-3（全行）・5-1・5-2、締め UI 1-1〜1-3・4-1・5-1・5-2、承認 UI 1-1〜1-4・3-1・3-2、契約 AC-6 の申し送り | M-4 の表をそのまま入出力にする。`operator` 6種（ゲスト・①〜④・ロール2種 × `"unknown"`）× 状態3種 × `generated` 真偽の全組で4つの真偽を読む。`generated` が偽の `Draft` で①④の `close` が偽・`edit` が真。空月（`generated` 真）で `close` が真。`workMonthNote` は M-5 の8つの文をそれぞれ1組で読み、順の優先（ゲストで `Approved` → 「ゲストとして閲覧しています。」、締めた直後で `PendingApproval` → 「締めました。…」）を読む |
| `apps/web/src/lib/work-month.test.ts` | 1-3・1-4・AC-3、入力 UI 1-1・1-6 | `formatHours`: `{9,30}`→`9.5`、`{10,0}`→`10.0`、`{7,45}`→`7.75`、`{0,15}`→`0.25`、`{0,0}`→`0.0`、`{180,15}`→`180.25`。`formatTimeOfDay`: `{9,0}`→`9:00`、`{17,45}`→`17:45`、`{0,15}`→`0:15`。`formatTimeRange` が `〜` で結ぶ。**`readOwnership`**（M-3）を表で読む —— キーの値が `true` → `"own"`、`false` → `"other"`、キーが無い → `"unknown"`、値が `"true"`（文字列）・`1`・`null` → `"unknown"`、本文が `null`・配列・文字列 → `"unknown"`。**キー名の持ち主が1つであること**（M-3）: `OPERATOR_OWNERSHIP_KEY` を輸入し、その値を、`apps/web/src` 配下の `*.ts`・`*.tsx`（テストを含む）の字面から走査して、現れるファイルが `lib/work-month.ts` のちょうど1つであることを読む（引用符を問わず値の字面で探す）。走査の述語は「パスと内容の組の列」を受けて現れるパスを返す関数に切り出し、陽性対照として、実際の列に値の字面を含む第2の組を足して与えると2つを返すことを読む（陽性対照の内容も定数から組み、字面をテストに書かない）。走査した `*.ts`・`*.tsx` が0件でないことも読む |
| `apps/web/src/lib/work-month-client.test.ts` | AC-6、入力 UI 3-6-c、`bff-wiring.md` 6-6・6-7 | 6つの関数に手書きの Fake の `fetchImpl` を渡し、送った要求のメソッド（GET・PUT・DELETE・POST×3）とパスを読む。`workMonthApiPath("c-001","2026-07")` が `/api/work-months/c-001/2026-07`、`/`・`?`・`#` を含む値が1つのセグメントに符号化される。R-5 の本文を JSON として読んだ値が `input` と深く等しく、キーがちょうど `startTime`・`endTime`・`break`（`break: null` のときもキーがある）、`workingHours` が無い、`Content-Type` が `application/json`。R-6〜R-9 が本文を持たない。分類: 200 の表現 → `ok`、401 → `refresh`、400・403・404・409 の `{"error":{"code":"X"}}` → `rejected`（`status` と `code`）、429・500・例外・JSON でない本文・`code` の無い 403・`dailyRecords` の無い 200 → `failed`。**`ok` の `ownership`**: 本文の `[OPERATOR_OWNERSHIP_KEY]` が `true` → `"own"`、`false` → `"other"`、キーが無い → `"unknown"` で、**キーが無くても `ok` である**（`failed` にしない）。6つの関数のいずれの `ok` でも同じに読む（操作の成功の応答も表現である＝契約 D-6）。`fetchImpl` を省くと呼び出しの時点の `globalThis.fetch` を使う |
| `apps/web/src/lib/work-month-feedback.test.ts` | AC-6、入力 UI 4-1-a・4-3・4-9、締め UI 6-1、承認 UI 10-8 | M-9 の2つの表を入出力にする。`operationFailureKind`: `rejected` の 400 → `invalid-input`、403・404 → `not-allowed`、409 → `state-changed`（それぞれ `code` を2通り以上変えても同じ種類）、`failed` → `communication`。`loadFailureKind`: `rejected` の 404・400 → `missing`、403・409・`failed` → `failed`。`OPERATION_FAILURE_MESSAGES` の4つの文が M-9 の字面と一致し、互いに異なり、`TIME_ENTRY_MESSAGES`（M-7）の7つの文のどれとも異なる。`WORK_MONTH_LOAD_ERROR_MESSAGE` が「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」、`WORK_MONTH_MISSING` が M-9 の title・description・linkLabel の字面と一致する |
| `apps/web/src/lib/work-month-calendar.test.ts` | 入力 UI 1-4・1-5・4-2・9-1〜9-7・9-9 | マスの数と先頭の空き: `2026-06`（1日が月曜）→ 空き0・計35、`2026-03`（1日が日曜）→ 空き6・計42、`2026-02` → 空き6・計35、`2028-02`（うるう年）→ 空き1・日29・計35。`mark`: `isHoliday` が真の平日・日曜・土曜 → `hol` で `holiday` 真（9-1〜9-3）、偽の日曜 → `sun`、偽の土曜 → `sat`、偽の平日 → 空文字（9-4〜9-6）。`isHoliday` が呼ばれる引数が `YYYY-MM-DD`。`future`: `today` が `2026-10-02` のとき `2026-10-02` は偽・`2026-10-03` は真。`record` が日付で結び付く。`WEEKDAY_HEADERS` が月〜日の7つ。`dayLabel`（M-6 の2例と、未来の日の「（未来の日付）」）。`firstInputDay`（土日・祝日・未来を飛ばす、該当が無ければ1日） |
| `apps/web/src/lib/jst-month.test.ts`（既存へ追加） | 入力 UI 4-2 | `currentJstDate`: `2026-10-31T14:59:59Z` → `2026-10-31`、`2026-10-31T15:00:00Z` → `2026-11-01`、`2026-12-31T15:00:00Z` → `2027-01-01` |
| `apps/web/src/lib/time-entry.test.ts` | 入力 UI 3-2-a・3-4・3-5・3-6-a・3-6-b・3-6-f・4-1・4-5〜4-9・5-1 | `HOUR_OPTIONS`・`MINUTE_OPTIONS` の値の集合が `daily-record-entry.md` D-10 と過不足なく一致し昇順（期待値を字面で書かず、D-10 の条文から読むか、`daily-record-entry.md` AC-3 の 3-A の許可の行から読む。読み方は tester が選ぶ）。M-7 の理由の表を1行ずつ読む（未選択の3つは `tried` 真のときだけ出て偽のとき出ない、行の割り当て、上の行が優先されること）。休憩なしで休憩の行を見ない。`TIME_ENTRY_MESSAGES` の7つの文が互いに異なる（4-9）。**不変条件**: `daily-record-entry.md` AC-3 の 3-B・3-C の表の、時刻の組を持つ各行（`working-hours.test.ts` と同じく表から読む。行をテストへ手で写さない）について、`timeEntryProblems(form, true)` が空であることと `computeWorkingHours` が `null` でないことが一致する。`previewWorkingHours` は未選択が残ると `null`、そろえば `computeWorkingHours` と深く等しい。`readTimeEntry` は休憩なしで `break: null`、未選択が残ると `incomplete`。`formFromRecord` が `break: null` で `hasBreak` 偽 |
| `apps/web/src/lib/side-nav-items.test.ts`（既存へ追加） | 一覧 UI 10-5（勤務月画面） | `workMonthSideNavCurrent` を M-10 の表で読む。技術者で自分の契約 `c-001`・`now` が `2026-10-31T14:59:59Z`: `c-001 × 2026-10` → `this-month`、`c-001 × 2026-09` → `own-work-months`、`c-other × 2026-10` → `own-work-months`、`ownContracts` が `undefined` → `own-work-months`。`now` が `2026-10-31T15:00:00Z` のとき `c-001 × 2026-11` → `this-month`。ゲスト → `demo-work-months`、承認者 → `pending-approvals` |
| `apps/web/src/app/_components/app-side-nav.test.tsx`（既存へ追加） | 一覧 UI 10-5 | `current` に `WorkMonthLocation` を渡す。技術者・R-10 が `c-001` を返す・時計を `2026-10-15T00:00:00Z` に固める: `c-001 × 2026-10` で「今月の入力」に `aria-current="page"` が付き「自分の勤務月」に付かない。`c-other × 2026-10` で「自分の勤務月」に付く。承認者で「承認待ち」、ゲストで「サンプル勤務月」に付き、どちらも `fetch` を呼ばない。`aria-current` を持つ要素がちょうど1つ |
| `apps/web/src/app/_components/work-month-screen.test.tsx` | `login-ui.md` 6-7・6-9、1-8・1-9 | T-0-2 の形。本体 `workMonthScreen` へ手書きの Fake の `ScreenDeps`、`{ contractId: "c-001", yearMonth: "2026-07" }`、**手書きの Fake の `view`**（受けた props を `data-*` 属性で描くだけの部品）を渡し、`next/navigation` だけを `vi.mock` する。区分3種のいずれでも `redirect` を呼ばずに描く（Fake の `view` がちょうど1回、`viewer`・`contractId: "c-001"`・`yearMonth: "2026-07"` の props で `AppShell` の子として描かれ、`<header>` と `<nav>` が出る）。`misconfigured` で例外を投げ `redirect` も描画もしない。本体の呼び出し（`await workMonthScreen(...)` が戻るまで）の間 `globalThis.fetch` が呼ばれない。戻り値を描画した後は、技術者の行で `AppSideNav` が R-10 を呼ぶため、Fake の `fetch` は R-10 に応答できるようにしておく（描画の後の呼び出しは本行では数えない） |
| `apps/web/src/app/_components/work-month-summary.test.tsx` | 1-3〜1-5・4-1〜4-3・4-5・4-6 | `Draft` で「締め時に確定します」が出て超過・不足の数値が無い。`PendingApproval`・`Approved` で超過・不足の値が `formatHours` で出る。超過 `{0,15}`・不足 `{0,0}` で、超過の数値と見出し「超過」の要素に `text-danger` があり、不足の数値と見出し・総稼働時間・精算幅に無い（4-6）。超過も不足も0なら `text-danger` を持つ要素が無い。`Draft` の「締め時に確定します」が `text-danger` を持たない。精算幅が `lowerBound`〜`upperBound` |
| `apps/web/src/app/_components/work-month-calendar.test.tsx` | 1-6、入力 UI 1-1・1-4〜1-6・9-1〜9-11 | `cells` を props で渡す。`editable` 真で日のマスが `button`（数がその月の日数）・アクセシブル名が `dayLabel`・選んだ日だけ `aria-pressed="true"`・押すと `onSelect` がその日付で1回。`editable` 偽で `button` が0個・`onSelect` が呼ばれない。実績のある日に `formatHours` と時刻の範囲が出て休憩の時刻が出ない。祝日のマスに「祝」の札（日曜の祝日にも出る）、`data-dow` が `hol`・`sat`・`sun`、無印の日に `data-dow` が無い。曜日の見出しの並び。9-11: `editable` の真偽で札と `data-dow` が変わらない |
| `apps/web/src/app/_components/daily-record-form.test.tsx` | 入力 UI 3-1・3-1-a・3-2・3-2-a・3-2-b・3-2-c・3-4・3-6・3-6-a・3-6-b・4-1・4-2・4-4・4-5〜4-9・5-1・6-1 | 時刻の選択が `combobox` で、休憩ありで8つ・アクセシブル名が互いに異なり M-7 の8つと一致、休憩なしで4つ。`input` の `text`・`number` が無い。休憩の組が1つだけで、休憩を足す操作が無い。未選択で保存 → `onSave` 0回・該当の行に `role="alert"` の理由・選んだ値が残る。M-7 の組み合わせの理由がそれぞれ1つの行に出る（理由の文が互いに異なる）。理由があるとき保存しても `onSave` 0回。妥当な値で `onSave` が `readTimeEntry` の `input` で1回（休憩なしで `break: null`）。保存前の稼働時間: 未選択が残ると「—」、そろうと `computeWorkingHours` から得た値（例: 9:00〜18:00・休憩 12:00〜13:00 → `8.0`）、弾く組で「—」（0 や負の値を出さない）。`cell.future` 真で「未来の日付には入力できません」が出て `combobox` と「保存」が無い。`cell.record` があると初期値がその時刻（休憩 `null` で「休憩あり」が外れている）で「削除」が出て、押すと `onDelete` 1回。`busy` 真で保存を押しても `onSave` 0回 |
| `apps/web/src/app/_components/work-month-operations.test.tsx` | 締め UI 1-4・2-1〜2-3・5-3、承認 UI 2-1・10-1〜10-7・10-9 | `CloseAction`: 「締める」を押すと `onConfirm` を呼ばずに `dialog` が M-8 の `title`・2段落で出る、取消・Esc で `onConfirm` 0回・`dialog` が消える、確定で1回。`ReviewActions`: 「承認」「差戻し」それぞれで承認 UI 10-9-a・10-9-b の `title`・`description`・`confirmLabel` の `dialog` が出る（字面は承認 UI 10-9 の表から写し、出所をテストに残す）、差戻しの `dialog` に `textbox`・`combobox` が無い（10-5）、取消で0回・確定で1回。`busy` 真で確定しても0回 |
| `apps/web/src/app/_components/work-month-view.test.tsx` | AC-1〜AC-7、入力 UI 2-1〜2-3・3-3・3-6-c・4-3・4-10・5-2・6-1・6-2・7-1・7-2、締め UI 3-1〜3-4・6-1、承認 UI 4-1〜4-4・5-1〜5-4・10-8、一覧 UI 10-9-e・10-9-f | `globalThis.fetch` に、パスとメソッドで応答を返し要求を記録する手書きの Fake を置き、`next/navigation` の `useRouter` だけを `vi.mock` する。`isHoliday` は Fake の関数を props で渡す（`lib/holidays/` を輸入しない）。表現の本文は `OPERATOR_OWNERSHIP_KEY` を輸入して組む。**どの行でも R-10 への要求が0回**（M-3）。**①技術者本人**（ロール `Engineer`・真偽 `true`。`Draft`・`generated` 真）: カレンダーの日が `button`、日を選んでフォーム、保存で PUT が1回・**応答の `totalHours` と日の値で置き換わる**（Fake は保存前の稼働時間と異なる値を返し、画面が応答の値を示すことを読む＝3-6-c）、削除で DELETE が1回・その日が「未入力」へ、「締める」→ 確定で POST `/close` が1回・状態バッジが「締め済」・超過／不足の数値が出る・入力の `button` が消える（締め UI AC-3）。空月（`generated` 真・実績0件）で「締める」がある（締め UI 5-2）。**未生成の当月**（`generated` 偽）: 空状態の title と「稼働実績を入力する」が出る、「締める」が無い、描画と開いただけでは R-1 の GET 以外を送らない（10-9-e）、保存の応答が `generated` 真なら「締める」が出る（2-3）。**②他人の契約**（ロール `Engineer`・真偽 `false`。`c-other` を開く。M-10）: `Draft`・`PendingApproval`・`Approved` のどれでも、入力の `button`・フォーム・「締める」・「承認」・「差戻し」が無く、見出し・状態バッジ・総稼働時間・精算幅・カレンダーの値が出る、GET 以外を送らない。**③承認者**（ロール `Approver`・真偽 `false`）: `PendingApproval` で「承認」「差戻し」が出て、承認の確定で POST `/approve` が1回・状態バッジが応答の `Approved` の語へ変わり「承認」「差戻し」が消える、差戻しの確定で POST `/reject` が1回・状態バッジが応答の `Draft` の語へ変わる（承認 UI AC-4・AC-5）。`Draft` で入力の `button`・「締める」が無い。**④承認者かつ本人**（ロール `Approver`・真偽 `true`）: `Draft` で入力の `button` と「締める」がある（AC-3 ④）、`PendingApproval` で「承認」「差戻し」が無い（承認 UI 3-1）。**真偽が無い**（キーを持たない本文。M-3）: ロール `Engineer`・`Approver` のそれぞれで、`Draft`・`PendingApproval` のどちらでも入力の `button`・フォーム・「締める」・「承認」・「差戻し」が無く、見出し・状態バッジ・総稼働時間・精算幅・カレンダーの値が出て、`role="alert"` が無い（取得の失敗にしない）。**取得中**（R-1 の応答を保留した間）: 操作要素が無く、内容の領域が `aria-busy="true"`。**`PendingApproval`・`Approved` の本人**: 操作要素が無い（5-1・5-2）。**ゲスト**: 真偽 `true` の本文でも、どの状態でも操作要素が無い、ゲストの注記が出る。**取得の結果（M-9）**: R-1 の 404 と 400 のそれぞれで、`EmptyState` の title「勤務月を表示できません」・description「契約が見つからないか、URL の形式が正しくありません。」と、リンク「勤務月の一覧を開く」が出て、その `href` が `defaultScreenPath(viewer)` の値と一致する（区分3種のそれぞれで）。このとき `role="alert"`・再試行の `button`・見出し（`<h1>`）・カレンダーが無い。R-1 の 500・例外・403 のそれぞれで、`role="alert"` が「勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。」を持って出て、再試行を押すと R-1 の GET がもう1回送られ、その応答が `ok` なら表現が出る。**操作の結果（M-9）**: 401 で `refresh` がちょうど1回・表示値が変わらない。保存の 400 で `role="alert"` が「入力した内容を受け付けられませんでした。内容を見直して、もう一度保存してください。」を持ち、選んだ値が残り、総稼働時間が変わらない（4-4・4-10）。保存の 403 と 404 のそれぞれで「この勤務月では、この操作はできません。」が出て表示値が変わらない。保存の 409 で R-1 が取り直され、取り直した表現（`PendingApproval`）で入力の `button` が消え、「勤務月の状態が変わっていたため、最新の内容に更新しました。」が出る（6-4）。締めの 409 で R-1 が取り直され、2回目の POST が無い（締め UI 6-1）。承認の 409 で R-1 が取り直される（承認 UI 10-8）。保存の 500 と例外のそれぞれで「通信に失敗しました。時間をおいて、もう一度お試しください。」が出て、表示値が変わらず PUT は1回だけ（自動で送り直さない）。締めの 500 で状態バッジが `Draft` の語のまま（6-3）。409 の取り直しが 500 なら、取得の失敗の文言が出る。操作の失敗の `ErrorBanner` に再試行の `button` が無い。**ロール切替**: `viewer` の props をロール `Engineer` から `Approver` へ替えて描き直すと、R-1 の GET がもう1回送られる（M-2）。**通信中**: 保存の応答を待つ間に保存をもう一度押しても PUT は1回 |
| `apps/web/src/app/_components/work-month-holidays-boundary.test.ts` | M-1・M-12（段の境界） | ファイル走査で、`lib/work-month-calendar.ts`・`app/_components/work-month-view.tsx`・`app/_components/work-month-screen.tsx` と、段1 の表のテストファイル（本テスト自身を除く）の字面に `holidays` の輸入（`from "…/holidays…"` の形）が無いことを読む。陽性対照として、述語に輸入の字面を含む内容を与えると当たることを読む。走査したファイルが0件でないことも読む |

#### 段2 — 祝日データ（M-12。原本が置かれてから書く）

| テストファイル | 担う AC | 備考 |
|---|---|---|
| `apps/web/src/lib/holidays/data.test.ts` | 入力 UI 10-3・10-4・10-4-a・10-5 | `HOLIDAY_ROWS` が0件でない。各行の `date` が `YYYY-MM-DD` で実在する日付（`2026-02-30` のような値でない）、`name` が空文字でない、`date` が重複しない、`date` が昇順。載る年が最初の年から最後の年まで欠けなく続く（10-4-a。期待値の年を字面で書かず、データから最初と最後の年を読んで間を数える）。陽性対照として、年の欠けの述語に1年を抜いた列を与えると落ちることを読む |
| `apps/web/src/lib/holidays/index.test.ts` | 入力 UI 10-2・10-5、AC-9 | `HOLIDAY_ROWS` のすべての `date` で `isHoliday` が真（名称を問わない＝10-5）。`HOLIDAY_ROWS` に無い日（データの最初の年の範囲で、行の `date` の集合に無い日を1つ以上データから選ぶ）で偽。データの最初の年より前の日（その年の前年の `01-01`）で偽（計算で補わない＝10-5） |
| `apps/web/src/lib/holidays/services-boundary.test.ts` | 入力 UI 10-2-a | ファイル走査で、`services/` 配下に名前に `holiday`・`syukujitsu` を含むファイル・ディレクトリが無く、`services/` 配下のファイルの字面に `apps/web` の `lib/holidays` への参照が無いことを読む。走査したファイルが0件でないこと、陽性対照（述語に参照の字面を含む内容を与えると当たる）を添える |
| `apps/web/src/app/_components/work-month-holiday-view.test.tsx` | 入力 UI AC-9（本物のデータの配線） | `HOLIDAY_ROWS` の行のうち1つの日を含む月の表現を、Fake の `fetch` で R-1 に返し、`WorkMonthHolidayView` を描くと、その日のマスに札「祝」が出て `data-dow` が `hol` である。月と日は `HOLIDAY_ROWS` から読み、字面で書かない |

テストで担保しないもの（限界）:

- **本人の判定は表示の出し分けであり認可ではない。** 画面が ② と判定して操作を出さないことはテストで読むが、他人の勤務月への状態を変える要求を弾くのはドメイン API である（契約 AC-4-3・AC-5-4・AC-6-3・AC-7-4・AC-8-4）。他人の勤務月を読めること自体は人間が確定した仕様である（契約 AC-11-3・D-12）
- **本人かの真偽をドメイン API が正しく返すことは読まない。** 画面のテストは、Fake の応答に真偽を置いて①〜④の出し分けを読む。真偽が操作者を正しく表すこと（④の判定・識別子を載せないこと）は、#155（M-3）のテストが担う。#155 が入るまで、本物の応答には真偽が無く、画面は閲覧だけになる（M-3 の帰結）
- **仮のキー名と、#155 が決めるキー名が一致することは読まない。** 一致させるのは、#155 が入るときに `OPERATOR_OWNERSHIP_KEY` の値を改める作業である（M-3）
- 同梱した祝日データが内閣府の公表と一致することは読まない（入力 UI 8-5）。原本をコミットしないため、変換が原本の行を落としていないことも読まない（行の数と内容の突き合わせは、段2 のレビューで人間が原本と見比べる。入力 UI 10-7 の手順3）
- 利用条件が求める出典の表示を満たすことは読まない（M-12。レビューで人間が確かめる）
- `page.tsx`（薄い層）が `WorkMonthHolidayView` を本体へ渡していることは読まない（T-0-2 の薄い層）
- 保存前の稼働時間とドメインの算出の一致は、3-B・3-C の表の行の範囲でだけ読む（入力 UI 8-8）。M-7 の理由の分類と `computeWorkingHours` の一致も同じ範囲に留まる
- Next が動的セグメントの `params` を符号化したまま渡すか復号して渡すかは読まない。本体のテストは素の文字列を渡す。契約の `contractId` の書式（契約 AC-1-10）に当たる値は符号化しても変わらないため、ふつうの値では差が出ない
- 開いたまま JST の日・月の境目を越えても、未来の日の判定とサイドバーの選択中を描き直さない（M-6・M-10。W-6 の Q-10 と同じ扱い）
- 開いている間にデモデータの日次リセット（`demo-data-reset.md`）が走ると、表示中の表現は古くなる。次の操作の応答（404・409 等）で知る。M-9 の扱いに従い、リセットそのものは読まない
- M-9 の文言は、利用者が理由を理解できるかを読まない（字面の一致だけを読む）。文言の妥当さはレビューで人間が承認する
- 実ブラウザでの `ConfirmDialog` のフォーカスの扱いは部品のテスト（`design-system.md` AC-6 の 6-9）に任せ、画面のテストでは読まない

---

## 人間の決定を待っている論点

Q-1〜Q-4 は 2026-10-29 に人間が明示選択で決め、各節へ反映した（Q-1 → L-2・L-3・L-4、Q-2 → L-2・T 節、Q-3 → L-2・T 節の限界、Q-4 → C-3・L-1・T 節）。

Q-5〜Q-8 とテストの差し替えの方針も 2026-10-29 に人間が明示選択で決め、各節へ反映した（Q-5 → W-2・W-3・T-2 の限界、Q-6 → W-5・T-2、Q-7 → C-2・W-6・T-2、Q-8 → W-1・W-4・W-6・T-2 と `design-system.md` 6-13 の改定（6-13 の行・6-13-ii〜6-13-v・10-3-p・10-6・10-6-q・11-7）、差し替えの方針 → T-0）。

Q-9・Q-10 も 2026-10-29 に人間が明示選択で決め、各節へ反映した（Q-9 = (a) → W-1・W-4・T-2、Q-10 → C-2・W-1・W-4・W-6・W-7・T-2。Q-10 により、月の境目に合わせたタイマーで描き直す設計を W-6・T-2 の限界から取り下げた。`design-system.md` 6-13 は再改定していない）。

Q-11（今月の入力の経路の置き場と、本人の契約を確かめる場所）は 2026-10-30 に人間が明示選択で決め、各節へ反映した（Q-11 = (e) 送るだけのページ → C-2・W-1・W-7・T-2）。(e) は、Q-10 の「本人の契約かをサーバー側で確かめずに送らない」を既存の条文のまま満たす置き場が無かったこと（ページは `lambda-client` を輸入できない＝`bff-wiring.md` 1-2・13-6、Route Handler に置くと `bff-wiring.md` の改定が要る）を受けて、確かめる場所を送り先の勤務月画面とドメイン API に移した決定である。`bff-wiring.md` と `docs/rules/architecture.md` は変えていない。決定の内容は W-7 が持つ。

Q-12〜Q-15（画面3）は 2026-10-30 に起票し、同日に人間が明示選択で決め、各節へ反映した。

- **Q-12 = (b)**（勤務月画面が本人かを知る手段）→ M-3・M-2・M-4・M-10・M-11・T-3。勤務月の表現（E-1・E-3〜E-7）に「操作者がその勤務月の技術者本人か」の真偽を足す。技術者の識別子は載せない。AC-3 は緩めない。ドメイン API の変更（契約 AC-2-5・AC-10-1・AC-10-3、`workmonth-implementation-design.md` の出力 DTO、Go の実装）は #55 とは別の Issue が行い、キー名もその Issue の仕様工程が決める。その Issue は 2026-10-30 に人間の承認を得て #155 として起票された。BFF は変えない（根拠は M-3）。採らなかった選択肢は、(a) ロールが `Approver` なら常に③として扱う（AC-3 ④ と承認 UI 3-1 の緩和が要る）、(c) E-8 の認可を緩める、(d) 本人の判定の経路を BFF に足す、であった
- **Q-13 = (a)**（勤務月の取得が成り立たないとき）→ M-9・T-3。取得の失敗は W-4 の取得エラーの文言を流用した `ErrorBanner`（`onRetry` 付き）、404・400 は1つの `EmptyState` に区分の既定の画面へのリンク。`EmptyState` の文言は AI 起案で人間の承認前
- **Q-14 = (b)**（操作が弾かれたときの文言）→ M-9・T-3。種類ごとに4つの文。文言は AI 起案で人間の承認前
- **Q-15 = (a)**（祝日データの用意）→ M-1・M-6・M-12・T-3。人間が原本を用意し、祝日のデータと `lib/holidays/` を画面3 の最後の段へ回す。原本はコミットしない

### 起案のまま残るもの

C-2 の `/work-months`・`/pending-approvals`・`/demo`・`/work-months/[contractId]/[yearMonth]`・`/this-month?contract=<契約ID>` のパスと、W 節・M 節のうち出所を書いていない決定（M-11 の全体と、M-12 のうち Q-15 の決定を除く部分を含む）（W-7 の関数の形・先読みの担保・契約 ID の読み方、W-8 の全体を含む）は AI の起案のまま人間の承認前であり、レビューで人間が確かめる対象として残る。`design-system.md` 6-13-v の観測可能な形（`button` のロール・`aria-expanded`・現れる位置・閉じると消えること）も AI の起案である。

レビューで人間が確かめる対象として、次も残る（未決の論点ではない。推測で埋めた箇所は無い）:

- **本人かの真偽が応答に無いときの扱い**（M-3。AI 起案）: `"unknown"` として操作要素を出さない。帰結として、#155 がドメイン API に入るまで、本物の環境の画面3 は閲覧だけになる。**#55 を #155 より先にマージ・デプロイしてよいか**は人間が確かめる
- **真偽の仮のキー名 `operatorIsOwner`**（M-3）: #155 の決定に従って改める。改める箇所は `lib/work-month.ts` の `OPERATOR_OWNERSHIP_KEY` と `readOwnership` だけである
- **M-9 の文言**（AI 起案・未承認）: `EmptyState` の title・description・リンクの文言（Q-13 = (a)）と、操作が弾かれたときの4つの文（Q-14 = (b)）
- **M-9 の分類の細部**（AI 起案）: R-1 の 403・409 を取得の失敗とすること、操作の失敗の `ErrorBanner` に再試行を出さないこと、409 の取り直しが失敗したときの表示
- **段の分け方と祝日データの形式**（M-1・M-12。AI 起案）: `work-month-holiday-view.tsx` を挟んで `lib/holidays/` を段2 に閉じ込める形、`workMonthScreen` が `view` を引数で受ける形、`HOLIDAY_ROWS` の形式と変換の規則（原本の書式の想定は原本を読んで確かめていない）、変換のスクリプトを置かないこと、`.gitignore` に原本を足すこと
- **利用条件の確認の受け渡し**（M-12。入力 UI 10-8）: 人間が原本とあわせて公表元の利用条件（URL と出典の表示について求める字面）を渡すことを段2 の前提とした。利用条件が画面への出典の表示を求めるなら、実装を止めて人間に諮る
- **転送の状態コード**: Q-10 は「302 で送る」と述べたが、Q-11 = (e) で選んだ `redirect()` は、URL を直接開いた要求に 307 を返す（W-7）。どちらも一時的な転送で送り先は同じであり、本仕様は状態コードを固定していない。302 そのものを要するなら、`redirect()` では満たせないため置き場を改める判断になる
