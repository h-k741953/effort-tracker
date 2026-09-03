// BFF が環境変数から受け取る認証の構成（docs/specs/bff-auth-termination.md
// AC-7-5・AC-5-5 (viii)）。
//
// - 環境変数の名の実体は構成側（infra/terraform/lambda_bff_ssr.tf の
//   environment ブロック）と本ファイルが持つ。仕様は役割で参照する（AC-7-5）。
//   **名の一致は機械検査されない**（限界 10-2 / 10-10）。
// - **未設定・空のときは、既定値へ黙って落ちず、推測もせず、失敗として扱う。**
// - AC-4-1: 値（とくに署名鍵）をログ・エラー文言・応答へ出さない。失敗の結果に
//   載せるのは**変数の名前だけ**であり、値は載せない。

/** 構成の読み取りに使う環境（テスト・呼び出し側から差し替えられる形にする）。 */
export type Environment = Record<string, string | undefined>;

export interface AuthConfig {
  readonly region: string;
  readonly userPoolId: string;
  readonly clientId: string;
  /** ホストされたサインイン画面のドメインのプレフィックス（AC-11-2 (i)）。 */
  readonly hostedUiDomainPrefix: string;
  /** AC-2-4: 期待する発行者。サーバー側の構成から導く。 */
  readonly issuer: string;
  /** ホストされたサインイン画面・トークンエンドポイントの基底 URL。 */
  readonly hostedUiBaseUrl: string;
}

export interface ConfigLoaded<T> {
  readonly ok: true;
  readonly value: T;
}

export interface ConfigMissing {
  readonly ok: false;
  /** 与えられていない変数の**名前**（値は持たない）。 */
  readonly missing: readonly string[];
}

export type ConfigResult<T> = ConfigLoaded<T> | ConfigMissing;

const REGION_VAR = "COGNITO_REGION";
const USER_POOL_ID_VAR = "COGNITO_USER_POOL_ID";
const CLIENT_ID_VAR = "COGNITO_CLIENT_ID";
const DOMAIN_PREFIX_VAR = "COGNITO_DOMAIN_PREFIX";
const ROLE_COOKIE_SIGNING_KEY_VAR = "ROLE_COOKIE_SIGNING_KEY";
/** AC-7-6: サインインの戻り先の組み立てに用いる公開オリジン（構成側が注入）。 */
const PUBLIC_ORIGIN_VAR = "PUBLIC_ORIGIN";

function read(environment: Environment, name: string): string | undefined {
  const value = environment[name];
  if (typeof value !== "string" || value.length === 0) {
    return undefined;
  }
  return value;
}

/**
 * トークン検証・サインインに要する構成を読む。1 つでも欠けていれば失敗を返す
 * （既定値を作らない＝AC-7-5）。
 */
export function loadAuthConfig(
  environment: Environment = process.env,
): ConfigResult<AuthConfig> {
  const region = read(environment, REGION_VAR);
  const userPoolId = read(environment, USER_POOL_ID_VAR);
  const clientId = read(environment, CLIENT_ID_VAR);
  const hostedUiDomainPrefix = read(environment, DOMAIN_PREFIX_VAR);

  const missing: string[] = [];
  if (region === undefined) missing.push(REGION_VAR);
  if (userPoolId === undefined) missing.push(USER_POOL_ID_VAR);
  if (clientId === undefined) missing.push(CLIENT_ID_VAR);
  if (hostedUiDomainPrefix === undefined) missing.push(DOMAIN_PREFIX_VAR);
  if (
    region === undefined ||
    userPoolId === undefined ||
    clientId === undefined ||
    hostedUiDomainPrefix === undefined
  ) {
    return { ok: false, missing };
  }

  return {
    ok: true,
    value: {
      region,
      userPoolId,
      clientId,
      hostedUiDomainPrefix,
      issuer: `https://cognito-idp.${region}.amazonaws.com/${userPoolId}`,
      hostedUiBaseUrl: `https://${hostedUiDomainPrefix}.auth.${region}.amazoncognito.com`,
    },
  };
}

/**
 * デモ用ロール切替 Cookie の署名鍵を読む（AC-5-5 (viii)・Q-F = (b)）。
 * **未設定・空なら失敗**であり、既定値も生成鍵も使わない —— 予約同時実行数 5 で
 * インスタンスが分かれるため、インスタンスごとに生成した鍵では検証できない。
 */
export function loadRoleCookieSigningKey(
  environment: Environment = process.env,
): ConfigResult<string> {
  const signingKey = read(environment, ROLE_COOKIE_SIGNING_KEY_VAR);
  if (signingKey === undefined) {
    return { ok: false, missing: [ROLE_COOKIE_SIGNING_KEY_VAR] };
  }
  return { ok: true, value: signingKey };
}

/**
 * 公開オリジンを読む（AC-7-6・Q-H = (a)）。**未設定・空なら失敗**であり、
 * 既定値へ黙って落ちず、要求元のホストへも推測で落ちない（AC-7-6 (ii)）。
 */
export function loadPublicOrigin(
  environment: Environment = process.env,
): ConfigResult<string> {
  const publicOrigin = read(environment, PUBLIC_ORIGIN_VAR);
  if (publicOrigin === undefined) {
    return { ok: false, missing: [PUBLIC_ORIGIN_VAR] };
  }
  return { ok: true, value: publicOrigin };
}
