// docs/specs/web-screens-implementation-design.md C-4。login-ui.md AC-6-7〜6-10-a。
// 純粋な関数。Cookie・状態・稼働実績に触れない（6-9-b）。画面の出し分けであり認可ではない（8-7）。

import {
  DEMO_WORK_MONTHS_PATH,
  OWN_WORK_MONTHS_PATH,
  PENDING_APPROVALS_PATH,
} from "./screen-routes";
import type { Viewer } from "./viewer";

export type ScreenId =
  | "own-work-months"
  | "pending-approvals"
  | "demo-work-months"
  | "work-month"
  | "help"
  | "contracts"
  | "work-trends";

export type ScreenDecision = { kind: "show" } | { kind: "redirect"; to: string };

/** 6-7-a: 区分ごとの既定の画面。 */
export function defaultScreenPath(viewer: Viewer): string {
  if (viewer.kind === "guest") return DEMO_WORK_MONTHS_PATH;
  return viewer.role === "Engineer" ? OWN_WORK_MONTHS_PATH : PENDING_APPROVALS_PATH;
}

/** 6-7: 区分ごとに開ける画面。 */
function openableScreens(viewer: Viewer): readonly ScreenId[] {
  if (viewer.kind === "guest") return ["demo-work-months", "help", "work-month"];
  return viewer.role === "Engineer"
    ? ["own-work-months", "contracts", "work-trends", "help", "work-month"]
    : ["pending-approvals", "help", "work-month"];
}

export function decideScreen(screen: ScreenId, viewer: Viewer): ScreenDecision {
  return openableScreens(viewer).includes(screen)
    ? { kind: "show" }
    : { kind: "redirect", to: defaultScreenPath(viewer) };
}

/** 6-10・6-10-a: ログイン済みは既定の画面へ移し、ゲストは移さない。 */
export function decideLoginScreen(viewer: Viewer): ScreenDecision {
  return viewer.kind === "guest"
    ? { kind: "show" }
    : { kind: "redirect", to: defaultScreenPath(viewer) };
}
