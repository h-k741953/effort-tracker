// デモ用ロール切替の署名済み Cookie（docs/specs/bff-auth-termination.md
// AC-5-5・Q-B = (a)・Q-F = (b)）。
//
// - AC-5-5 (i): ロールの値は **BFF が署名した Cookie を BFF 自身が検証して得た
//   値だけ**から決める。署名が検証できない・欠けている Cookie を採用しない。
// - AC-5-5 (iii): 値は Engineer / Approver のいずれかに限る（AC-5-3）。
// - AC-5-5 (viii) / AC-7-5: 署名鍵は BFF の環境変数から受け取る。**未設定・空の
//   ときは既定値へ黙って落ちず、推測もせず、失敗として扱う。**
// - AC-5-5 (viii) / AC-11-13: 署名と検証のために npm 依存を追加しない
//   （Node 標準の crypto だけを使う）。
// - AC-4-1: 鍵をログ・エラー文言・応答・URL へ出さない。結果は理由を持たない。
import { createHmac, timingSafeEqual } from "node:crypto";

/** AC-5-3: ロールは 2 つだけ。Guest というロール値を作らない。 */
export type Role = "Engineer" | "Approver";

const ALLOWED_ROLES: readonly Role[] = ["Engineer", "Approver"];

/**
 * Cookie の値の形式は `${role}.${base64url(HMAC-SHA256(signingKey, role))}`。
 * 区切りは 1 つだけで、ロール側には現れない文字を使う。
 */
const SEPARATOR = ".";

/** ロール Cookie の名前。5-8 と同じ属性で送出する（AC-5-5 (vi)）。 */
export const ROLE_COOKIE_NAME = "et_role";

export interface SignedRoleCookie {
  readonly ok: true;
  /** Set-Cookie に載せる値。 */
  readonly value: string;
}

export interface RoleCookieRejected {
  readonly ok: false;
}

export type SignRoleCookieResult = SignedRoleCookie | RoleCookieRejected;

export interface VerifiedRoleCookie {
  readonly ok: true;
  readonly role: Role;
}

export type VerifyRoleCookieResult = VerifiedRoleCookie | RoleCookieRejected;

function isAllowedRole(role: string): role is Role {
  return (ALLOWED_ROLES as readonly string[]).includes(role);
}

function signature(role: string, signingKey: string): string {
  return createHmac("sha256", signingKey).update(role).digest("base64url");
}

/**
 * ロールに署名する。**鍵が未設定（空）なら成立させない**（AC-5-5 (viii)）。
 * **許可外のロール値にも署名しない**（AC-5-5 (iii)）。
 */
export function signRoleCookie(params: {
  role: Role;
  signingKey: string;
}): SignRoleCookieResult {
  const { role, signingKey } = params;

  // AC-7-5: 鍵が無い状態を既定値で埋めない。無署名の Cookie を発行しない。
  if (typeof signingKey !== "string" || signingKey.length === 0) {
    return { ok: false };
  }
  // 型は Role でも、実行時には別の値が渡りうる（境界の入力）。
  if (typeof role !== "string" || !isAllowedRole(role)) {
    return { ok: false };
  }

  return { ok: true, value: `${role}${SEPARATOR}${signature(role, signingKey)}` };
}

/**
 * Cookie の値を検証してロールを得る。署名が検証できない・欠けている・別の鍵で
 * 署名された・許可外のロールである場合はいずれも成立させない（AC-5-5 (i)(iii)・
 * AC-8-5 の拒否側）。**鍵が未設定（空）のときも成立させない**（AC-5-5 (viii)）。
 */
export function verifyRoleCookie(params: {
  cookieValue: string;
  signingKey: string;
}): VerifyRoleCookieResult {
  const { cookieValue, signingKey } = params;

  if (typeof signingKey !== "string" || signingKey.length === 0) {
    return { ok: false }; // AC-5-5 (viii)
  }
  if (typeof cookieValue !== "string" || cookieValue.length === 0) {
    return { ok: false };
  }

  const separatorIndex = cookieValue.indexOf(SEPARATOR);
  if (separatorIndex <= 0 || separatorIndex !== cookieValue.lastIndexOf(SEPARATOR)) {
    return { ok: false }; // 署名の欠けた Cookie（ロールのみ）を含む
  }

  const role = cookieValue.slice(0, separatorIndex);
  const presented = cookieValue.slice(separatorIndex + SEPARATOR.length);

  if (!isAllowedRole(role)) {
    return { ok: false }; // 署名が正当でも許可外の値は採用しない（AC-5-5 (iii)）
  }

  if (!signaturesMatch(presented, signature(role, signingKey))) {
    return { ok: false };
  }

  return { ok: true, role };
}

function signaturesMatch(presented: string, expected: string): boolean {
  const presentedBytes = Buffer.from(presented, "utf8");
  const expectedBytes = Buffer.from(expected, "utf8");
  if (presentedBytes.length !== expectedBytes.length) {
    return false;
  }
  return timingSafeEqual(presentedBytes, expectedBytes);
}
