// docs/specs/readonly-screens-implementation-design.md KP-1・T-0-2。契約一覧の本体。

import { redirect } from "next/navigation";
import { decideScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { AppShell } from "./app-shell";
import { ContractsView } from "./contracts-view";

export async function contractsScreen(deps: ScreenDeps) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  const decision = decideScreen("contracts", viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }
  const theme = await deps.getTheme();
  return (
    <AppShell viewer={viewer} theme={theme} current="contracts">
      <ContractsView />
    </AppShell>
  );
}
