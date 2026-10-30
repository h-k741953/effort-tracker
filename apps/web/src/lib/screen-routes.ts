// docs/specs/web-screens-implementation-design.md C-2。
// 画面のパスはここで1か所に持つ。ページ・判定・リンクはこの定数を参照する。
// 勤務月一覧・承認待ち一覧・デモ勤務月群のパスは AI の起案で、人間の承認前である（C-2）。

/** ログイン画面。サインインの戻り先（`auth-handlers.ts` の SIGN_IN_LANDING_PATH）と同じ。 */
export const LOGIN_PATH = "/";

/** 技術者の既定の画面（本人の勤務月一覧）。 */
export const OWN_WORK_MONTHS_PATH = "/work-months";

/** 承認者の既定の画面（承認待ち一覧）。 */
export const PENDING_APPROVALS_PATH = "/pending-approvals";

/** ゲストの既定の画面（デモ勤務月群）。 */
export const DEMO_WORK_MONTHS_PATH = "/demo";

/** 勤務月画面（C-2）。各セグメントを符号化する。 */
export function workMonthPath(contractId: string, yearMonth: string): string {
  return `/work-months/${encodeURIComponent(contractId)}/${encodeURIComponent(yearMonth)}`;
}

/** 今月の入力の経路（C-2・W-7）。年月を含まない。契約 ID をクエリの値として符号化する。 */
export function thisMonthPath(contractId: string): string {
  return `/this-month?${new URLSearchParams({ contract: contractId }).toString()}`;
}

/** 本人の勤務月一覧（9-12 の受け口のクエリ `contract` を組み立てる）。 */
export function ownWorkMonthsPath(params: { contractId?: string }): string {
  if (params.contractId === undefined) return OWN_WORK_MONTHS_PATH;
  return `${OWN_WORK_MONTHS_PATH}?${new URLSearchParams({ contract: params.contractId }).toString()}`;
}

/** ヘルプ（C-2。ページは本仕様では置かない）。 */
export const HELP_PATH = "/help";

/** 契約一覧（C-2。ページは本仕様では置かない）。 */
export const CONTRACTS_PATH = "/contracts";

/** 稼働の推移（C-2。ページは本仕様では置かない）。 */
export const WORK_TRENDS_PATH = "/work-trends";
