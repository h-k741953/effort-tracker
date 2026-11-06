// docs/specs/readonly-screens-implementation-design.md H-1。ヘルプの本体。

import { redirect } from "next/navigation";
import { decideScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { AppShell } from "./app-shell";
import { HelpView } from "./help-view";

export async function helpScreen(deps: ScreenDeps) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  const decision = decideScreen("help", viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }
  const theme = await deps.getTheme();
  return (
    <AppShell viewer={viewer} theme={theme} current="help">
      <HelpView />
    </AppShell>
  );
}
