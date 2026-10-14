import { SideNav } from "web";

export const Engineer = () => (
  <div className="flex h-96 w-60">
    <SideNav
      items={[
        { label: "今月の入力", href: "/work-months/current", current: false },
        { label: "自分の勤務月", href: "/work-months", current: true },
        { label: "契約一覧", href: "/contracts", current: false },
        { label: "稼働の推移", href: "/trends", current: false },
      ]}
      footerItems={[{ label: "ヘルプ", href: "/help", current: false }]}
    />
  </div>
);

export const Approver = () => (
  <div className="flex h-96 w-60">
    <SideNav
      items={[{ label: "承認待ち", href: "/approvals", current: true }]}
      footerItems={[{ label: "ヘルプ", href: "/help", current: false }]}
    />
  </div>
);
