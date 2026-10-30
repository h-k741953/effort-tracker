// docs/specs/web-screens-implementation-design.md W-1・T-0-2。本人の勤務月一覧の薄い層。

import { ownWorkMonthsScreen } from "../_components/own-work-months-screen";
import { serverScreenDeps } from "../_lib/screen-deps";

export default async function OwnWorkMonthsPage(props: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  return ownWorkMonthsScreen(serverScreenDeps, await props.searchParams);
}
