// docs/specs/design-system.md AC-6-4

import type { ReactNode } from "react";

export interface CardProps {
  children: ReactNode;
  title?: ReactNode;
}

export function Card({ children, title }: CardProps) {
  return (
    <div className="rounded-(--et-radius-card) border border-border bg-surface p-4 text-surface-foreground shadow-(--et-shadow)">
      {title !== undefined ? <h2 className="mb-2 text-base font-semibold">{title}</h2> : null}
      {children}
    </div>
  );
}
