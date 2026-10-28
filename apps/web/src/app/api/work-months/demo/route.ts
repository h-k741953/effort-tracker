// R-4: ゲスト向けデモ勤務月群（契約 E-2）。R-2・R-3 とは別のパスの GET。
// 処理は src/lib/work-month-handlers.ts にあり、ここは薄い配線である
// （入口のレート制限も処理の側で通る）。
import { createLambdaClient } from "@/lib/lambda-client";
import { handleListDemoWorkMonths } from "@/lib/work-month-handlers";

const lambdaClient = createLambdaClient();

export async function GET(request: Request): Promise<Response> {
  return handleListDemoWorkMonths(request, { lambdaClient });
}
