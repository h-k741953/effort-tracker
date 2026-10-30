"use client";

// docs/specs/web-screens-implementation-design.md W-6・W-8。
// 技術者のときだけ自分の契約を取り、「今月の入力」の形を決める。取得中・失敗・0件は hidden。

import { useEffect, useState } from "react";
import { SideNav } from "@/components/side-nav";
import type { ContractSummary } from "@/lib/work-month-list";
import { fetchOwnContracts } from "@/lib/work-month-list-client";
import { sideNavItems, workMonthSideNavCurrent, type SideNavCurrent, type WorkMonthLocation } from "@/lib/side-nav-items";
import { thisMonthEntry, type ThisMonthEntry } from "@/lib/this-month-entry";
import type { Viewer } from "@/lib/viewer";

const HIDDEN: ThisMonthEntry = { kind: "hidden" };

export function AppSideNav({ viewer, current }: { viewer: Viewer; current: SideNavCurrent | WorkMonthLocation }) {
  const [thisMonth, setThisMonth] = useState<ThisMonthEntry>(HIDDEN);
  // 契約の列と、それを取った時に effect の中で読んだ時刻を一緒に持つ（描画中に時刻を読まない）。
  const [own, setOwn] = useState<{ contracts: ContractSummary[]; now: Date } | undefined>(undefined);
  const isEngineer = viewer.kind === "signed-in" && viewer.role === "Engineer";

  useEffect(() => {
    if (!isEngineer) return;
    let cancelled = false;
    const now = new Date();
    void fetchOwnContracts(now).then((result) => {
      if (cancelled) return;
      setThisMonth(result.kind === "ok" ? thisMonthEntry(result.value) : HIDDEN);
      setOwn(result.kind === "ok" ? { contracts: result.value, now } : undefined);
    });
    return () => {
      cancelled = true;
    };
  }, [isEngineer]);

  const resolvedCurrent: SideNavCurrent =
    typeof current === "string"
      ? current
      : workMonthSideNavCurrent({
          viewer,
          contractId: current.contractId,
          yearMonth: current.yearMonth,
          ownContracts: isEngineer ? own?.contracts : undefined,
          now: isEngineer ? own?.now : undefined,
        });
  const { items, footerItems } = sideNavItems({
    viewer,
    current: resolvedCurrent,
    thisMonth: isEngineer ? thisMonth : HIDDEN,
  });
  return <SideNav items={items} footerItems={footerItems} />;
}
