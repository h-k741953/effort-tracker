// (ii) サインインの戻り（docs/specs/bff-auth-termination.md AC-11-2 (ii)）。
// 構成側の戻り先（cognito_callback_urls）が指すのはこの経路である。
// **その一致は機械検査されない**（限界 10-10）。パスの実体は
// src/lib/cognito-oidc.ts の CALLBACK_PATH と揃える。
//
// AC-11-11: 処理は src/lib/ 側にあり、ここは薄い配線である。
import { handleSignInCallback } from "@/lib/auth-handlers";
import { withRequestRateLimit } from "@/lib/rate-limit";

export async function GET(request: Request): Promise<Response> {
  const now = new Date();
  return withRequestRateLimit(request.headers, now, () =>
    handleSignInCallback(request, { now }),
  );
}
