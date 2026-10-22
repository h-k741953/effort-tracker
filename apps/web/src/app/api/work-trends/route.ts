// R-11: 稼働の推移（契約 E-9）。AC-6-7 (ii): R-2・R-3・R-10 とは別のパスの GET。
// docs/specs/bff-wiring.md AC-10-2: 処理は src/lib/work-month-handlers.ts にあり、ここは
// 薄い配線である（入口のレート制限も処理の側で通る＝AC-5-1）。
import { createLambdaClient } from "@/lib/lambda-client";
import { handleListWorkTrends } from "@/lib/work-month-handlers";

const lambdaClient = createLambdaClient();

export async function GET(request: Request): Promise<Response> {
  return handleListWorkTrends(request, { lambdaClient });
}
