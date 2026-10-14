import type { ReactNode } from "react";
import { ConfirmDialog } from "web";

// ConfirmDialog は画面全体を覆う（fixed inset-0）。カードの中で文言とボタンがすべて見えるよう、
// 高さを持つ枠で包み、transform-gpu で fixed の基準をその枠にする。
const Frame = ({ children }: { children: ReactNode }) => (
  <div className="relative h-72 w-full transform-gpu overflow-hidden">{children}</div>
);

export const Close = () => (
  <Frame>
    <ConfirmDialog
      open
      title="2026年9月の勤務月を締めますか？"
      description={<p className="text-sm">締めると、承認者の承認が済むまで稼働実績を編集できません。</p>}
      confirmLabel="締める"
      onConfirm={() => {}}
      onCancel={() => {}}
    />
  </Frame>
);

export const Danger = () => (
  <Frame>
    <ConfirmDialog
      open
      title="この勤務月を差し戻しますか？"
      description={<p className="text-sm">差し戻すと、技術者が稼働実績を修正できる状態に戻ります。</p>}
      confirmLabel="差し戻す"
      confirmVariant="danger"
      onConfirm={() => {}}
      onCancel={() => {}}
    />
  </Frame>
);
