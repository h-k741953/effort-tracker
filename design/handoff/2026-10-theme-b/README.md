# Handoff: effort-tracker 画面デザイン（テーマ B：リッチなダッシュボード風）

## Overview
SES／受託向けの勤怠・工数管理アプリ「effort-tracker」の画面一式。技術者が日ごとの稼働実績（開始・終了・休憩）を入力して勤務月を締め、承認者が承認または差戻しする。ゲストはデモ用サンプルを閲覧のみ。画面の文言はすべて日本語。

## About the Design Files
このフォルダの `.dc.html` は **HTML で作ったデザインの参照物（プロトタイプ）** であり、そのまま本番に使うコードではない。見た目と振る舞いの意図を示すもの。
実装では、対象コードベースの既存環境（React など）と既存の部品・作法で **再実装** すること。コンポーネントは **effort-tracker Design System（`window.EffortTrackerDS`、web@0.1.0）** の AppHeader / RoleSwitcher / StatusBadge / Button / Card / NumberField / FieldError / ErrorBanner / EmptyState / ConfirmDialog / Pagination をそのまま使い、同じ役割の部品を自作しない。テーマ B の見た目は DS 部品の上から `et-theme.css` で上書きしている（DS の props では表現できないため）。本番では DS 側のトークン・バリアントとして取り込むのが望ましい。

ブラウザで開く場合: 各 `.dc.html` はローカルサーバー経由で開く（`npx serve .` など）。`support.js` と `_ds/` は同梱済み。

## Fidelity
**High-fidelity。** 色・タイポ・余白・角丸・影・動き・文言は最終。ピクセル単位で再現すること。

## 業務上の決まり（変えない）
- 勤務月の状態は 3 つ。必ず StatusBadge で表す: `Draft`＝下書き／`PendingApproval`＝締め済／`Approved`＝承認済。
- その場で使えない操作は **非表示**（disabled で残さない）。例外: Pagination の「前へ」「次へ」は常に表示し、使えない側を非活性（opacity .45、cursor:not-allowed）。
- 画面に出さない: 通知（ベル・メール）、ユーザー・ロール管理画面、承認者の名前、差戻しの理由、精算金額。
- 締めは技術者が取り消せない。下書きに戻す手段は承認者の差戻しだけ。自分の勤務月は自己承認できない（承認・差戻しボタンを出さない）。
- 取り消せない操作（締める・承認・差戻し）の前は ConfirmDialog。差戻しに理由入力欄は設けない。

## 共通レイアウト（シェル）
- ルート: `data-et="light|dark"` を付けた全画面の横並び（サイドバー＋本体）。ログイン画面のみサイドバーなし。
- **サイドバー**（`SideNav.dc.html`）: 幅 200px、地 `--et-sidebar`、上下 padding 24px・左右 16px。見出し「メニュー」（11px/700/letter-spacing .1em）。項目は 14px/700、padding 10px 12px、角丸 10px、先頭に 8px の丸。選択中は背景 `--et-sidebar-active`、丸がグラデーション。ホバーで右へ 2px。最下部に区切り線（1px `--et-sidebar-active`）を挟んで「ヘルプ」を固定（margin-top:auto）。
  - 技術者: 今月の入力／自分の勤務月／契約一覧／稼働の推移 ＋ ヘルプ
  - 承認者: 承認待ち／承認担当者一覧 ＋ ヘルプ
  - ゲスト: サンプル勤務月 ＋ ヘルプ
  - 760px 以下: 上部の横並びに変形、見出し非表示。
