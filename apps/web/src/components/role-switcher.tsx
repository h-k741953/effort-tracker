// docs/specs/design-system.md AC-6-3
//
// 型 Role は role-cookie.ts の既存の型を輸入する（リテラル union を再定義
// しない）。選択肢はちょうど2つ（Engineer→技術者／Approver→承認者。P-4）。
// ゲストを表す値を持たない。
//
// DOM 構造の選択: ネイティブの `role="radiogroup"` + `<input type="radio">`
// を採る。ラジオグループはブラウザ標準で「グループにアクセシブル名を持たせる
// （aria-label）」「現在の選択が checked 属性で判別できる」の両方を
// 構造だけで満たし、カスタム ARIA の管理（キーボード操作の再実装等）が要ら
// ない。select 要素も候補だったが、2択の排他選択は radiogroup がより素直に
// 表現でき、将来 A/B の表示切り替え（アイコン等）を追加する余地も広い。
//
// 6-3-iii: 同一文書に複数配置しても各インスタンスが独立したグループとして
// 振る舞うこと。ネイティブ radio の name はインスタンスごとに useId() で
// 一意化する（独立性はコンポーネントの内側で得る。props は増やさない）。

import { useId, type ChangeEvent } from "react";
import type { Role } from "@/lib/role-cookie";

export interface RoleSwitcherProps {
  role: Role;
  onChange: (role: Role) => void;
}

const OPTIONS: ReadonlyArray<{ value: Role; label: string }> = [
  { value: "Engineer", label: "技術者" },
  { value: "Approver", label: "承認者" },
];

export function RoleSwitcher({ role, onChange }: RoleSwitcherProps) {
  const name = useId();

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange(event.target.value as Role);
  };

  return (
    <div role="radiogroup" aria-label="ロール切替" className="flex items-center gap-3">
      {OPTIONS.map((option) => (
        <label key={option.value} className="flex items-center gap-1 text-sm text-foreground">
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={role === option.value}
            onChange={handleChange}
            className="focus-visible:ring-2 focus-visible:ring-focus-ring"
          />
          {option.label}
        </label>
      ))}
    </div>
  );
}
