import { describe, expect, it } from "vitest";
import {
  averageLabel,
  axisRange,
  defaultPeriod,
  endOptions,
  monthAxisLabels,
  monthKind,
  periodOptions,
  rowsInPeriod,
  rowsOfContract,
  settlementRangeSummary,
  withStart,
  withinCount,
  type WorkTrendRow,
} from "./work-trends";

// docs/specs/work-trends-ui.md 1-2・1-5・2-1・2-2・2-4・3-1〜3-3・3-6・3-6-a・4-1〜4-4-b・5-3・5-6・5-7。
// 設計 readonly-screens-implementation-design.md SP-4・SP-5・SP-6・S-1。
// 平均の期待値は 4-1-d の組から書き、実装の関数で期待値を作らない。

type Hm = { hours: number; minutes: number };
const hm = (hours: number, minutes = 0): Hm => ({ hours, minutes });
const RANGE = { lowerBound: hm(140), upperBound: hm(180) };

/** 既定は「精算幅 140〜180 に収まった承認済の月」。必要な項目だけ上書きする。 */
function tr(yearMonth: string, o: Partial<WorkTrendRow> = {}): WorkTrendRow {
  return {
    contractId: "c-1",
    yearMonth,
    state: "Approved",
    settlementRange: RANGE,
    totalHours: hm(160),
    excess: hm(0),
    shortfall: hm(0),
    ...o,
  };
}
const draft = (yearMonth: string, o: Partial<WorkTrendRow> = {}): WorkTrendRow =>
  tr(yearMonth, { state: "Draft", excess: null, shortfall: null, ...o });

const months = (rows: readonly WorkTrendRow[]) => rows.map((r) => r.yearMonth);

describe("rowsOfContract（1-2・1-5）", () => {
  const rows = [
    tr("2026-09", { contractId: "c-2" }),
    tr("2026-07", { contractId: "c-1" }),
    tr("2026-08", { contractId: "c-2" }),
    tr("2026-05", { contractId: "c-1" }),
    tr("2026-06", { contractId: "c-3" }),
  ];

  it("選んだ契約の行だけを、入力の順のまま返す（他の契約を含めず、並べ替えない）", () => {
    expect(rowsOfContract(rows, "c-1")).toEqual([rows[1], rows[3]]);
    expect(rowsOfContract(rows, "c-2")).toEqual([rows[0], rows[2]]);
  });

  it("行が無い契約・知らない契約・空文字は空の配列", () => {
    expect(rowsOfContract(rows, "c-9")).toEqual([]);
    expect(rowsOfContract(rows, "")).toEqual([]);
    expect(rowsOfContract([], "c-1")).toEqual([]);
  });

  it("入力を書き換えず、新しい配列を返す", () => {
    const input = rows.slice();
    const out = rowsOfContract(input, "c-1");
    expect(input).toEqual(rows);
    expect(out).not.toBe(input);
    const all = rowsOfContract([rows[1], rows[3]], "c-1");
    expect(all).toEqual([rows[1], rows[3]]);
  });
});

describe("defaultPeriod（3-1）", () => {
  it("最も古い月から最も新しい月まで（入力の順に依らない。年をまたぐ）", () => {
    expect(defaultPeriod([tr("2026-02"), tr("2025-11"), tr("2026-01"), tr("2025-12")])).toEqual({
      start: "2025-11",
      end: "2026-02",
    });
  });

  it("1件なら開始と終了が同じ月", () => {
    expect(defaultPeriod([tr("2026-07")])).toEqual({ start: "2026-07", end: "2026-07" });
  });

  it("0件は undefined", () => {
    expect(defaultPeriod([])).toBeUndefined();
  });
});

