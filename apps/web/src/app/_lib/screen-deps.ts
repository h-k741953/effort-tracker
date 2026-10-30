// docs/specs/web-screens-implementation-design.md T-0-2。
// 画面の本体へ注入する依存の型と、本物の依存。サーバー専用（next/headers を輸入する）。
// 本体（_components/*-screen.tsx）はこのファイルを輸入しない。

import { cookies } from "next/headers";
import { THEME_COOKIE_NAME, resolveTheme, type Theme } from "@/lib/theme-cookie";
import type { ResolveViewerResult } from "@/lib/viewer";
import { getCurrentViewer } from "./current-viewer";

export type ScreenDeps = {
  getCurrentViewer: () => Promise<ResolveViewerResult>;
  getTheme: () => Promise<Theme>;
};

export const serverScreenDeps: ScreenDeps = {
  getCurrentViewer,
  getTheme: async () => resolveTheme((await cookies()).get(THEME_COOKIE_NAME)?.value),
};

// docs/specs/web-screens-implementation-design.md W-7。今月の入力の経路の本体へ注入する依存。
export type ThisMonthDeps = {
  getCurrentViewer: () => Promise<ResolveViewerResult>;
  now: () => Date;
};

export const serverThisMonthDeps: ThisMonthDeps = {
  getCurrentViewer,
  now: () => new Date(),
};
