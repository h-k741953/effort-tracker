import { useState } from "react";
import { FieldError, NumberField } from "web";

export const Default = () => {
  const [value, setValue] = useState<number | "">(7.5);
  return (
    <NumberField id="hours-default" label="稼働時間（時間）" value={value} onChange={setValue} min={0} max={24} step={0.25} />
  );
};

export const Empty = () => {
  const [value, setValue] = useState<number | "">("");
  return <NumberField id="hours-empty" label="稼働時間（時間）" value={value} onChange={setValue} min={0} max={24} step={0.25} />;
};

export const Invalid = () => {
  const [value, setValue] = useState<number | "">(25);
  return (
    <div className="flex flex-col gap-1">
      <NumberField
        id="hours-invalid"
        label="稼働時間（時間）"
        value={value}
        onChange={setValue}
        min={0}
        max={24}
        step={0.25}
        invalid
        errorId="hours-invalid-error"
      />
      <FieldError id="hours-invalid-error">0〜24 の範囲で入力してください。</FieldError>
    </div>
  );
};
