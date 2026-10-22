// R-10: 契約一覧（契約 E-8）。AC-6-7 (ii): R-2・R-3・R-11 とは別のパスの GET。
// docs/specs/bff-wiring.md AC-10-2: 処理は src/lib/work-month-handlers.ts にあり、ここは
// 薄い配線である（入口のレート制限も処理の側で通る＝AC-5-1）。
import { createLambdaClient } from "@/lib/lambda-client";
import { handleListContracts } from "@/lib/work-month-handlers";

const lambdaClient = createLambdaClient();

export async function GET(request: Request): Promise<Response> {
  return handleListContracts(request, { lambdaClient });
}
