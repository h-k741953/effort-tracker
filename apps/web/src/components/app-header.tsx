// docs/specs/design-system.md AC-6-11
//
// RoleSwitcher を内蔵しない。right に差し込ませる（AC-7-2 / login-ui.md AC-6-5）。
// アプリ名の前の装飾の印（時計の図案）は aria-hidden とし、テキストを持たない
// （見出しのテキストを effort-tracker のままにする。6-11-i）。

import type { ReactNode } from "react";

export interface AppHeaderProps {
  right?: ReactNode;
}

export function AppHeader({ right }: AppHeaderProps) {
  return (
    <header className="flex items-center justify-between border-b border-border bg-background px-4 py-3 text-foreground">
      <div className="flex items-center gap-2">
        <span className="inline-flex size-8 items-center justify-center rounded-(--et-radius-control) bg-linear-to-br from-primary to-et-grad-to text-primary-foreground">
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="size-5"
          >
            <circle cx="12" cy="12" r="9" />
            <path d="M12 7v5l3 2" />
          </svg>
        </span>
        <h1 className="text-lg font-semibold">effort-tracker</h1>
      </div>
      {right !== undefined ? <div>{right}</div> : null}
    </header>
  );
}