describe("periodOptions（3-2。SP-7 (d)）", () => {
  it("重複なし・昇順（年をまたぐ）で、value は YYYY-MM、label は「2026年7月」の形", () => {
    const options = periodOptions([
      tr("2026-01"),
      tr("2025-12"),
      tr("2026-01", { contractId: "c-2" }),
      tr("2025-09"),
      tr("2025-12", { state: "Draft", excess: null, shortfall: null }),
    ]);
    expect(options).toEqual([
      { value: "2025-09", label: "2025年9月" },
      { value: "2025-12", label: "2025年12月" },
      { value: "2026-01", label: "2026年1月" },
    ]);
  });

  it("0件は空の配列", () => {
    expect(periodOptions([])).toEqual([]);
  });
});

describe("endOptions（3-3）", () => {
  const options = [
    { value: "2026-05", label: "2026年5月" },
    { value: "2026-06", label: "2026年6月" },
    { value: "2026-07", label: "2026年7月" },
  ];

  it.each([
    ["先頭", "2026-05", ["2026-05", "2026-06", "2026-07"]],
    ["中ほど", "2026-06", ["2026-06", "2026-07"]],
    ["末尾", "2026-07", ["2026-07"]],
  ])("開始が%s → 開始と同じかそれより後だけ（開始より前を含めない）", (_name, start, expected) => {
    expect(endOptions(options, start).map((o) => o.value)).toEqual(expected);
  });

  it("選択肢の label を保ち、入力を書き換えない", () => {
    const input = options.slice();
    expect(endOptions(input, "2026-06")[0]).toEqual({ value: "2026-06", label: "2026年6月" });
    expect(input).toEqual(options);
  });
});

describe("withStart（3-3）", () => {
  it("開始を終了より後へ変えたとき、終了を開始にそろえる", () => {
    expect(withStart({ start: "2026-05", end: "2026-07" }, "2026-09")).toEqual({
      start: "2026-09",
      end: "2026-09",
    });
  });

  it.each([
    ["終了より前", "2026-06"],
    ["終了と同じ", "2026-07"],
  ])("開始を%sへ変えたとき、終了をそろえない", (_name, start) => {
    expect(withStart({ start: "2026-05", end: "2026-07" }, start)).toEqual({ start, end: "2026-07" });
  });

  it("年をまたぐ比較（2026-01 は 2025-12 より後）", () => {
    expect(withStart({ start: "2025-10", end: "2025-12" }, "2026-01")).toEqual({
      start: "2026-01",
      end: "2026-01",
    });
  });

  it("入力の期間を書き換えない", () => {
    const period = { start: "2026-05", end: "2026-07" };
    withStart(period, "2026-09");
    expect(period).toEqual({ start: "2026-05", end: "2026-07" });
  });
});

describe("rowsInPeriod（3-6・3-6-a）", () => {
  const rows = [
    tr("2026-05"),
    tr("2026-08"),
    tr("2026-06"),
    tr("2026-07"),
    tr("2026-04"),
    tr("2026-09"),
  ];

  it("開始・終了ちょうどの月を含み、外の月を含めない。入力の順のまま返す", () => {
    expect(months(rowsInPeriod(rows, { start: "2026-05", end: "2026-07" }))).toEqual([
      "2026-05",
      "2026-06",
      "2026-07",
    ]);
  });

  it("1か月の期間は、その月の行だけ", () => {
    expect(months(rowsInPeriod(rows, { start: "2026-08", end: "2026-08" }))).toEqual(["2026-08"]);
  });

  it("期間に行が無ければ空の配列。入力を書き換えない", () => {
    const input = rows.slice();
    expect(rowsInPeriod(input, { start: "2027-01", end: "2027-03" })).toEqual([]);
    expect(input).toEqual(rows);
  });

  it("年をまたぐ期間", () => {
    const r = [tr("2025-11"), tr("2025-12"), tr("2026-01"), tr("2026-02"), tr("2026-03")];
    expect(months(rowsInPeriod(r, { start: "2025-12", end: "2026-02" }))).toEqual([
      "2025-12",
      "2026-01",
      "2026-02",
    ]);
  });
});

