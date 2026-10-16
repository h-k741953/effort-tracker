// R-7: 締め（契約 E-5）。
// docs/specs/bff-wiring.md AC-10-2: 処理は src/lib/work-month-handlers.ts にあり、ここは
// 薄い配線である（入口のレート制限も処理の側で通る＝AC-5-1）。
import { createLambdaClient } from "@/lib/lambda-client";
import { handleCloseWorkMonth } from "@/lib/work-month-handlers";

const lambdaClient = createLambdaClient();

export async function POST(
  request: Request,
  context: { params: Promise<{ contractId: string; yearMonth: string }> },
): Promise<Response> {
  return handleCloseWorkMonth(request, await context.params, { lambdaClient });
}
