// docs/specs/design-system.md AC-6-1
//
// 状態バッジ。公開 props は `state` の1つのみ（表の前文・AC-7-2）。
// 操作者の種別を受け取らないことで、誰が見ても同一の表示になる。

export type StatusBadgeState = "Draft" | "PendingApproval" | "Approved";

export interface StatusBadgeProps {
  state: StatusBadgeState;
}

const LABELS: Record<StatusBadgeState, string> = {
  Draft: "下書き",
  PendingApproval: "締め済",
  Approved: "承認済",
};

const TOKEN_CLASSES: Record<StatusBadgeState, string> = {
  Draft: "bg-state-draft text-state-draft-foreground",
  PendingApproval: "bg-state-pending-approval text-state-pending-approval-foreground",
  Approved: "bg-state-approved text-state-approved-foreground",
};

export function StatusBadge({ state }: StatusBadgeProps) {
  return (
    <span className={`inline-flex items-center rounded-md px-2 py-1 text-sm ${TOKEN_CLASSES[state]}`}>
      {LABELS[state]}
    </span>
  );
}