describe("monthKind（2-1・2-2・4-3-a・5-3）", () => {
  it.each<[string, WorkTrendRow, string]>([
    ["Draft", draft("2026-07"), "in-progress"],
    ["Draft（総稼働時間が精算幅の外でも）", draft("2026-07", { totalHours: hm(200) }), "in-progress"],
    ["PendingApproval で超過が 0 でない", tr("2026-07", { state: "PendingApproval", excess: hm(0, 15) }), "excess"],
    ["Approved で超過が 0 でない", tr("2026-07", { excess: hm(5, 15) }), "excess"],
    ["超過が hours だけ 0 でない", tr("2026-07", { excess: hm(1, 0) }), "excess"],
    ["超過が minutes だけ 0 でない", tr("2026-07", { excess: hm(0, 1) }), "excess"],
    ["PendingApproval で不足が 0 でない", tr("2026-07", { state: "PendingApproval", shortfall: hm(0, 30) }), "shortfall"],
    ["Approved で不足が 0 でない", tr("2026-07", { shortfall: hm(3, 0) }), "shortfall"],
    ["不足が minutes だけ 0 でない", tr("2026-07", { shortfall: hm(0, 1) }), "shortfall"],
    ["Approved で両方 0", tr("2026-07"), "within"],
    ["PendingApproval で両方 0", tr("2026-07", { state: "PendingApproval" }), "within"],
  ])("%s → %s", (_name, row, expected) => {
    expect(monthKind(row)).toBe(expected);
  });

  it("totalHours が精算幅の外でも、excess・shortfall が 0 なら収まった（算出し直さない。4-3-a・9-3）", () => {
    expect(monthKind(tr("2026-07", { totalHours: hm(200) }))).toBe("within");
    expect(monthKind(tr("2026-07", { totalHours: hm(100) }))).toBe("within");
  });

  it("totalHours が精算幅に収まって見えても、保存された excess が 0 でなければ超過（算出し直さない）", () => {
    expect(monthKind(tr("2026-07", { totalHours: hm(160), excess: hm(0, 15) }))).toBe("excess");
    expect(monthKind(tr("2026-07", { totalHours: hm(160), shortfall: hm(0, 15) }))).toBe("shortfall");
  });
});

