// docs/specs/web-screens-implementation-design.md L-1・C-3・C-4。ログイン画面。

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { THEME_COOKIE_NAME, resolveTheme } from "@/lib/theme-cookie";
import { decideLoginScreen } from "@/lib/screen-access";
import { getCurrentViewer } from "./_lib/current-viewer";
import { LoginEntries } from "./_components/login-entries";
import { SiteHeader } from "./_components/site-header";

export default async function LoginPage() {
  const viewer = await getCurrentViewer();
  if (viewer.kind === "misconfigured") {
    // Q-4 (ii) = (a): ゲストや既定の画面へ黙って落とさず、Next のエラーの描画に任せる。
    throw new Error("認証の構成が欠けています。");
  }

  const decision = decideLoginScreen(viewer);
  if (decision.kind === "redirect") {
    redirect(decision.to);
  }

  const theme = resolveTheme((await cookies()).get(THEME_COOKIE_NAME)?.value);

  return (
    <>
      <SiteHeader viewer={viewer} theme={theme} screen="login" />
      <main className="flex flex-col items-center gap-6 px-6 py-12">
        <LoginEntries />
      </main>
    </>
  );
}
