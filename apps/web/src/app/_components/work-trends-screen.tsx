// docs/specs/readonly-screens-implementation-design.md SP-1・T-0-2。稼働の推移の本体。

import { redirect } from "next/navigation";
import { decideScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { AppShell } from "./app-shell";
import { WorkTrendsView } from "./work-trends-view";

export async function workTrendsScreen(deps: ScreenDeps) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    throw new Error("認証の構成が欠けています。");
  }
  const decision = decideScreen("work-trends", viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }
  const theme = await deps.getTheme();
  return (
    <AppShell viewer={viewer} theme={theme} current="work-trends">
      <WorkTrendsView />
    </AppShell>
  );
}