describe("averageLabel（4-1・4-1-a・4-1-d・4-1-e・4-2）", () => {
  const closed = (state: "PendingApproval" | "Approved", total: Hm, ym = "2026-07"): WorkTrendRow =>
    tr(ym, { state, totalHours: total });

  it.each<[string, Hm[], string]>([
    ["(a) 160時間00分・161時間00分", [hm(160, 0), hm(161, 0)], "160.5"],
    ["(b) 160時間30分・160時間35分（小数第2位が 4 → 切り捨て）", [hm(160, 30), hm(160, 35)], "160.5"],
    ["(c) 160時間30分・160時間36分（160.55 → 切り上げ。浮動小数の誤差で 160.5 としない）", [hm(160, 30), hm(160, 36)], "160.6"],
    ["(d) 160時間00分・160時間00分・160時間20分", [hm(160, 0), hm(160, 0), hm(160, 20)], "160.1"],
    ["(e) 確定した月が1件で 162時間30分", [hm(162, 30)], "162.5"],
    ["(f) 159時間55分・160時間00分（繰り上がって小数第1位が 0。160 としない）", [hm(159, 55), hm(160, 0)], "160.0"],
  ])("%s → %s", (_name, totals, expected) => {
    // 締め済と承認済を交互に並べる（どちらも確定した月）。
    const rows = totals.map((t, i) => closed(i % 2 === 0 ? "PendingApproval" : "Approved", t, `2026-0${i + 1}`));
    expect(averageLabel(rows)).toBe(expected);
  });

  // 4-1-d・ADR 0021 決定2: 正確な値（分の整数の比）に端数の扱いを当てる。浮動小数へ落としてから丸めると、
  // 正確には小数第2位が 5 ちょうどの値が、2 進数の表現誤差で切り捨て側へ寄る組がある。期待値は手で導いた。
  //   150時間27分 = 9027 分 → 9027 / 60 = 150.45 → 小数第2位が 5 → 切り上げ → 150.5（浮動小数の toFixed(1) は 150.4）
  //   107時間03分 = 6423 分 → 6423 / 60 = 107.05 → 切り上げ → 107.1（同 107.0）
  //   147時間57分 = 8877 分 → 8877 / 60 = 147.95 → 切り上げ → 繰り上がって 148.0（同 147.9）
  //   150時間27分・150時間27分（2 件の平均 = 150.45）→ 150.5
  //   150時間00分・150時間00分・150時間09分（3 件。合計 9000 + 9000 + 9009 = 27009 分）→ 27009 / (60 × 3) = 150.05 → 小数第2位が 5 → 切り上げ → 150.1
  //     （件数で割る式 (sum / 60 / n) の浮動小数は 150.0499… となり 150.0 に寄る。分の整数の比のまま丸める）
  it.each<[string, Hm[], string]>([
    ["150時間27分（150.45 → 150.5）", [hm(150, 27)], "150.5"],
    ["107時間03分（107.05 → 107.1）", [hm(107, 3)], "107.1"],
    ["147時間57分（147.95 → 繰り上がって 148.0）", [hm(147, 57)], "148.0"],
    ["150時間27分が2件（平均 150.45 → 150.5）", [hm(150, 27), hm(150, 27)], "150.5"],
    ["150時間00分・150時間00分・150時間09分（平均 150.05 → 150.1。3 件で割る形）", [hm(150, 0), hm(150, 0), hm(150, 9)], "150.1"],
  ])("端数がちょうど半分: %s", (_name, totals, expected) => {
    const rows = totals.map((t, i) => closed("Approved", t, `2026-0${i + 1}`));
    expect(averageLabel(rows)).toBe(expected);
  });

  it("(a) の組に入力中の月を足しても、平均に含めない（4-1）", () => {
    const rows = [
      closed("PendingApproval", hm(160, 0), "2026-05"),
      draft("2026-06", { totalHours: hm(10, 0) }),
      closed("Approved", hm(161, 0), "2026-07"),
    ];
    expect(averageLabel(rows)).toBe("160.5");
  });

  it("期間の外の月は含めない（rowsInPeriod で絞った行を受ける。3-6）", () => {
    const rows = [
      closed("Approved", hm(10, 0), "2026-04"),
      closed("Approved", hm(160, 0), "2026-05"),
      closed("Approved", hm(161, 0), "2026-06"),
      closed("Approved", hm(300, 0), "2026-07"),
    ];
    expect(averageLabel(rowsInPeriod(rows, { start: "2026-05", end: "2026-06" }))).toBe("160.5");
  });

  it.each<[string, WorkTrendRow[]]>([
    ["空の配列", []],
    ["すべて入力中", [draft("2026-06", { totalHours: hm(100) }), draft("2026-07", { totalHours: hm(120) })]],
  ])("確定した月が0件（%s）→ 値を返さない（0 と示さない。4-2）", (_name, rows) => {
    expect(averageLabel(rows)).toBeUndefined();
  });

  it("入力を書き換えない", () => {
    const rows = [closed("Approved", hm(160, 0)), closed("Approved", hm(161, 0), "2026-08")];
    const copy = structuredClone(rows);
    averageLabel(rows);
    expect(rows).toEqual(copy);
  });
});