- **ヘッダー**（AppHeader）: padding 14px 32px、下線 1px `--border`、地 `--background`。
  - 左: ロゴ。30×30px・角丸 9px のグラデーション角丸四角に時計（円＋針）を白抜きしたマーク ＋「effort-tracker」（Space Grotesk 19px/700、letter-spacing -.03em、gap 10px）。DS の AppHeader はロゴ差し替え不可のため CSS で付与している。正式ロゴ画像があれば差し替える。
  - 右（gap 16px）: RoleSwitcher（技術者／承認者、ゲスト時は出さない）→ テーマ切替（ライト／ダーク、太陽・月アイコン）→ ゲスト時のみ「ログイン」リンク。
  - 2 つの切替は同じセグメント型: 外枠 padding 3px・地 `--et-hover`・枠 1px `--border`・角丸 13px。項目 padding 6px 14px（テーマは 6px 12px）、13px/700。ロール切替の選択中はグラデーション地＋白文字、テーマ切替の選択中は `--surface` 地＋影。
  - テーマは localStorage `et-theme` に保存し、画面遷移後も引き継ぐ。
- **本体 main**: `max-width:1024px; width:100%; margin:0 auto; padding:24px; gap:32px`（縦並び）。width:100% は必須（中身が無いときに縮まないように）。
- 見出し h1: 24px/700、letter-spacing -.01em。補足: 14px `--muted-foreground`。
- 見出しの上の現在地表示（勤務月画面）: 「自分の勤務月 ＞ 2026年9月」。左はリンク（承認者は「承認待ち」、ゲストは「サンプル勤務月」）。

## Screens / Views

### 1. ログイン（Login.dc.html）
- 入口 3 つ: (a)「ゲストとして続行（閲覧のみ）」＋注記「入力・締め・承認はできません。」 (b) メールアドレス＋パスワード（「ログイン」「新規登録」切替） (c)「Google でログイン」。
- 多要素認証: 別状態「2 段階認証」。カード「認証アプリのコード」、6 桁の確認コード入力、「確認する」（primary）、「別のアカウントでログイン」。SMS は出さない。
- 失敗状態: ErrorBanner「メールアドレスまたはパスワードが正しくありません」。メールアドレスは残し、パスワードは空にする。機密は画面に表示しない。

### 2. ゲスト向けデモ一覧（Guest Demo List.dc.html）
- 注記（role="note"）: 「デモ用のサンプルです（閲覧のみ・毎日リセットされます）」。
- 表（下記「一覧の表」）: 契約／対象年月／状態／「表示する」。年月降順、1 ページ 5 件、Pagination。
- 行を開くと閲覧専用の勤務月（要約カード 3 枚＋カレンダー）。操作ボタンなし。

### 3. 自分の勤務月（My Months (Engineer).dc.html）— 技術者
- 「直近の勤務月」カード（横長）: 左に年月（24px/700）・契約名・StatusBadge・説明文、右に「直近の勤務月を開く」（primary）。
- 絞り込みバー（role="search"）: 契約（幅 300px）／対象年月（160px）／状態（160px）のプルダウン、各「すべて」から。絞り込み中だけ「条件をクリア」。右端に件数「全 16 件」／「16 件中 3 件」（aria-live）。条件変更でページ 1 に戻る。
- 表: 契約／対象年月／状態／「開く」。年月降順、1 ページ 6 件。
- 0 件（絞り込み結果）: EmptyState「条件に合う勤務月はありません」＋「条件をクリア」。
- 別状態: 勤務月 0 件 → EmptyState「勤務月がまだありません」＋「稼働実績を入力する」。読み込み失敗 → ErrorBanner＋再試行。
- `?c=<契約名>` で契約の絞り込み済みで開く（契約一覧から遷移時）。

### 4. 承認待ち（Pending Approvals (Approver).dc.html）— 承認者
- 締め済（PendingApproval）のみ。承認済・下書きは載せない。承認済の一覧画面は作らない。
- 絞り込み: 契約・対象年月（状態は締め済のみのため無し）。
- 表: 契約／対象年月／状態（締め済）／「開く」。一覧上で承認・差戻しはさせない。
- 0 件: EmptyState「承認待ちの勤務月はありません」。

### 5. 勤務月画面（WorkMonthScreen.dc.html）
URL パラメータ: `variant`（draft-engineer / pending-engineer / pending-approver / pending-self-approver / approved / guest）, `y`, `m`, `c`（契約名）, `scenario`, `nav=current`。

