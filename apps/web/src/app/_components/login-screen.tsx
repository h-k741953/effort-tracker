// docs/specs/web-screens-implementation-design.md L-1・C-3・C-4・T-0-2。ログイン画面の本体。

import { redirect } from "next/navigation";
import { decideLoginScreen } from "@/lib/screen-access";
import type { ScreenDeps } from "../_lib/screen-deps";
import { LoginEntries } from "./login-entries";
import { SiteHeader } from "./site-header";

export async function loginScreen(deps: ScreenDeps) {
  const viewer = await deps.getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    // Q-4 (ii) = (a): ゲストや既定の画面へ黙って落とさず、Next のエラーの描画に任せる。
    throw new Error("認証の構成が欠けています。");
  }

  const decision = decideLoginScreen(viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }

  const theme = await deps.getTheme();

  return (
    <>
      <SiteHeader viewer={viewer} theme={theme} screen="login" />
      <main className="flex flex-col items-center gap-6 px-6 py-12">
        <LoginEntries />
      </main>
    </>
  );
}