describe("withinCount（4-3・8-3）", () => {
  it("確定した月のうち収まった月を数える。入力中は数えない", () => {
    const rows = [
      tr("2026-03"), // 収まった
      tr("2026-04", { state: "PendingApproval" }), // 収まった
      tr("2026-05", { excess: hm(0, 15) }), // 超過
      tr("2026-06", { shortfall: hm(2, 0) }), // 不足
      draft("2026-07"), // 入力中（数えない）
      draft("2026-08", { totalHours: hm(160) }), // 入力中（数えない）
    ];
    expect(withinCount(rows)).toEqual({ within: 2, settled: 4 });
  });

  it("総稼働時間が精算幅の外でも、保存された値が 0 なら収まった側に数える（4-3-a）", () => {
    expect(withinCount([tr("2026-03", { totalHours: hm(200) })])).toEqual({ within: 1, settled: 1 });
  });

  it("すべて入力中 → 0 / 0（8-3）", () => {
    expect(withinCount([draft("2026-06"), draft("2026-07")])).toEqual({ within: 0, settled: 0 });
  });

  it("空の配列 → 0 / 0", () => {
    expect(withinCount([])).toEqual({ within: 0, settled: 0 });
  });

  it("確定した月がすべて収まった／すべて外れた", () => {
    expect(withinCount([tr("2026-03"), tr("2026-04")])).toEqual({ within: 2, settled: 2 });
    expect(withinCount([tr("2026-03", { excess: hm(1) }), tr("2026-04", { shortfall: hm(1) })])).toEqual({
      within: 0,
      settled: 2,
    });
  });
});

describe("settlementRangeSummary（4-4・4-4-a・4-4-b）", () => {
  it("すべて同じ → same と、その精算幅", () => {
    expect(settlementRangeSummary([tr("2026-06"), tr("2026-07"), draft("2026-08")])).toEqual({
      kind: "same",
      range: RANGE,
    });
  });

  it("1件だけ → same", () => {
    expect(settlementRangeSummary([tr("2026-06")])).toEqual({ kind: "same", range: RANGE });
  });

  it("入力中の月だけ精算幅が違う → varies（入力中の月を含めて判定する。4-4）", () => {
    const other = { lowerBound: hm(150), upperBound: hm(190) };
    expect(settlementRangeSummary([tr("2026-06"), tr("2026-07"), draft("2026-08", { settlementRange: other })])).toEqual({
      kind: "varies",
    });
  });

  it.each<[string, WorkTrendRow["settlementRange"]]>([
    ["下限の hours だけ違う", { lowerBound: hm(141), upperBound: hm(180) }],
    ["下限の minutes だけ違う", { lowerBound: hm(140, 15), upperBound: hm(180) }],
    ["上限の hours だけ違う", { lowerBound: hm(140), upperBound: hm(181) }],
    ["上限の minutes だけ違う", { lowerBound: hm(140), upperBound: hm(180, 30) }],
  ])("%s → varies", (_name, other) => {
    expect(settlementRangeSummary([tr("2026-06"), tr("2026-07", { settlementRange: other })])).toEqual({
      kind: "varies",
    });
  });
});

