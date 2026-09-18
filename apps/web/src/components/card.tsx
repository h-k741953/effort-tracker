// docs/specs/design-system.md AC-6-4

import type { ReactNode } from "react";

export interface CardProps {
  children: ReactNode;
  title?: ReactNode;
}

export function Card({ children, title }: CardProps) {
  return (
    <div className="rounded-md border border-border bg-surface p-4 text-surface-foreground">
      {title !== undefined ? <h2 className="mb-2 text-base font-semibold">{title}</h2> : null}
      {children}
    </div>
  );
}
