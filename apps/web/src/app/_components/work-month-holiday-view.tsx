"use client";

// docs/specs/web-screens-implementation-design.md M-11・M-12。本物の祝日の判定を WorkMonthView へ渡すだけの層。

import { isHoliday } from "@/lib/holidays";
import { WorkMonthView, type WorkMonthViewBaseProps } from "./work-month-view";

export function WorkMonthHolidayView(props: WorkMonthViewBaseProps) {
  return <WorkMonthView {...props} isHoliday={isHoliday} />;
}
