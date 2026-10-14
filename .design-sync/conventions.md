# effort-tracker デザインシステムの使い方

SES／受託向けの勤怠・工数管理アプリの共通コンポーネント。画面の文言は日本語で書く。

## セットアップ

プロバイダやラッパーは要らない。`styles.css` を読み込めば、コンポーネントもトークンもフォント（Zen Kaku Gothic New・Space Grotesk）も効く。コンポーネントは `window.EffortTrackerDS` から使う（`AppHeader` / `SideNav` / `ThemeToggle` / `RoleSwitcher` / `StatusBadge` / `Button` / `Card` / `SelectField` / `FieldError` / `ErrorBanner` / `EmptyState` / `ConfirmDialog` / `Pagination`）。

明色が既定。暗色にするときは、画面の最上位の要素（`<html>` など）に `data-theme="dark"` を付ける。トークンの値が切り替わるので、クラスを変える必要は無い（`dark:` のクラスは使わない）。

## スタイルの書き方（Tailwind のユーティリティクラス）

色は必ずトークンのクラスで当てる。パレット色（`bg-blue-500` など）・16進や `rgb()` の直書き・`style` 属性での色指定は使わない（CSS に含まれていない）。

| 用途 | クラス |
|---|---|
| ページの地と文字 | `bg-background` / `text-foreground` |
| カード・面 | `bg-surface` / `text-surface-foreground` |
| 補足の文字 | `text-muted-foreground` |
| 枠線 | `border` + `border-border`、入力欄の枠は `border-et-input-border` |
| 主要操作 | `bg-primary` / `text-primary-foreground` |
| ホバーの面・淡い面 | `bg-et-hover` / `bg-et-soft` |
| 危険・エラー | `bg-danger` / `text-danger-foreground`、文字だけなら `text-danger`、枠は `border-danger`、淡い地は `bg-et-danger-soft` |
| 状態バッジの地と文字 | `bg-state-draft` / `bg-state-pending-approval` / `bg-state-approved` と、それぞれの `text-state-*-foreground` |
| カレンダーの土曜・日曜祝日 | `text-et-sat` / `bg-et-sat-bg`、`text-et-sun` / `bg-et-sun-bg`、祝日の札は `bg-et-sun-chip` |
| サイドバー | `bg-et-sidebar` / `text-et-sidebar-muted`、選択中は `bg-et-sidebar-active` / `text-et-sidebar-foreground` |

- 角丸はトークンで当てる: カードと面は `rounded-(--et-radius-card)`、ボタン・入力欄は `rounded-(--et-radius-control)`、バッジは `rounded-(--et-radius-badge)`、小さな札は `rounded-(--et-radius-chip)`。Tailwind 既定の角丸（md・lg などの段階）は CSS に無い。
- 影は `shadow-(--et-shadow)`、持ち上げは `shadow-(--et-shadow-hover)`、ダイアログは `shadow-(--et-shadow-dialog)`。
- 時間・件数などの数字は `font-numeric` と `tabular-nums` で書く（本文は既定の `font-sans`）。
- 自作の対話要素には、フォーカスリングとして `focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background` を付ける。
- 配置は `flex` / `grid` / `gap-{1,2,3,4,6,8}` / `p-{2,3,4,6}` / `mx-auto max-w-{3xl,4xl,5xl}` / `grid-cols-{2,3,4}` / `text-{sm,base,lg,xl}` / `font-{medium,semibold}` を使う。CSS に入っているクラスだけが効くので、ここに無い間隔や大きさのクラスは避ける。

## コンポーネントの約束

- 状態は `StatusBadge` で表す（`Draft`→下書き / `PendingApproval`→締め済 / `Approved`→承認済）。色を自作しない。
- 操作は `Button`（`variant`: `primary` / `secondary`（既定）/ `danger`）。取り消せない操作の前は `ConfirmDialog` で確認する。
- 選択の入力は `SelectField`（選択肢は `options` で渡す。時刻なら時 0〜23・分 0/15/30/45）、その誤りは `FieldError` を `errorId` で結んで出す。画面全体の失敗は `ErrorBanner`、空の一覧は `EmptyState`。
- 画面の左端に `SideNav`（項目は `items`、ヘルプなどは `footerItems`）、上端に `AppHeader`。明暗の切替は `ThemeToggle`、デモ用のロール切替は `RoleSwitcher`。
- props の正確な定義は `components/general/<Name>/<Name>.d.ts`、使用例は `<Name>.prompt.md` を見る。トークンの値は `styles.css` が読み込む `_ds_bundle.css`。

## 例

```jsx
const { AppHeader, SideNav, Card, StatusBadge, Button } = window.EffortTrackerDS;

<div className="flex min-h-screen bg-background text-foreground">
  <SideNav items={[{ label: "自分の勤務月", href: "/work-months", current: true }]} />
  <div className="flex flex-1 flex-col">
    <AppHeader />
    <main className="mx-auto flex w-full max-w-3xl flex-col gap-4 p-4">
      <Card title="2026年9月の勤務月">
        <div className="flex items-center justify-between gap-4 text-sm">
          <span className="font-numeric tabular-nums">合計 152.5 時間</span>
          <StatusBadge state="Draft" />
        </div>
      </Card>
      <div className="flex justify-end gap-2">
        <Button variant="primary">締める</Button>
      </div>
    </main>
  </div>
</div>
```
