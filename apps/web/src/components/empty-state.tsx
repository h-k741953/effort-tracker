// docs/specs/design-system.md AC-6-8

import type { ReactNode } from "react";

export interface EmptyStateProps {
  title: string;
  description?: string;
  action?: ReactNode;
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-2 rounded-md border border-border p-6 text-center">
      <p className="text-base font-medium text-foreground">{title}</p>
      {description !== undefined ? (
        <p className="text-sm text-muted-foreground">{description}</p>
      ) : null}
      {action !== undefined ? action : null}
    </div>
  );
}
