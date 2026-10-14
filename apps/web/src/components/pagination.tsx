// docs/specs/design-system.md AC-6-10
//
// 1ページあたりの件数を props に持たず、既定値も持たない（P-7）。
// 先頭・末尾での非活性は範囲外の入力の抑止であり、AC-7-2 の対象外。

export interface PaginationProps {
  page: number;
  pageCount: number;
  onPageChange: (page: number) => void;
}

const BUTTON_CLASSES =
  "rounded-(--et-radius-control) border border-border bg-surface px-2 py-1 text-surface-foreground hover:bg-et-hover disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background";

export function Pagination({ page, pageCount, onPageChange }: PaginationProps) {
  return (
    <nav aria-label="ページ送り">
      <button
        type="button"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
        className={BUTTON_CLASSES}
      >
        前へ
      </button>
      <button
        type="button"
        disabled={page >= pageCount}
        onClick={() => onPageChange(page + 1)}
        className={BUTTON_CLASSES}
      >
        次へ
      </button>
    </nav>
  );
}
