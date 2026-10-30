// docs/specs/web-screens-implementation-design.md W-7・T-0-2。今月の入力の経路の薄い層。
// 先読みされない動的なページに保つ（押された時点の JST の当月で送り先を決めるため）。

import { thisMonthScreen, type ThisMonthSearchParams } from "../_components/this-month-screen";
import { serverThisMonthDeps } from "../_lib/screen-deps";

export const dynamic = "force-dynamic";

export default async function ThisMonthPage(props: { searchParams: Promise<ThisMonthSearchParams> }) {
  return thisMonthScreen(serverThisMonthDeps, await props.searchParams);
}
