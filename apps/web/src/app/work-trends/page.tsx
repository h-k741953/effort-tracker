// docs/specs/readonly-screens-implementation-design.md SP-1。稼働の推移の薄い層。

import { workTrendsScreen } from "../_components/work-trends-screen";
import { serverScreenDeps } from "../_lib/screen-deps";

export default async function WorkTrendsPage() {
  return workTrendsScreen(serverScreenDeps);
}