- 見出し: 「<契約名> × <YYYY>年<M>月」＋ StatusBadge（状態変化時にポップ）＋ 状態に応じた説明文。
- **要約カード 3 枚**（横 1 列固定、`grid-template-columns:repeat(3,minmax(0,1fr))`, gap 16px、高さ揃え、ホバーで 4px 浮く）:
  1. 総稼働時間 2. 精算幅「140〜180」 3. 超過／不足
  - 下書き: 3 枚目は「締め時に確定します」（数値を出さない）。
  - 締め済・承認済: 3 枚目の中に「超過」「不足」を 2 列。例: 総稼働 185.0 → 超過 5.0・不足 0.0。
  - 数値: Space Grotesk、`clamp(26px,3vw,44px)`/700、グラデーション文字、単位「時間」14px muted。
- **2 カラム**（flex-wrap, gap 32px）: 左 カレンダー（flex 2 1 520px）、右 サイド（flex 1 1 280px）。操作が無い状態ではカレンダーが全幅。
- **カレンダー**: 月曜始まり、7 列、gap 4px。マス min-height 76px、padding 8px、角丸 10px、枠 1px `--border`。
  - 日付（12px/600）。土曜は青（`--et-sat` 文字、`--et-sat-bg` 地）、日曜・祝日は赤（`--et-sun` / `--et-sun-bg`）。曜日見出しの「土」「日」も同色。祝日のみ「祝」の札（11px/700、padding 1px 6px、角丸 6px、`--et-sun-chip` 地）。土日に札は出さない。
  - 入力済み: 右寄せで稼働時間（14px/600）、その下に「9:00〜18:00」（11px/600 muted）。未入力は空欄。
  - 編集可（下書き・本人）: マスはボタン。選択中は枠 2px `--primary`。ホバーで地 `--background`。
  - 各日の合計＝総稼働時間。
- **稼働実績の入力**（右サイドのカード、日を選ぶと表示）:
  - 対象日（18px/700）。未来の日 → FieldError「未来の日付には入力できません」、入力欄と保存は出さない。
  - 勤務時間: 開始・終了。休憩: 「休憩あり」チェック（初期オン）＋休憩開始・休憩終了（初期 12:00／13:00）。チェックを外すと休憩欄が消える。
  - 各時刻は **時と分の 2 つのプルダウン（選択のみ、入力不可）**。時 0〜23、分 00/15/30/45、未選択「--」。幅 84px、高さ 42px、角丸 10px、枠 1px `--et-input-border`、右に ▾。横に「時」「分」の文字。aria-label「開始（時）」等。
  - 稼働時間（自動計算・aria-live）: `終了−開始−（休憩あり なら 休憩終了−休憩開始）`。大きく表示＋内訳「9:00〜18:00（休憩 1:00）」。
  - 検証（各行の下に FieldError、該当プルダウンを赤枠）: 未選択で保存「時刻を選んでください」／「時を選んでください」／「分を選んでください」、「終了は開始より後の時刻にしてください」、「休憩の終了は休憩の開始より後にしてください」、「休憩は勤務時間（開始〜終了）の中に収めてください」、「稼働時間が 0 になります。休憩の時刻を見直してください」。
  - ボタン: 「保存」（上書き）、「削除」（入力済みの日のみ。未入力に戻す）、「閉じる」。
  - 実績 0 件の月: EmptyState「まだ稼働実績がありません」＋「稼働実績を入力する」（最初の平日を選択）。「月を開く」ボタンは作らない（最初の入力で自動作成）。
