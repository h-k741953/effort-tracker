// docs/specs/readonly-screens-implementation-design.md KP-1。契約一覧の薄い層。

import { contractsScreen } from "../_components/contracts-screen";
import { serverScreenDeps } from "../_lib/screen-deps";

export default async function ContractsPage() {
  return contractsScreen(serverScreenDeps);
}
