"use client";

// docs/specs/web-screens-implementation-design.md L-4。
// ロールはサーバーが持つ（props）。先回りして書き換えず、成功・401 は router.refresh() のみで描き直す。

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { ErrorBanner } from "@/components/error-banner";
import { RoleSwitcher } from "@/components/role-switcher";
import { requestRoleSwitch } from "@/lib/role-switch-client";
import type { Role } from "@/lib/role-cookie";

const FAILURE_TEXT = "ロールを切り替えられませんでした。";

export function RoleSwitchControl({ role }: { role: Role }) {
  const router = useRouter();
  const [failed, setFailed] = useState(false);
  // 切替は反転なので、通信中の2回目を送ると元へ戻る。state ではなく ref で同期的に弾く。
  const inFlight = useRef(false);

  const handleChange = async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    try {
      const result = await requestRoleSwitch();
      if (result === "failed") {
        setFailed(true);
        return;
      }
      setFailed(false);
      router.refresh();
    } finally {
      inFlight.current = false;
    }
  };

  return (
    <div className="flex flex-col gap-2">
      <RoleSwitcher role={role} onChange={() => void handleChange()} />
      {failed ? <ErrorBanner>{FAILURE_TEXT}</ErrorBanner> : null}
    </div>
  );
}
