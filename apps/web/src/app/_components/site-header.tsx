// docs/specs/web-screens-implementation-design.md L-1。値は props で受ける（テストで描画できるように）。

import { AppHeader } from "@/components/app-header";
import type { Theme } from "@/lib/theme-cookie";
import type { Viewer } from "@/lib/viewer";
import { HeaderControls, type HeaderScreen } from "./header-controls";

export function SiteHeader({
  viewer,
  theme,
  screen,
}: {
  viewer: Viewer;
  theme: Theme;
  screen: HeaderScreen;
}) {
  return <AppHeader right={<HeaderControls viewer={viewer} theme={theme} screen={screen} />} />;
}
