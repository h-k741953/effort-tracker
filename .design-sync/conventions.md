# effort-tracker デザインシステムの使い方

SES／受託向けの勤怠・工数管理アプリの共通コンポーネント。画面の文言は日本語で書く。

## セットアップ

プロバイダやラッパーは要らない。`styles.css` を読み込めば、コンポーネントもトークンも効く。コンポーネントは `window.EffortTrackerDS` から使う（`AppHeader` / `RoleSwitcher` / `StatusBadge` / `Button` / `Card` / `NumberField` / `FieldError` / `ErrorBanner` / `EmptyState` / `ConfirmDialog` / `Pagination`）。

## スタイルの書き方（Tailwind のユーティリティクラス）

色は必ずトークンのクラスで当てる。パレット色（`bg-blue-500` など）・16進や `rgb()` の直書き・`style` 属性での色指定は使わない（CSS に含まれていない）。

| 用途 | クラス |
|---|---|
| ページの地と文字 | `bg-background` / `text-foreground` |
| カード・面 | `bg-surface` / `text-surface-foreground` |
| 補足の文字 | `text-muted-foreground` |
| 枠線 | `border` + `border-border` |
| 主要操作 | `bg-primary` / `text-primary-foreground` |
| 危険・エラー | `bg-danger` / `text-danger-foreground`、文字だけなら `text-danger`、枠は `border-danger` |
| 状態バッジの地と文字 | `bg-state-draft` / `bg-state-pending-approval` / `bg-state-approved` と、それぞれの `text-state-*-foreground` |

- 角丸は `rounded-md` だけ（`rounded-lg` / `rounded-full` などは CSS に無い）。
- 自作の対話要素には、フォーカスリングとして `focus-visible:ring-2 focus-visible:ring-focus-ring` を付ける。
- 配置は `flex` / `grid` / `gap-{1,2,3,4,6,8}` / `p-{2,3,4,6}` / `mx-auto max-w-{3xl,4xl,5xl}` / `grid-cols-{2,3,4}` / `text-{sm,base,lg,xl}` / `font-{medium,semibold}` を使う。CSS に入っているクラスだけが効くので、ここに無い間隔や大きさのクラスは避ける。

## コンポーネントの約束

- 状態は `StatusBadge` で表す（`Draft`→下書き / `PendingApproval`→締め済 / `Approved`→承認済）。色を自作しない。
- 操作は `Button`（`variant`: `primary` / `secondary`（既定）/ `danger`）。取り消せない操作の前は `ConfirmDialog` で確認する。
- 入力欄は `NumberField`、その誤りは `FieldError` を `errorId` で結んで出す。画面全体の失敗は `ErrorBanner`、空の一覧は `EmptyState`。
- props の正確な定義は `components/general/<Name>/<Name>.d.ts`、使用例は `<Name>.prompt.md` を見る。トークンの値は `styles.css` が読み込む `_ds_bundle.css`。

## 例

```jsx
const { AppHeader, Card, StatusBadge, Button } = window.EffortTrackerDS;

<div className="min-h-screen bg-background text-foreground">
  <AppHeader />
  <main className="mx-auto flex max-w-3xl flex-col gap-4 p-4">
    <Card title="2026年9月の勤務月">
      <div className="flex items-center justify-between gap-4 text-sm">
        <span>合計 152.5 時間</span>
        <StatusBadge state="Draft" />
      </div>
    </Card>
    <div className="flex justify-end gap-2">
      <Button variant="primary">締める</Button>
    </div>
  </main>
</div>
```
