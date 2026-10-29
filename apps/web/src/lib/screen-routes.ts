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
