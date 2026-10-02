import { Button, EmptyState } from "web";

export const WithAction = () => (
  <EmptyState
    title="勤務月がまだありません"
    description="稼働実績を入力すると、ここに勤務月が表示されます。"
    action={<Button variant="primary">稼働実績を入力する</Button>}
  />
);

export const TitleOnly = () => <EmptyState title="承認待ちの勤務月はありません" />;
