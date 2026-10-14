// docs/specs/design-system.md AC-6-2
//
// `disabled` は多重送信の抑止にのみ使う（AC-7-2）。値・文言は呼び出し側が
// 渡すため、本コンポーネントは業務ルールを内蔵しない（AC-7-1）。
// 角丸は 4-4-d、フォーカスリングは 4-5 / 4-5-a、動きは 4-7（motion-safe の下でだけ）。

import type { ButtonHTMLAttributes } from "react";

export type ButtonVariant = "primary" | "secondary" | "danger";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: "bg-primary bg-linear-to-r from-primary to-et-grad-to text-primary-foreground",
  secondary: "bg-surface text-surface-foreground border border-border hover:bg-et-hover",
  danger: "bg-danger text-danger-foreground",
};

export function Button({ variant = "secondary", type = "button", className, ...rest }: ButtonProps) {
  const classes = [
    "inline-flex items-center justify-center rounded-(--et-radius-control) px-3 py-2 text-sm font-medium",
    "transition-transform duration-(--et-dur) ease-(--et-ease)",
    "motion-safe:hover:translate-y-(--et-btn-lift) motion-safe:active:scale-(--et-press)",
    "focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background",
    VARIANT_CLASSES[variant],
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return <button type={type} className={classes} {...rest} />;
}
