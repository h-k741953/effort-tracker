// docs/specs/design-system.md AC-6-14 / AC-12
//
// 型 Theme は theme-cookie.ts の型を輸入する（リテラル union を再定義しない）。
// 保存・<html> の data-theme の付け替え・OS の設定の読み取りをしない。部品は theme を
// 表示し、選ばれた値を onChange へ渡すだけである（保存と属性はアプリが持つ。AC-12-3）。
//
// 同一文書に複数置いても各インスタンスが独立したグループになるよう、ネイティブ radio の
// name はインスタンスごとに useId() で一意化する（RoleSwitcher と同じ形。6-3-iii）。
// 角丸は 4-4-g、フォーカスリングは 4-5 / 4-5-a。

import { useId, type ChangeEvent } from "react";
import type { Theme } from "@/lib/theme-cookie";

export interface ThemeToggleProps {
  theme: Theme;
  onChange: (theme: Theme) => void;
}

const OPTIONS: ReadonlyArray<{ value: Theme; label: string }> = [
  { value: "light", label: "ライト" },
  { value: "dark", label: "ダーク" },
];

function Icon({ theme }: { theme: Theme }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="mr-1 size-4"
    >
      {theme === "light" ? (
        <>
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
        </>
      ) : (
        <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
      )}
    </svg>
  );
}

export function ThemeToggle({ theme, onChange }: ThemeToggleProps) {
  const name = useId();

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange(event.target.value as Theme);
  };

  return (
    <div
      role="radiogroup"
      aria-label="テーマ切替"
      className="inline-flex items-center gap-1 rounded-(--et-radius-control) border border-border bg-et-hover p-1"
    >
      {OPTIONS.map((option) => (
        <label
          key={option.value}
          className="inline-flex cursor-pointer items-center rounded-(--et-radius-control) px-3 py-1 text-sm text-muted-foreground has-checked:bg-surface has-checked:text-foreground has-checked:shadow-(--et-shadow) has-focus-visible:ring-2 has-focus-visible:ring-offset-2 has-focus-visible:ring-focus-ring has-focus-visible:ring-offset-background"
        >
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={theme === option.value}
            onChange={handleChange}
            className="sr-only focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background"
          />
          <Icon theme={option.value} />
          {option.label}
        </label>
      ))}
    </div>
  );
}