describe("axisRange（5-7。SP-7 (b)）", () => {
  it("始まりが 0 より大きくなる組: 最小 = 精算幅の下限 140 → 120、最大 = 上限 180 → 190", () => {
    expect(axisRange([tr("2026-07", { totalHours: hm(150) })])).toEqual({ start: 120, end: 190 });
  });

  it("入力中の月だけが低い組: 入力中を最小に含める → 始まりが 0（含めなければ 120）", () => {
    const rows = [tr("2026-06", { totalHours: hm(150) }), draft("2026-07", { totalHours: hm(20, 30) })];
    expect(axisRange(rows)).toEqual({ start: 0, end: 190 });
  });

  it("最小が 5 時間でも始まりは負にならない（max(0, …)）", () => {
    expect(axisRange([draft("2026-07", { totalHours: hm(5) })])).toEqual({ start: 0, end: 190 });
  });

  it("最小がちょうど 150（境界。(150-20)/10 = 13 → 130）、最大がちょうど 170（(170+10)/10 = 18 → 180）", () => {
    const range = { lowerBound: hm(150), upperBound: hm(170) };
    expect(axisRange([tr("2026-07", { settlementRange: range, totalHours: hm(160) })])).toEqual({
      start: 130,
      end: 180,
    });
  });

  it("最小が 139時間45分（139.75 → floor(11.975) = 11 → 110）", () => {
    expect(axisRange([tr("2026-07", { totalHours: hm(139, 45) })])).toEqual({ start: 110, end: 190 });
  });

  it("最大が精算幅の上限を超える総稼働時間 185 → ceil(19.5) = 20 → 200", () => {
    expect(axisRange([tr("2026-07", { totalHours: hm(185), excess: hm(5) })])).toEqual({ start: 120, end: 200 });
  });

  it("最大が入力中の月の総稼働時間 200 → 210（入力中を最大にも含める）", () => {
    expect(axisRange([tr("2026-06", { totalHours: hm(160) }), draft("2026-07", { totalHours: hm(200) })])).toEqual({
      start: 120,
      end: 210,
    });
  });

  it("精算幅が月で異なるとき、最小は最も低い下限、最大は最も高い上限", () => {
    const rows = [
      tr("2026-06", { settlementRange: { lowerBound: hm(130), upperBound: hm(170) }, totalHours: hm(150) }),
      tr("2026-07", { settlementRange: { lowerBound: hm(150), upperBound: hm(190) }, totalHours: hm(160) }),
    ];
    expect(axisRange(rows)).toEqual({ start: 110, end: 200 });
  });
});

describe("monthAxisLabels（5-6・S-1。SP-7 (i)）", () => {
  const labels = (yms: string[]) => monthAxisLabels(yms.map((ym) => tr(ym)));

  it("年をまたぐ期間: 先頭の月と各年の1月に年が付く", () => {
    expect(labels(["2025-11", "2025-12", "2026-01", "2026-02"])).toEqual([
      "2025年11月",
      "12月",
      "2026年1月",
      "2月",
    ]);
  });

  it("年の途中から始まる期間: 先頭だけに年が付く", () => {
    expect(labels(["2026-07", "2026-08"])).toEqual(["2026年7月", "8月"]);
  });

  it("1件だけの期間: 年を付けた形", () => {
    expect(labels(["2026-08"])).toEqual(["2026年8月"]);
    expect(labels(["2026-01"])).toEqual(["2026年1月"]);
  });

  it("先頭が1月: 先頭と1月が重なっても年は1回だけ", () => {
    expect(labels(["2026-01", "2026-02", "2026-03"])).toEqual(["2026年1月", "2月", "3月"]);
  });

  it("2回の1月（2年をまたぐ）にそれぞれ年が付き、月に先頭の0を付けない", () => {
    expect(labels(["2025-10", "2025-11", "2025-12", "2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07", "2026-08", "2026-09", "2026-10", "2026-11", "2026-12", "2027-01"])).toEqual([
      "2025年10月",
      "11月",
      "12月",
      "2026年1月",
      "2月",
      "3月",
      "4月",
      "5月",
      "6月",
      "7月",
      "8月",
      "9月",
      "10月",
      "11月",
      "12月",
      "2027年1月",
    ]);
  });

  it("先頭の月は、受けた行の先頭（期間で絞った後の先頭）であって、契約の最古の月ではない", () => {
    const all = ["2026-05", "2026-06", "2026-07", "2026-08"].map((ym) => tr(ym));
    const inPeriod = rowsInPeriod(all, { start: "2026-07", end: "2026-08" });
    expect(monthAxisLabels(inPeriod)).toEqual(["2026年7月", "8月"]);
  });

  it("受けた行と同じ数・同じ順で返し、0 件は空の配列。入力を書き換えない", () => {
    expect(monthAxisLabels([])).toEqual([]);
    const rows = [tr("2026-09"), tr("2026-10"), tr("2026-11")];
    const copy = structuredClone(rows);
    expect(monthAxisLabels(rows)).toHaveLength(3);
    expect(rows).toEqual(copy);
  });
});
