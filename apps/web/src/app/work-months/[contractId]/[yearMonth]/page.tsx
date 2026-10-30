// docs/specs/web-screens-implementation-design.md M-1・M-12・T-0-2。勤務月画面の薄い層。

import { WorkMonthHolidayView } from "../../../_components/work-month-holiday-view";
import { workMonthScreen } from "../../../_components/work-month-screen";
import { serverScreenDeps } from "../../../_lib/screen-deps";

export default async function WorkMonthPage(props: {
  params: Promise<{ contractId: string; yearMonth: string }>;
}) {
  return workMonthScreen(serverScreenDeps, await props.params, WorkMonthHolidayView);
}
