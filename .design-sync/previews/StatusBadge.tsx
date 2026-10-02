import { StatusBadge } from "web";

export const AllStates = () => (
  <div className="flex items-center gap-2">
    <StatusBadge state="Draft" />
    <StatusBadge state="PendingApproval" />
    <StatusBadge state="Approved" />
  </div>
);

export const InListRow = () => (
  <div className="flex items-center justify-between gap-4 border-b border-border py-2 text-sm text-foreground">
    <span>2026年9月</span>
    <StatusBadge state="PendingApproval" />
  </div>
);
