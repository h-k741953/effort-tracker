// docs/specs/design-system.md AC-6-7 / 6-7-i
//
// 色以外にも文字で「エラーである」ことが分かる（role="alert" + 表示文字）。
// 再試行のラベル文言は内蔵せず固定の日本語のみ使う（トリガーの意味を示す
// だけで業務理由文は持たない。理由文は children として呼び出し側が渡す）。
// 固定文字「エラー」は children に依存せず常に出し、children の理由文とは
// 別の要素に置く（10-6-d の完全一致での問い合わせに応じるため）。

import type { ReactNode } from "react";
import { Button } from "./button";

export interface ErrorBannerProps {
  children: ReactNode;
  onRetry?: () => void;
}

export function ErrorBanner({ children, onRetry }: ErrorBannerProps) {
  return (
    <div role="alert" className="rounded-md border border-danger bg-background p-3 text-danger">
      <p className="font-semibold">エラー</p>
      <p>{children}</p>
      {onRetry !== undefined ? (
        <Button variant="danger" onClick={onRetry}>
          再試行
        </Button>
      ) : null}
    </div>
  );
}
