// docs/specs/design-system.md AC-6-11
//
// RoleSwitcher を内蔵しない。right に差し込ませる（AC-7-2 / login-ui.md AC-6-5）。

import type { ReactNode } from "react";

export interface AppHeaderProps {
  right?: ReactNode;
}

export function AppHeader({ right }: AppHeaderProps) {
  return (
    <header className="flex items-center justify-between border-b border-border bg-background px-4 py-3 text-foreground">
      <h1 className="text-lg font-semibold">effort-tracker</h1>
      {right !== undefined ? <div>{right}</div> : null}
    </header>
  );
}
