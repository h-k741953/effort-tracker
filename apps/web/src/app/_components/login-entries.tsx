// docs/specs/web-screens-implementation-design.md L-2。login-ui.md AC-1-1〜4-3。
// 入口は3つ。フォーム・入力欄は置かない。(b)(c) は外部へ 302 する経路のため <a>（全体の遷移）。

import Link from "next/link";
import { DEMO_WORK_MONTHS_PATH } from "@/lib/screen-routes";

const SIGN_IN_PATH = "/api/auth/sign-in";

const ENTRY_CLASS =
  "inline-flex w-full items-center justify-center rounded-(--et-radius-control) border border-border bg-surface px-4 py-2 text-sm font-medium text-foreground focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-focus-ring focus-visible:ring-offset-background";

export function LoginEntries() {
  return (
    <div className="flex w-full max-w-sm flex-col gap-4">
      <div className="flex flex-col gap-1">
        <Link href={DEMO_WORK_MONTHS_PATH} className={ENTRY_CLASS}>
          ゲストとして続行（閲覧のみ）
        </Link>
        <p className="text-xs text-muted-foreground">入力・締め・承認はできません。</p>
      </div>
      {/* 外部（Cognito）へ 302 する経路のため next/link を使わず、<a> で全体の遷移にする（L-2） */}
      <a href={SIGN_IN_PATH} className={ENTRY_CLASS}>
        メールアドレスでログイン
      </a>
      {/* 同上 */}
      <a href={SIGN_IN_PATH} className={ENTRY_CLASS}>
        Google でログイン
      </a>
    </div>
  );
}
