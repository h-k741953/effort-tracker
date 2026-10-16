// R-5: 稼働実績の入力・編集（契約 E-3）／R-6: 稼働実績の削除（契約 E-4）。
// docs/specs/bff-wiring.md AC-10-2: 処理は src/lib/work-month-handlers.ts にあり、ここは
// 薄い配線である（入口のレート制限も処理の側で通る＝AC-5-1）。
import { createLambdaClient } from "@/lib/lambda-client";
import { handleDeleteDailyRecord, handlePutDailyRecord } from "@/lib/work-month-handlers";

const lambdaClient = createLambdaClient();

type Context = {
  params: Promise<{ contractId: string; yearMonth: string; date: string }>;
};

export async function PUT(request: Request, context: Context): Promise<Response> {
  return handlePutDailyRecord(request, await context.params, { lambdaClient });
}

export async function DELETE(request: Request, context: Context): Promise<Response> {
  return handleDeleteDailyRecord(request, await context.params, { lambdaClient });
}
