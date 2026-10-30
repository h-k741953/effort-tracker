// docs/specs/web-screens-implementation-design.md W-6・W-8。純粋な関数。
// 項目の集合・文言・並びは work-month-listing-ui.md 10-2〜10-4・10-6。承認担当者一覧の項目は作らない（10-4-a）。

import type { SideNavItem } from "@/components/side-nav";
import {
  CONTRACTS_PATH,
  DEMO_WORK_MONTHS_PATH,
  HELP_PATH,
  OWN_WORK_MONTHS_PATH,
  PENDING_APPROVALS_PATH,
  WORK_TRENDS_PATH,
} from "./screen-routes";
import type { ThisMonthEntry } from "./this-month-entry";
import type { Viewer } from "./viewer";

export type SideNavCurrent =
  | "this-month"
  | "own-work-months"
  | "contracts"
  | "work-trends"
  | "pending-approvals"
  | "demo-work-months"
  | "help";

function thisMonthItem(thisMonth: ThisMonthEntry, current: boolean): SideNavItem | undefined {
  const label = "今月の入力";
  if (thisMonth.kind === "link") return { label, href: thisMonth.href, current };
  if (thisMonth.kind === "choice") return { label, options: thisMonth.options, current };
  return undefined;
}

export function sideNavItems(params: {
  viewer: Viewer;
  current: SideNavCurrent;
  thisMonth: ThisMonthEntry;
}): { items: SideNavItem[]; footerItems: SideNavItem[] } {
  const { viewer, current, thisMonth } = params;
  const footerItems: SideNavItem[] = [{ label: "ヘルプ", href: HELP_PATH, current: current === "help" }];

  if (viewer.kind === "guest") {
    return {
      items: [{ label: "サンプル勤務月", href: DEMO_WORK_MONTHS_PATH, current: current === "demo-work-months" }],
      footerItems,
    };
  }
  if (viewer.role === "Approver") {
    return {
      items: [{ label: "承認待ち", href: PENDING_APPROVALS_PATH, current: current === "pending-approvals" }],
      footerItems,
    };
  }
  const entry = thisMonthItem(thisMonth, current === "this-month");
  return {
    items: [
      ...(entry ? [entry] : []),
      { label: "自分の勤務月", href: OWN_WORK_MONTHS_PATH, current: current === "own-work-months" },
      { label: "契約一覧", href: CONTRACTS_PATH, current: current === "contracts" },
      { label: "稼働の推移", href: WORK_TRENDS_PATH, current: current === "work-trends" },
    ],
    footerItems,
  };
}
