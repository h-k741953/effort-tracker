// docs/specs/web-screens-implementation-design.md W-1。値は props で受ける。ログイン画面からは使わない。

import type { ReactNode } from "react";
import type { SideNavCurrent } from "@/lib/side-nav-items";
import type { Theme } from "@/lib/theme-cookie";
import type { Viewer } from "@/lib/viewer";
import { AppSideNav } from "./app-side-nav";
import { SiteHeader } from "./site-header";

export function AppShell({
  viewer,
  theme,
  current,
  children,
}: {
  viewer: Viewer;
  theme: Theme;
  current: SideNavCurrent;
  children: ReactNode;
}) {
  return (
    <>
      <SiteHeader viewer={viewer} theme={theme} screen="app" />
      <div className="flex">
        <AppSideNav viewer={viewer} current={current} />
        <main className="flex flex-1 flex-col gap-4 p-6">{children}</main>
      </div>
    </>
  );
}
