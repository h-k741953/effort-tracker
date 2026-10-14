// docs/specs/design-system.md AC-6-12
//
// options の外の選択肢を内蔵しない（時 0〜23・分 0 / 15 / 30 / 45 は呼び出し側が渡す。
// AC-7-1）。値を数値へ変換しない（onChange へは選んだ選択肢の value の文字列をそのまま渡す）。
// hideLabel が真のときも <label for> は DOM に残し、アクセシブル名として読ませる。
// 角丸は 4-4-e、フォーカスリングは 4-5 / 4-5-a。

import type { ChangeEvent } from "react";

export interface SelectFieldProps {
  id: string;
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
  placeholder?: string;
  hideLabel?: boolean;
  errorId?: string;
  invalid?: boolean;
}

export function SelectField({
  id,
  label,
  value,
  options,
  onChange,
  placeholder,
  hideLabel,
  errorId,
  invalid,
}: SelectFieldProps) {
  const handleChange = (event: ChangeEvent<HTMLSelectElement>) => {
    onChange(event.target.value);
  };

  const borderClass = invalid ? "border-danger" : "border-et-input-border";

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className={hideLabel ? "sr-only" : "text-sm font-medium text-foreground"}>
        {label}
      </label>
      <select
        id={id}
        value={value}
        onChange={handleChange}
        aria-invalid={invalid ? "true" : undefined}
        aria-describedby={errorId}
        className={`rounded-(--et-radius-control) border ${borderClass} bg-surface px-2 py-1 text-foreground focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background`}
      >
        {placeholder !== undefined ? <option value="">{placeholder}</option> : null}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}
