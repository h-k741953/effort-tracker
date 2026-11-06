// docs/specs/readonly-screens-implementation-design.md H-2。文言は docs/specs/help-ui.md AC-3 の字面。

import { Card } from "@/components/card";
import { StatusBadge, type StatusBadgeState } from "@/components/status-badge";

const FLOW_BADGED: ReadonlyArray<{ state: StatusBadgeState; text: string }> = [
  { state: "Draft", text: "技術者が日ごとの稼働実績を入力・編集・削除します。入力が済んだら締めます。" },
  {
    state: "PendingApproval",
    text: "締めると承認待ちになります。承認者が、承認するか差し戻すかを選びます。この間、技術者は稼働実績を編集できません。",
  },
  { state: "Approved", text: "承認されると承認済になります。承認済の勤務月は、以降だれも状態を変えられません。" },
];

const FLOW_PLAIN: readonly string[] = [
  "差し戻されると下書きに戻り、技術者が稼働実績を修正できるようになります。",
  "締めた勤務月を、技術者が自分で下書きに戻す操作はありません。下書きに戻るのは、承認者が差し戻したときだけです。",
  "自分が技術者である勤務月は、承認者であっても承認・差戻しできません。",
];

const ENTRY: readonly string[] = [
  "開始時刻と終了時刻、休憩の有無を入力します。休憩ありのときは、休憩の開始と終了の時刻も入力します。稼働時間はこれらの時刻から算出され、直接は入力しません。",
  "時は 0〜23、分は 00・15・30・45 から選びます。終了時刻は開始時刻より後で、23:45 までです。日をまたぐ入力はできません。",
  "休憩は1日に1回までです。休憩は開始時刻から終了時刻までの間に収めます。",
  "稼働時間が 0 になる入力はできません。稼働の無い日は入力しません。",
  "1日に入力できる稼働実績は1件です。同じ日に入力し直すと、前の入力を置き換えます。",
  "今日より後の日には入力できません。今日の判断は日本時間で行います。",
  "下書きの間は、入力済みの日の稼働実績を削除できます。",
  "月の総稼働時間は、日ごとの稼働時間を15分単位で切り捨ててから合計した値です。",
];

const RANGE: readonly string[] = [
  "精算幅は、契約が定める月の稼働時間の下限と上限です。",
  "総稼働時間が上限を上回った分を超過、下限を下回った分を不足といいます。",
  "超過と不足は、締めたときの総稼働時間から算出されます。下書きの間は表示されません。",
  "勤務月の精算幅は、その勤務月が作られたときの契約の値です。後から契約が変わっても、作られた勤務月の精算幅は変わりません。",
];

export function HelpView() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-semibold">ヘルプ</h1>
      <Card title="勤務月の流れ">
        {FLOW_BADGED.map(({ state, text }) => (
          <div key={state} className="mb-2 flex items-start gap-2">
            <StatusBadge state={state} />
            <p>{text}</p>
          </div>
        ))}
        {FLOW_PLAIN.map((text) => (
          <p key={text} className="mb-2">
            {text}
          </p>
        ))}
      </Card>
      <Card title="稼働実績の入力">
        {ENTRY.map((text) => (
          <p key={text} className="mb-2">
            {text}
          </p>
        ))}
      </Card>
      <Card title="精算幅と超過／不足">
        {RANGE.map((text) => (
          <p key={text} className="mb-2">
            {text}
          </p>
        ))}
      </Card>
    </div>
  );
}
