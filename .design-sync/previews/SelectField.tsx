import { useState } from "react";
import { FieldError, SelectField } from "web";

// 選択肢は数値だけを示し、単位（時・分）は選択欄の右横に置く。ラベルには単位を付けない。
const HOURS = Array.from({ length: 24 }, (_, h) => ({ value: String(h), label: String(h) }));
const MINUTES = [0, 15, 30, 45].map((m) => ({ value: String(m), label: String(m).padStart(2, "0") }));

// 選択欄と同じ高さ（境界線1px＋上下の余白＋1行）の箱に入れ、単位を選択欄の上下中央に揃える。
const Unit = ({ children }: { children: string }) => (
  <span className="inline-flex items-center border border-transparent py-1 text-base leading-normal">
    <span className="text-sm text-foreground">{children}</span>
  </span>
);

export const StartTime = () => {
  const [hour, setHour] = useState("9");
  const [minute, setMinute] = useState("0");
  return (
    <div className="flex items-end gap-2">
      <SelectField id="start-hour" label="開始" value={hour} options={HOURS} onChange={setHour} placeholder="--" />
      <Unit>時</Unit>
      <SelectField id="start-minute" label="開始の分" hideLabel value={minute} options={MINUTES} onChange={setMinute} placeholder="--" />
      <Unit>分</Unit>
    </div>
  );
};

export const Unselected = () => {
  const [hour, setHour] = useState("");
  const [minute, setMinute] = useState("");
  return (
    <div className="flex items-end gap-2">
      <SelectField id="end-hour" label="終了" value={hour} options={HOURS} onChange={setHour} placeholder="--" />
      <Unit>時</Unit>
      <SelectField id="end-minute" label="終了の分" hideLabel value={minute} options={MINUTES} onChange={setMinute} placeholder="--" />
      <Unit>分</Unit>
    </div>
  );
};

export const Invalid = () => {
  const [hour, setHour] = useState("8");
  const [minute, setMinute] = useState("0");
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-end gap-2">
        <SelectField id="end-hour-invalid" label="終了" value={hour} options={HOURS} onChange={setHour} placeholder="--" invalid errorId="end-time-error" />
        <Unit>時</Unit>
        <SelectField id="end-minute-invalid" label="終了の分" hideLabel value={minute} options={MINUTES} onChange={setMinute} placeholder="--" invalid errorId="end-time-error" />
        <Unit>分</Unit>
      </div>
      <FieldError id="end-time-error">終了は開始より後の時刻にしてください。</FieldError>
    </div>
  );
};
