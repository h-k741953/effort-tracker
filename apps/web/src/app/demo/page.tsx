// docs/specs/web-screens-implementation-design.md W-1・T-0-2。デモ勤務月群の薄い層。

import { demoWorkMonthsScreen } from "../_components/demo-work-months-screen";
import { serverScreenDeps } from "../_lib/screen-deps";

export default async function DemoWorkMonthsPage() {
  return demoWorkMonthsScreen(serverScreenDeps);
}