- **締める**（下書き・本人・実績 1 件以上）: サイドのカード「勤務月を締める」＋「締める」（primary）。
- **承認・差戻し**（締め済・本人でない承認者のみ）: 「差戻し」（danger）、「承認」（primary）。
- **ConfirmDialog**（重ね幕 `--et-overlay`＋blur 6px、ダイアログ最大幅 480px、角丸 16px、padding 28px）:
  - 締め: タイトル「2026年9月の勤務月を締めますか？」本文「締めると稼働実績は編集できなくなり、ご自身で取り消すことはできません。」「下書きに戻す方法は、承認者による差戻しだけです。」ボタン「取消」（下書きのまま戻る）／「締める」（締め済へ）。未入力の日があっても警告しない。
  - 承認: 「この勤務月を承認しますか？」／「承認すると、この勤務月の稼働実績が確定します。」
  - 差戻し: 「この勤務月を差し戻しますか？」／「差し戻すと、技術者が稼働実績を修正できる下書きの状態に戻ります。」（confirm は danger）
- 締めた直後: StatusBadge 締め済、超過／不足表示、入力欄と締めるボタンが消え「締めました。承認されるまで稼働実績は編集できません。」
- 状態とロールの見え方一覧は `Work Month States.dc.html` を参照（1a〜1f、2a〜2d、3a〜3b）。

### 6. 契約一覧（Contracts.dc.html）— 技術者
- 表: 契約（300）／精算幅（120）／契約期間（200）／今月の状態（110）／「勤務月を見る」（120）。押すと自分の勤務月を `?c=` で絞り込んで開く。

### 7. 稼働の推移（Trends.dc.html）— 技術者
- 絞り込み: 契約、期間（開始）〜（終了）。終了の選択肢は開始以降のみ。契約変更で期間は全期間に戻る。
- 要約カード 3 枚: 平均稼働（確定した月）／精算幅に収まった月「9 / 11 か月」／精算幅。
- 棒グラフ（高さ 360px）:
  - 縦軸は 0 始まりではなく、データに合わせて自動: `lo = floor((min(精算幅下限, 確定月の最小) − 20) / 10) × 10`、`hi = ceil((max(精算幅上限, 確定月の最大) + 10) / 10) × 10`。下端に省略の印と目盛り、グラフ下に「縦軸は N 時間から始まります（入力中の月は下端に表示）。」
  - 精算幅は帯（primary 10%）＋上下 2px 実線、左に目盛り（primary 色）。
  - 棒: 最大幅 40px、角丸上 8px、グラデーション。超過分は上限線より上を赤の斜線で積む、不足分は下限線まで赤の点線枠。札「超過 5」「不足 2.5」（10px/700 赤）。数値ラベルは帯より前面、地の色の背景。
  - 入力中（下書き）の月は斜線＋点線の棒、数値に「（入力中）」、平均から除外。
  - 棒は下から伸びる出現アニメ。role="img" に全月の読み上げ文。

### 8. 承認担当者一覧（Engineers.dc.html）— 承認者
- 表: 技術者（150）／契約（300）／2026年10月（120）／2026年9月（120）／操作（150）。閲覧のみ。締め済の月がある人だけ「承認待ちを開く」。

### 9. ヘルプ（Help.dc.html）— 全ロール（`?role=`）
- カード 3 枚: 勤務月の流れ（StatusBadge 3 つ＋説明）／稼働実績の入力（定義リスト）／精算幅と超過／不足。

## 一覧の表（共通）
- `role="table"`、各行 `display:grid` で **列幅固定**（行と見出しがずれないこと）:
  - 勤務月系: `480px 150px 130px 120px`、column-gap 12px、行 padding 16px 12px。
  - 契約一覧: `300px 120px 200px 110px 120px`／承認担当者: `150px 300px 120px 120px 150px`。
- セル内は折り返さない（white-space:nowrap）。行の角丸なし、区切りは 2 行目以降の上線 1px `--border`。ホバーで行の地 `--background`（動かさない）。見出し行 12px/500 muted、下線。

