import { Card, StatusBadge } from "web";

export const WithTitle = () => (
  <Card title="2026年9月の勤務月">
    <p className="text-sm">稼働時間の合計は 152.5 時間です。精算幅 140〜180 時間の範囲内です。</p>
  </Card>
);

export const WithoutTitle = () => (
  <Card>
    <p className="text-sm">締めると、承認者の承認が済むまで稼働実績を編集できません。</p>
  </Card>
);

export const WithStatus = () => (
  <Card title="2026年8月の勤務月">
    <div className="flex items-center justify-between gap-4 text-sm">
      <span>合計 168.0 時間</span>
      <StatusBadge state="Approved" />
    </div>
  </Card>
);
