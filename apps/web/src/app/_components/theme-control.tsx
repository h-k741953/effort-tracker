"use client";

// docs/specs/web-screens-implementation-design.md L-5。design-system.md AC-12-3。
// Cookie の保存と <html data-theme> の付け替えのみ。サーバーへ要求を送らない。

import { useState } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { serializeThemeCookie, type Theme } from "@/lib/theme-cookie";

export function ThemeControl({ initialTheme }: { initialTheme: Theme }) {
  const [theme, setTheme] = useState<Theme>(initialTheme);

  const handleChange = (next: Theme) => {
    document.cookie = serializeThemeCookie(next);
    document.documentElement.setAttribute("data-theme", next);
    setTheme(next);
  };

  return <ThemeToggle theme={theme} onChange={handleChange} />;
}