## Interactions & Behavior
- ボタン: ホバーで 1px 上へ、押下で scale(.97)。primary はグラデーション地＋影（ホバーで影が強まる）。secondary のホバー地 `--et-hover`。danger のホバー brightness(.92)。
- 要約カード: ホバーで translateY(-4px)、影 `--et-shadow-hover`。
- 出現: main 直下の要素を上から順に translateY(14px)→0＋フェード、420ms、70ms ずつ遅延。
- StatusBadge の状態変化: scale(.7)→1 のポップ。ダイアログ: translateY(10px) scale(.95)→1。
- テーマ切替: 色の移り変わり 240ms ease。
- フォーカス: `outline:2px solid var(--focus-ring); outline-offset:2px`。
- `prefers-reduced-motion: reduce` で全アニメーション・トランジション・変形を止める。
- 遷移: 一覧の「開く」→ 勤務月画面（variant/y/m/c を URL で渡す）。ロール切替は一覧画面では相手ロールの一覧へ遷移、勤務月画面ではその場で操作が切り替わる。

## State Management
- 勤務月: `{ contract, y, m, status: Draft|PendingApproval|Approved, days: { [d]: { sh, sm, eh, em, brk, bsh, bsm, beh, bem } } }`。稼働分は時刻から算出して保持・合計。
- 遷移: Draft →(締める)→ PendingApproval →(承認)→ Approved ／ PendingApproval →(差戻し)→ Draft。
- 権限: 編集・締めは「下書き かつ 本人（技術者）」。承認・差戻しは「締め済 かつ 承認者 かつ 本人でない」。
- 今日（未来日判定）: プロトタイプは 2026-10-02 固定。
- 一覧: ページ、絞り込み条件（契約・年月・状態）、ホバー行。テーマは localStorage。
- プロトタイプでは画面間でデータを共有していない（締め・承認の結果は一覧に戻ると元に戻る）。実装では API と共有状態で一貫させる。

## Design Tokens（テーマ B）
| トークン | ライト | ダーク | 用途 |
|---|---|---|---|
| --background | #eef1f7 | #0b1020 | ページの地 |
| --surface | #ffffff | #131a2e | カード・ダイアログ |
| --foreground | #0f172a | #e6e9f2 | 本文 |
| --muted-foreground | #475569 | #9aa4bd | 補足・単位 |
| --border | #dfe4ee | #243049 | 枠・区切り |
| --primary | #4338ca | #818cf8 | 主要操作・リンク |
| --primary-foreground | #ffffff | #0b1020 | 主要操作の文字 |
| --et-primary-hover | #3730a3 | #a5b4fc | 主要操作ホバー |
| --et-grad-from / -to | #4338ca / #0369a1 | #818cf8 / #38bdf8 | グラデーション |
| --danger / -foreground | #be123c / #ffffff | #fb7185 / #0b1020 | 差戻し・エラー |
| --et-danger-soft | #fff1f2 | #2a1424 | ErrorBanner の地 |
| --focus-ring | #4338ca | #a5b4fc | フォーカス |
| --et-hover | #f1f4fb | #1a2340 | ホバー面 |
| --et-soft | #eef2ff | #1b2442 | 淡い面 |
| --et-input-border | #7c8799 | #6b7799 | 入力欄の枠 |
| --et-sat / -bg | #1d4ed8 / #eff4ff | #93c5fd / #141e38 | 土曜 |
| --et-sun / -bg / -chip | #be123c / #fff1f2 / #ffe4e8 | #fda4af / #241526 / #3a1a2c | 日曜・祝日 |
| --et-sidebar / -foreground / -muted / -active | #0f172a / #e2e8f0 / #94a3b8 / #1e293b | #070b16 / #e6e9f2 / #9aa4bd / #131a2e | サイドバー |
| --state-draft / -foreground | #e2e8f0 / #334155 | #2a3350 / #dbe1ef | 下書きバッジ |
| --state-pending-approval / -foreground | #fef3c7 / #92400e | #3d2a0a / #fcd34d | 締め済バッジ |
| --state-approved / -foreground | #d1fae5 / #065f46 | #0c3324 / #6ee7b7 | 承認済バッジ |

