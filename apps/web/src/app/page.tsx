// docs/specs/web-screens-implementation-design.md L-1・T-0-2。ログイン画面の薄い層。

import { loginScreen } from "./_components/login-screen";
import { serverScreenDeps } from "./_lib/screen-deps";

export default async function LoginPage() {
  return loginScreen(serverScreenDeps);
}
