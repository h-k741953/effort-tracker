// docs/specs/design-system.md AC-6-2
//
// `disabled` は多重送信の抑止にのみ使う（AC-7-2）。値・文言は呼び出し側が
// 渡すため、本コンポーネントは業務ルールを内蔵しない（AC-7-1）。

import type { ButtonHTMLAttributes } from "react";

export type ButtonVariant = "primary" | "secondary" | "danger";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: "bg-primary text-primary-foreground",
  secondary: "bg-surface text-surface-foreground border border-border",
  danger: "bg-danger text-danger-foreground",
};

export function Button({ variant = "secondary", type = "button", className, ...rest }: ButtonProps) {
  const classes = [
    "inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium",
    "focus-visible:ring-2 focus-visible:ring-focus-ring",
    VARIANT_CLASSES[variant],
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return <button type={type} className={classes} {...rest} />;
}
