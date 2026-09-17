// docs/specs/design-system.md AC-6-9 / 6-9-iii
//
// 取り消せない旨の文言は内蔵しない。description として呼び出し側が渡す
// （AC-7-1）。Esc キーと取消操作は onCancel のみを呼び、onConfirm を呼ばない
// （monthly-closing-ui.md AC-2-3 の UI 側の担保）。open が真になったとき、
// ダイアログ自身へ初期フォーカスを当てる（Esc の受け口をフォーカスの外に
// 置かない。初期描画・再描画の両方で満たす）。

import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from "react";
import { Button } from "./button";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description: ReactNode;
  confirmLabel: string;
  confirmVariant?: "primary" | "danger";
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  confirmVariant = "primary",
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const titleId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open) {
      dialogRef.current?.focus();
    }
  }, [open]);

  if (!open) {
    return null;
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      onCancel();
    }
  };

  return (
    <div
      ref={dialogRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      tabIndex={-1}
      onKeyDown={handleKeyDown}
      className="rounded-md border border-border bg-surface p-4 text-surface-foreground outline-none focus-visible:ring-2 focus-visible:ring-focus-ring"
    >
      <h2 id={titleId} className="text-base font-semibold">
        {title}
      </h2>
      <div>{description}</div>
      <div className="mt-4 flex justify-end gap-2">
        <Button variant="secondary" onClick={onCancel}>
          取消
        </Button>
        <Button variant={confirmVariant} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </div>
  );
}
