// docs/specs/design-system.md AC-6-13
//
// どの項目を出すかを判定しない（ロール・認証状態・勤務月の状態を受け取らない。AC-7-2）。
// 項目は呼び出し側が渡す。footerItems は区切り線の後ろ、最下部に置く。
// 角丸は 4-4-j（項目は control、項目の丸印は全円）、丸印の塗りは 6-13-iv、フォーカスリングは 4-5 / 4-5-a、
// 動きは 4-7（motion-safe の下でだけ）。

"use client";

import Link from "next/link";
import { useState } from "react";

export interface SideNavLinkItem {
  label: string;
  href: string;
  current: boolean;
}

/** 選択肢を開く項目（6-13-v）。href を持たない。 */
export interface SideNavOptionsItem {
  label: string;
  options: readonly { label: string; href: string }[];
  current: boolean;
}

export type SideNavItem = SideNavLinkItem | SideNavOptionsItem;

export interface SideNavProps {
  items: readonly SideNavItem[];
  footerItems?: readonly SideNavItem[];
}

const ITEM_CLASS =
  "flex items-center gap-2 rounded-(--et-radius-control) px-3 py-2 text-sm transition-transform duration-(--et-dur) ease-(--et-ease) hover:bg-et-sidebar-active hover:text-et-sidebar-foreground motion-safe:hover:translate-x-(--et-nav-shift) focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background";

function itemClass(current: boolean): string {
  return `${ITEM_CLASS} ${current ? "bg-et-sidebar-active text-et-sidebar-foreground" : "text-et-sidebar-muted"}`;
}

function Marker({ current }: { current: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={`size-2 rounded-full ${current ? "bg-primary" : "bg-et-sidebar-muted"}`}
    />
  );
}

function OptionsItem({ item }: { item: SideNavOptionsItem }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        aria-expanded={open}
        aria-current={item.current ? "page" : undefined}
        onClick={() => setOpen((v) => !v)}
        className={`w-full text-left ${itemClass(item.current)}`}
      >
        <Marker current={item.current} />
        {item.label}
      </button>
      {open ? (
        <ul className="mt-1 flex flex-col gap-1 pl-5">
          {item.options.map((option) => (
            <li key={option.href}>
              <Link href={option.href} className={itemClass(false)}>
                {option.label}
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </>
  );
}

function NavList({ items }: { items: readonly SideNavItem[] }) {
  return (
    <ul className="flex flex-col gap-1">
      {items.map((item) => (
        <li key={item.label}>
          {"options" in item ? (
            <OptionsItem item={item} />
          ) : (
            <Link
              href={item.href}
              aria-current={item.current ? "page" : undefined}
              className={itemClass(item.current)}
            >
              <Marker current={item.current} />
              {item.label}
            </Link>
          )}
        </li>
      ))}
    </ul>
  );
}

export function SideNav({ items, footerItems = [] }: SideNavProps) {
  return (
    <nav aria-label="メインメニュー" className="flex flex-col gap-2 bg-et-sidebar p-3 text-et-sidebar-muted">
      <NavList items={items} />
      {footerItems.length > 0 ? (
        <>
          <hr className="mt-auto border-et-sidebar-active" />
          <NavList items={footerItems} />
        </>
      ) : null}
    </nav>
  );
}
