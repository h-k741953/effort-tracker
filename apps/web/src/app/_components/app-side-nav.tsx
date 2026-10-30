"use client";

// docs/specs/web-screens-implementation-design.md W-6・W-8。
// 技術者のときだけ自分の契約を取り、「今月の入力」の形を決める。取得中・失敗・0件は hidden。

import { useEffect, useState } from "react";
import { SideNav } from "@/components/side-nav";
import { fetchOwnContracts } from "@/lib/work-month-list-client";
import { sideNavItems, type SideNavCurrent } from "@/lib/side-nav-items";
import { thisMonthEntry, type ThisMonthEntry } from "@/lib/this-month-entry";
import type { Viewer } from "@/lib/viewer";

const HIDDEN: ThisMonthEntry = { kind: "hidden" };

export function AppSideNav({ viewer, current }: { viewer: Viewer; current: SideNavCurrent }) {
  const [thisMonth, setThisMonth] = useState<ThisMonthEntry>(HIDDEN);
  const isEngineer = viewer.kind === "signed-in" && viewer.role === "Engineer";

  useEffect(() => {
    if (!isEngineer) return;
    let cancelled = false;
    void fetchOwnContracts(new Date()).then((result) => {
      if (cancelled) return;
      setThisMonth(result.kind === "ok" ? thisMonthEntry(result.value) : HIDDEN);
    });
    return () => {
      cancelled = true;
    };
  }, [isEngineer]);

  const { items, footerItems } = sideNavItems({
    viewer,
    current,
    thisMonth: isEngineer ? thisMonth : HIDDEN,
  });
  return <SideNav items={items} footerItems={footerItems} />;
}
