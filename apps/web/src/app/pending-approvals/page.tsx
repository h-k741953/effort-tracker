// docs/specs/web-screens-implementation-design.md W-1・T-0-2。承認待ち一覧の薄い層。

import { pendingApprovalsScreen } from "../_components/pending-approvals-screen";
import { serverScreenDeps } from "../_lib/screen-deps";

export default async function PendingApprovalsPage() {
  return pendingApprovalsScreen(serverScreenDeps);
}