本文のコントラストはライト・ダークとも WCAG AA（4.5:1 以上）を確認済み。

- 角丸: カード・ダイアログ 16px／ボタン・切替・入力 10px／日セル 10px／バッジ 8px（先頭に 6px の丸）。
- 影: 標準 `0 1px 3px rgba(15,23,42,.06), 0 12px 32px -14px rgba(15,23,42,.22)`、ホバー `0 2px 6px rgba(15,23,42,.08), 0 28px 48px -18px rgba(67,56,202,.40)`、ダイアログ `0 32px 80px -20px rgba(15,23,42,.45)`（ダークは et-theme.css 参照）。
- 動き: ホバー・押下 220ms、出現 420ms、イージング `cubic-bezier(.16,1,.3,1)`、時間差 70ms、テーマ 240ms。
- フォント: 本文 Zen Kaku Gothic New（400/500/700）、数字 Space Grotesk（500/600/700）＋ tabular-nums。
- 余白: 画面 24px、セクション間 32px、カード内 24px。

## Assets
- フォント: Google Fonts（Zen Kaku Gothic New, Space Grotesk）。
- ロゴのマーク: CSS のグラデーション＋インライン SVG マスク（仮）。
- 画像・アイコン素材なし（テーマ切替の太陽・月は `et-shell.js` 内のインライン SVG）。

## サンプルデータ（仮の値）
- 契約: 株式会社サンプル 基幹システム開発（精算幅 140〜180 時間）、株式会社サンプル 保守運用支援（120〜160 時間、仮）。契約期間・稼働の推移の値・技術者名は仮。

## Screenshots（`screenshots/`、ライトテーマ、幅約 900px のプレビューで撮影）
- `01-login.png` ログイン
- `02-guest-demo-list.png` ゲスト向けデモ一覧
- `03-my-months.png` / `03b-my-months-table.png` 自分の勤務月（直近カード・絞り込み／表）
- `04-pending-approvals.png` 承認待ち
- `05-work-month-draft.png` 勤務月（下書き・技術者本人、要約カード）
- `06-work-month-draft-input.png` 同（カレンダーと稼働実績の入力）
- `07-work-month-confirm-close.png` 締めの確認ダイアログ
- `08-work-month-pending-approver.png` 締め済・承認者（承認／差戻し）
- `09-work-month-pending-engineer.png` 締め済・技術者本人（閲覧のみ）
- `10-work-month-input-error.png` 入力エラー（終了が開始より前）
- `11-work-month-dark.png` ダークテーマ
- `12-contracts.png` 契約一覧
- `13-trends.png` / `13b-trends-chart.png` 稼働の推移（絞り込み・要約／グラフ）
- `14-engineers.png` 承認担当者一覧
- `15-help.png` ヘルプ

プレビュー幅が狭いため、一部の画面は右端が切れている。正確な見た目は各 `.dc.html` をブラウザ（幅 1280px 以上）で開いて確認すること。

## Files
- `Login.dc.html` ログイン
- `Guest Demo List.dc.html` ゲスト向けデモ一覧
- `My Months (Engineer).dc.html` 自分の勤務月
- `Pending Approvals (Approver).dc.html` 承認待ち
- `WorkMonthScreen.dc.html` 勤務月画面（状態・ロールは URL パラメータ）
- `Work Month States.dc.html` 勤務月画面の状態一覧（キャンバス）
- `Contracts.dc.html` 契約一覧／`Trends.dc.html` 稼働の推移／`Engineers.dc.html` 承認担当者一覧／`Help.dc.html` ヘルプ
- `SideNav.dc.html` サイドバー
- `et-theme.css` テーマ B の全トークンと DS 部品への上書き
- `et-shell.js` テーマ保存とヘッダー右側（ロール切替・テーマ切替）
- `_ds/…` effort-tracker Design System のバンドル、`support.js` プロトタイプ実行用
