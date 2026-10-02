import { ConfirmDialog } from "web";

export const Close = () => (
  <ConfirmDialog
    open
    title="2026年9月の勤務月を締めますか？"
    description={<p className="text-sm">締めると、承認者の承認が済むまで稼働実績を編集できません。</p>}
    confirmLabel="締める"
    onConfirm={() => {}}
    onCancel={() => {}}
  />
);

export const Danger = () => (
  <ConfirmDialog
    open
    title="この勤務月を差し戻しますか？"
    description={<p className="text-sm">差し戻すと、技術者が稼働実績を修正できる状態に戻ります。</p>}
    confirmLabel="差し戻す"
    confirmVariant="danger"
    onConfirm={() => {}}
    onCancel={() => {}}
  />
);
