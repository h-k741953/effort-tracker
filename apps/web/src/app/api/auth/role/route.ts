// (iii) デモ用ロール切替（docs/specs/bff-auth-termination.md AC-11-2 (iii)・
// AC-5-5）。署名済みロール Cookie を発行する唯一の経路であり、**本経路も入口で
// レート制限を通る**（AC-11-12）。
//
// AC-11-11: 処理は src/lib/ 側にあり、ここは薄い配線である。
import { handleRoleSwitch } from "@/lib/auth-handlers";
import { withRequestRateLimit } from "@/lib/rate-limit";

export async function POST(request: Request): Promise<Response> {
  const now = new Date();
  return withRequestRateLimit(request.headers, now, () =>
    handleRoleSwitch(request, { now }),
  );
}
