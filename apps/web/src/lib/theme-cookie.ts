// docs/specs/design-system.md AC-12-3 / AC-12-4 / AC-12-6
//
// - 12-3: 保存するのは light / dark の値だけ。署名しない・認証の情報と結び付けない
//   （値は見た目にしか効かない）。スクリプトから書くため HttpOnly を持たない。
// - 12-4: 保存が無いとき・読めない値のときは常に light（OS の設定を使わない）。
// - 12-6: ブラウザからも使うため、Node 専用のモジュールを輸入しない。

/** 12-6: テーマは 2 つだけ。 */
export type Theme = "light" | "dark";

/** 12-6: テーマ Cookie の名前（role-cookie.ts の `et_role` に倣う）。 */
export const THEME_COOKIE_NAME = "et_theme";

/** 保持の期間（1年）。値は仕様で固定しない（12-6）。 */
const MAX_AGE_SECONDS = 60 * 60 * 24 * 365;

/**
 * Cookie の値から Theme を決める。`light` / `dark` に字面で一致しないものは
 * すべて `light`（12-4）。
 */
export function resolveTheme(cookieValue: string | undefined): Theme {
  return cookieValue === "dark" ? "dark" : "light";
}

/** 選択を Set-Cookie 形式の文字列にする（12-6）。 */
export function serializeThemeCookie(theme: Theme): string {
  return `${THEME_COOKIE_NAME}=${theme}; Path=/; SameSite=Lax; Secure; Max-Age=${MAX_AGE_SECONDS}`;
}
