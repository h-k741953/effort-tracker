// docs/specs/design-system.md AC-6-13
//
// どの項目を出すかを判定しない（ロール・認証状態・勤務月の状態を受け取らない。AC-7-2）。
// 項目は呼び出し側が渡す。footerItems は区切り線の後ろ、最下部に置く。
// 角丸は 4-4-j（項目は control、項目の丸印は全円）、フォーカスリングは 4-5 / 4-5-a、
// 動きは 4-7（motion-safe の下でだけ）。

import Link from "next/link";

export interface SideNavItem {
  label: string;
  href: string;
  current: boolean;
}

export interface SideNavProps {
  items: readonly SideNavItem[];
  footerItems?: readonly SideNavItem[];
}

function NavList({ items }: { items: readonly SideNavItem[] }) {
  return (
    <ul className="flex flex-col gap-1">
      {items.map((item) => (
        <li key={item.href}>
          <Link
            href={item.href}
            aria-current={item.current ? "page" : undefined}
            className={`flex items-center gap-2 rounded-(--et-radius-control) px-3 py-2 text-sm transition-transform duration-(--et-dur) ease-(--et-ease) hover:bg-et-sidebar-active hover:text-et-sidebar-foreground motion-safe:hover:translate-x-(--et-nav-shift) focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background ${
              item.current ? "bg-et-sidebar-active text-et-sidebar-foreground" : "text-et-sidebar-muted"
            }`}
          >
            <span aria-hidden="true" className="size-2 rounded-full border border-current" />
            {item.label}
          </Link>
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
