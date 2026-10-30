"use client";

// docs/specs/web-screens-implementation-design.md W-4・W-8。本人一覧の0件の EmptyState の action。
// 画面固有の部品であり、design-system.md の台帳に載せない。当月は持たない（行き先は W-7 の経路）。

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/button";
import type { ThisMonthEntry } from "@/lib/this-month-entry";

const LABEL = "稼働実績を入力する";

export function InitialEntryAction({ entry }: { entry: ThisMonthEntry }) {
  const [open, setOpen] = useState(false);

  if (entry.kind === "hidden") return null;
  if (entry.kind === "link") {
    return (
      <Link
        href={entry.href}
        className="text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
      >
        {LABEL}
      </Link>
    );
  }
  return (
    <div className="flex flex-col items-center gap-2">
      <Button aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        {LABEL}
      </Button>
      {open ? (
        <ul className="flex flex-col gap-1">
          {entry.options.map((option) => (
            <li key={option.href}>
              <Link
                href={option.href}
                className="text-primary underline focus-visible:ring-2 focus-visible:ring-focus-ring"
              >
                {option.label}
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
