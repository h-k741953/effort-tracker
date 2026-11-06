// docs/specs/readonly-screens-implementation-design.md H-1。ヘルプの薄い層。

import { helpScreen } from "../_components/help-screen";
import { serverScreenDeps } from "../_lib/screen-deps";

export default async function HelpPage() {
  return helpScreen(serverScreenDeps);
}
