// docs/specs/web-screens-implementation-design.md W-7。今月の入力の経路の本体。何も描かず redirect() するだけ。
// ドメイン API・lambda-client・BFF を呼ばない。

import { redirect } from "next/navigation";
import { currentJstYearMonth } from "@/lib/jst-month";
import { defaultScreenPath } from "@/lib/screen-access";
import { ownWorkMonthsPath, workMonthPath } from "@/lib/screen-routes";
import type { ThisMonthDeps } from "../_lib/screen-deps";

export type ThisMonthSearchParams = { [key: string]: string | string[] | undefined };

export async function thisMonthScreen(
  deps: ThisMonthDeps,
  searchParams: ThisMonthSearchParams,
): Promise<never> {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  if (viewer.kind === "guest" || viewer.role !== "Engineer") {
    return redirect(defaultScreenPath(viewer));
  }
  const contract = searchParams.contract;
  if (typeof contract !== "string" || contract === "") {
    return redirect(ownWorkMonthsPath({}));
  }
  return redirect(workMonthPath(contract, currentJstYearMonth(deps.now())));
}
