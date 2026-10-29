// docs/specs/web-screens-implementation-design.md L-3。右側に出すものと並びを headerSlots で決める。

import Link from "next/link";
import type { Theme } from "@/lib/theme-cookie";
import { LOGIN_PATH } from "@/lib/screen-routes";
import type { Viewer } from "@/lib/viewer";
import { RoleSwitchControl } from "./role-switch-control";
import { ThemeControl } from "./theme-control";

export type HeaderSlot = "role" | "theme" | "login-link";
export type HeaderScreen = "login" | "app";

export function headerSlots(viewer: Viewer, screen: HeaderScreen): HeaderSlot[] {
  if (screen === "login") return ["theme"];
  if (viewer.kind === "guest") return ["theme", "login-link"];
  return ["role", "theme"];
}

export function HeaderControls({
  viewer,
  theme,
  screen,
}: {
  viewer: Viewer;
  theme: Theme;
  screen: HeaderScreen;
}) {
  return (
    <div className="flex items-center gap-3">
      {headerSlots(viewer, screen).map((slot) => {
        switch (slot) {
          case "role":
            return viewer.kind === "signed-in" ? (
              <RoleSwitchControl key={slot} role={viewer.role} />
            ) : null;
          case "theme":
            return <ThemeControl key={slot} initialTheme={theme} />;
          case "login-link":
            return (
              <Link
                key={slot}
                href={LOGIN_PATH}
                className="text-sm font-medium text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
              >
                ログイン
              </Link>
            );
        }
      })}
    </div>
  );
}
