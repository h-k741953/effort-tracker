// docs/specs/design-system.md AC-6-5
//
// min / max / step を既定値として内蔵しない（値域は業務側が持つ。AC-7-1）。

import type { ChangeEvent } from "react";

export interface NumberFieldProps {
  id: string;
  label: string;
  value: number | "";
  onChange: (value: number | "") => void;
  min?: number;
  max?: number;
  step?: number;
  errorId?: string;
  invalid?: boolean;
}

export function NumberField({
  id,
  label,
  value,
  onChange,
  min,
  max,
  step,
  errorId,
  invalid,
}: NumberFieldProps) {
  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const raw = event.target.value;
    onChange(raw === "" ? "" : Number(raw));
  };

  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-foreground">
        {label}
      </label>
      <input
        id={id}
        type="number"
        value={value}
        onChange={handleChange}
        min={min}
        max={max}
        step={step}
        aria-invalid={invalid ? "true" : undefined}
        aria-describedby={errorId}
        className="rounded-md border border-border bg-background px-2 py-1 text-foreground focus-visible:ring-2 focus-visible:ring-focus-ring"
      />
    </div>
  );
}
