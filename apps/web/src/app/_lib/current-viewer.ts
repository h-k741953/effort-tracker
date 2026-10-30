// docs/specs/web-screens-implementation-design.md C-3。
// headers() の cookie と現在時刻で resolveViewer を呼ぶだけの薄い層。
// React の cache で1要求1回にまとめる（ヘッダーとページが別々に解決しない）。

import { cache } from "react";
import { headers } from "next/headers";
import { resolveViewer } from "@/lib/viewer";

export const getCurrentViewer = cache(async () => {
  const requestHeaders = await headers();
  return resolveViewer({ cookieHeader: requestHeaders.get("cookie"), now: new Date() });
});
