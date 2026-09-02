// (i) サインインの開始（docs/specs/bff-auth-termination.md AC-11-2 (i)）。
//
// AC-11-11: 処理は src/lib/ 側にあり、ここは**入口でレート制限を通してから
// 呼ぶだけの薄い配線**である（AC-11-12「例外の経路を作らない」）。
// AC-11-10: ウィンドウの判定に用いる現在時刻はここから与える。
import { handleSignInStart } from "@/lib/auth-handlers";
import { withRequestRateLimit } from "@/lib/rate-limit";

export async function GET(request: Request): Promise<Response> {
  return withRequestRateLimit(request.headers, new Date(), () =>
    handleSignInStart(request),
  );
}
