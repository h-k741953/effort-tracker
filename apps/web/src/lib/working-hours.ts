// 稼働時間の算出（画面側）。
//
// docs/specs/daily-record-entry-ui.md AC-3-6-e が署名を定める純粋関数。
// ドメインの算出（docs/specs/daily-record-entry.md AC-2-1-a）の写しであり、
// 期待値の表（同 AC-3 の 3-B・3-C）で両側を同じ入力から検査する（UI AC-3-6-d）。
// 画面が示す稼働時間は、この関数の戻り値だけから得る（UI AC-3-6-f）。

export interface TimeOfDay {
  readonly hour: number;
  readonly minute: number;
}

export interface Break {
  readonly start: TimeOfDay;
  readonly end: TimeOfDay;
}

export interface WorkingHours {
  readonly hours: number;
  readonly minutes: number;
}

const MINUTES_PER_HOUR = 60;

// 1日の稼働時間の下限・上限（分。daily-record-entry.md D-7: 0時間15分〜23時間45分）。
const MIN_DAILY_MINUTES = 15;
const MAX_DAILY_MINUTES = 23 * MINUTES_PER_HOUR + 45;

function toMinutes(t: TimeOfDay): number {
  return t.hour * MINUTES_PER_HOUR + t.minute;
}

/**
 * 開始・終了・休憩から稼働時間を算出する。D-16 の前後関係と休憩の位置（端点を含む）、
 * D-7 の下限・上限を満たさない組は null を返す（画面は値を示さない。UI AC-3-6-b）。
 * 例外を投げず、現在時刻・入出力に依らない。
 * 時刻の粒度・値域の外（3-A の「弾く」値）に対する戻り値は固定しない（UI 限界 8-9）。
 */
export function computeWorkingHours(
  startTime: TimeOfDay,
  endTime: TimeOfDay,
  brk: Break | null,
): WorkingHours | null {
  const start = toMinutes(startTime);
  const end = toMinutes(endTime);
  if (end <= start) return null;

  let worked = end - start;
  if (brk !== null) {
    const breakStart = toMinutes(brk.start);
    const breakEnd = toMinutes(brk.end);
    if (breakEnd <= breakStart) return null;
    if (breakStart < start || breakEnd > end) return null;
    worked -= breakEnd - breakStart;
  }

  if (worked < MIN_DAILY_MINUTES || worked > MAX_DAILY_MINUTES) return null;
  return { hours: Math.floor(worked / MINUTES_PER_HOUR), minutes: worked % MINUTES_PER_HOUR };
}
