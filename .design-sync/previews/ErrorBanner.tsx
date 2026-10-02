import { ErrorBanner } from "web";

export const WithRetry = () => (
  <ErrorBanner onRetry={() => {}}>勤務月の読み込みに失敗しました。通信状況を確認してから再試行してください。</ErrorBanner>
);

export const WithoutRetry = () => <ErrorBanner>この勤務月を表示する権限がありません。</ErrorBanner>;
