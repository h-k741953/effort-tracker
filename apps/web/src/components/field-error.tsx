// docs/specs/design-system.md AC-6-6
//
// 文言は内蔵しない。呼び出し側が children として理由の文言を渡す（AC-7-1）。

import type { ReactNode } from "react";

export interface FieldErrorProps {
  id: string;
  children: ReactNode;
}

export function FieldError({ id, children }: FieldErrorProps) {
  return (
    <p id={id} role="alert" className="text-sm text-danger">
      {children}
    </p>
  );
}
