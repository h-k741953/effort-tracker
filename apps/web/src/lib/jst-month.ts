// docs/specs/web-screens-implementation-design.md W-7。
// 与えた時刻の JST（UTC+9。夏時間なし）の年月 YYYY-MM を返す。時刻は引数で受ける。

const JST_OFFSET_MS = 9 * 60 * 60 * 1000;

export function currentJstYearMonth(now: Date): string {
  const jst = new Date(now.getTime() + JST_OFFSET_MS);
  const year = String(jst.getUTCFullYear()).padStart(4, "0");
  const month = String(jst.getUTCMonth() + 1).padStart(2, "0");
  return `${year}-${month}`;
}

// docs/specs/web-screens-implementation-design.md M-11。与えた時刻の JST の当日 YYYY-MM-DD を返す。
export function currentJstDate(now: Date): string {
  const jst = new Date(now.getTime() + JST_OFFSET_MS);
  const day = String(jst.getUTCDate()).padStart(2, "0");
  return `${currentJstYearMonth(now)}-${day}`;
}
