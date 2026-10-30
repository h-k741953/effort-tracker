/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { defaultScreenPath } from "@/lib/screen-access";
import type { Viewer } from "@/lib/viewer";
import {
  CONTRACTS_PATH,
  JULY_RECORDS,
  dailyRecord,
  deferredResponse,
  errorBody,
  hm,
  installRoutedFetch,
  jsonResponse,
  t,
  workMonthApi,
  workMonthBody,
  type RoutedFetch,
  type RoutedHandler,
  type TestWorkMonthState,
  type WorkMonthBodyOptions,
} from "@/test-support/work-month";
import { NEW_TIME_ENTRY_FORM, type TimeEntryForm } from "@/lib/time-entry";
import { WorkMonthView } from "./work-month-view";

// docs/specs/work-month-screen-ui.md AC-1〜AC-7、daily-record-entry-ui.md 2-1〜2-3・3-3・3-6-c・4-3・4-10・5-2・6-1・6-2・
// 7-1・7-2、monthly-closing-ui.md 3-1〜3-4・6-1、approval-ui.md 4-1〜4-4・5-1〜5-4・10-8、work-month-listing-ui.md 10-9-e・10-9-f。
// 設計 M-2〜M-10・T-0・T-3。globalThis.fetch に手書きの Fake を置き、next/navigation の useRouter だけを vi.mock する。
// isHoliday は Fake の関数を props で渡す（lib/holidays/ を輸入しない）。表現の本文は OPERATOR_OWNERSHIP_KEY を輸入して組む
// （test-support/work-month.ts の workMonthBody の ownership 引数）。時計は Date だけを 2026-08-15 に固める。

const router = vi.hoisted(() => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const ENGINEER: Viewer = { kind: "signed-in", role: "Engineer" };
const APPROVER: Viewer = { kind: "signed-in", role: "Approver" };
const GUEST: Viewer = { kind: "guest" };

const MONTH = workMonthApi("c-001", "2026-07");
const OTHER_MONTH = workMonthApi("c-other", "2026-07");
const NO_HOLIDAY = () => false;

const LOAD_ERROR = "勤務月を読み込めませんでした。通信状態を確認して、もう一度お試しください。";
const MESSAGES = {
  invalidInput: "入力した内容を受け付けられませんでした。内容を見直して、もう一度保存してください。",
  notAllowed: "この勤務月では、この操作はできません。",
  stateChanged: "勤務月の状態が変わっていたため、最新の内容に更新しました。",
  communication: "通信に失敗しました。時間をおいて、もう一度お試しください。",
};

let current: RoutedFetch | undefined;

beforeEach(() => {
  router.refresh.mockReset();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-08-15T00:00:00Z"));
  current = undefined;
});

afterEach(() => {
  // どの行でも R-10 への要求が0回であり、登録の無い要求が無い（M-3）
  if (current !== undefined) {
    expect(current.requests.filter((r) => r.pathname === CONTRACTS_PATH)).toHaveLength(0);
    expect(current.unhandled).toEqual([]);
  }
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function backend(routes: Record<string, RoutedHandler>): RoutedFetch {
  current = installRoutedFetch(routes);
  return current;
}

/** R-1 だけを返す Fake。 */
function getOnly(options: WorkMonthBodyOptions, path = MONTH) {
  return backend({ [`GET ${path}`]: () => jsonResponse(workMonthBody(options)) });
}

interface ViewProps {
  viewer?: Viewer;
  contractId?: string;
  yearMonth?: string;
  isHoliday?: (date: string) => boolean;
}

function view(props: ViewProps = {}) {
  return (
    <WorkMonthView
      viewer={props.viewer ?? ENGINEER}
      contractId={props.contractId ?? "c-001"}
      yearMonth={props.yearMonth ?? "2026-07"}
      isHoliday={props.isHoliday ?? NO_HOLIDAY}
    />
  );
}

async function renderLoaded(props: ViewProps = {}) {
  const result = render(view(props));
  await screen.findByRole("heading", { level: 1 });
  return result;
}

const dayButton = (re: RegExp) => screen.getByRole("button", { name: re });
const dayButtons = () => screen.queryAllByRole("button", { name: /^\d+月\d+日/ });
const formControl = () => screen.queryByRole("combobox", { name: "開始（時）" });

/** 操作の要素（入力・フォーム・締め・承認・差戻し）が1つも無い。 */
function expectNoOperations() {
  expect(dayButtons()).toHaveLength(0);
  expect(formControl()).toBeNull();
  expect(screen.queryByRole("button", { name: "保存" })).toBeNull();
  expect(screen.queryByRole("button", { name: "削除" })).toBeNull();
  expect(screen.queryByRole("button", { name: "締める" })).toBeNull();
  expect(screen.queryByRole("button", { name: "承認" })).toBeNull();
  expect(screen.queryByRole("button", { name: "差戻し" })).toBeNull();
  expect(screen.queryByRole("button", { name: "稼働実績を入力する" })).toBeNull();
}

/** 閲覧の要素（見出し・状態バッジ・総稼働時間・精算幅・カレンダー）が出ている。 */
function expectReadable(stateLabel: string, contractName = "A社 / 保守") {
  expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(`${contractName} × 2026年7月`);
  expect(screen.getByText(stateLabel)).toBeTruthy();
  expect(screen.getByRole("heading", { name: "総稼働時間" })).toBeTruthy();
  expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
  expect(screen.getByRole("heading", { name: "精算幅" })).toBeTruthy();
  expect(screen.getByText(/140\.0〜180\.0/)).toBeTruthy();
  expect(screen.getByRole("heading", { name: "日次の稼働実績" })).toBeTruthy();
  expect(screen.getAllByText("月")).not.toHaveLength(0); // 曜日の見出し
  expect(screen.queryAllByRole("alert")).toHaveLength(0);
}

function pickLabel(name: string, label: string) {
  const el = screen.getByRole("combobox", { name });
  const option = within(el).getByRole("option", { name: label }) as HTMLOptionElement;
  fireEvent.change(el, { target: { value: option.value } });
}

function setTime(prefix: "開始" | "終了" | "休憩開始" | "休憩終了", hour: number, minute: number) {
  pickLabel(`${prefix}（時）`, String(hour));
  pickLabel(`${prefix}（分）`, String(minute).padStart(2, "0"));
}

/** 画面のフォームの値を読む（選択の value。休憩なしのときは休憩の欄が無い）。 */
function readFormValues(): TimeEntryForm {
  const value = (name: string) => (screen.getByRole("combobox", { name }) as HTMLSelectElement).value;
  const selection = (prefix: string) => ({ hour: value(`${prefix}（時）`), minute: value(`${prefix}（分）`) });
  const hasBreak = (screen.getByRole("checkbox", { name: "休憩あり" }) as HTMLInputElement).checked;
  return {
    start: selection("開始"),
    end: selection("終了"),
    hasBreak,
    breakStart: hasBreak ? selection("休憩開始") : { hour: "", minute: "" },
    breakEnd: hasBreak ? selection("休憩終了") : { hour: "", minute: "" },
  };
}

function enterWorkingDay() {
  setTime("開始", 9, 0);
  setTime("終了", 18, 0);
}

async function confirmDialog(label: string) {
  const dialog = await screen.findByRole("dialog");
  fireEvent.click(within(dialog).getByRole("button", { name: label }));
}

const SAVED_RECORD = dailyRecord("2026-07-03", t(9, 0), t(18, 0), { start: t(12, 0), end: t(13, 0) }, hm(7, 45));

describe("① 技術者本人（Engineer・真偽 true。Draft・generated 真）", () => {
  const body = (extra: WorkMonthBodyOptions = {}): WorkMonthBodyOptions => ({ ownership: true, ...extra });

  it("カレンダーの日が button（31日）。日を選ぶとフォームが出て、実績の時刻が初期値になる", async () => {
    getOnly(body());
    await renderLoaded();
    expect(dayButtons()).toHaveLength(31);
    expect(formControl()).toBeNull();
    fireEvent.click(dayButton(/^7月1日/));
    expect(screen.getByRole("combobox", { name: "開始（時）" })).toBeTruthy();
    expect((screen.getByRole("combobox", { name: "開始（時）" }) as HTMLSelectElement).selectedOptions[0].textContent).toBe("9");
    expect(dayButton(/^7月1日/).getAttribute("aria-pressed")).toBe("true");
  });

  it("実績が1件以上で日を選んでいないとき、入力の案内の文が出る", async () => {
    getOnly(body());
    await renderLoaded();
    expect(screen.getByText("カレンダーの日を選ぶと、ここで稼働時間を入力・編集・削除できます。")).toBeTruthy();
  });

  it("保存: PUT が1回。応答の totalHours と日の値で置き換わる（保存前の稼働時間 8.0 ではなく応答の 7.75）", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(body())),
      [`PUT ${MONTH}/daily-records/2026-07-03`]: () =>
        jsonResponse(
          workMonthBody(body({ totalHours: hm(24, 30), dailyRecords: [...JULY_RECORDS, SAVED_RECORD] })),
        ),
    });
    await renderLoaded();
    fireEvent.click(dayButton(/^7月3日/));
    enterWorkingDay();
    expect(screen.getByRole("heading", { name: "稼働実績の入力" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(screen.getByText(/^\s*24\.5\s*(時間)?\s*$/)).toBeTruthy());
    expect(fake.count("PUT", `${MONTH}/daily-records/2026-07-03`)).toBe(1);
    const put = fake.requests.find((r) => r.method === "PUT")!;
    expect(put.body).toEqual({
      startTime: { hour: 9, minute: 0 },
      endTime: { hour: 18, minute: 0 },
      break: { start: { hour: 12, minute: 0 }, end: { hour: 13, minute: 0 } },
    });
    expect(dayButton(/^7月3日（金） 7\.75 時間（9:00〜18:00）/)).toBeTruthy();
    expect(screen.queryByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeNull();
  });

  it("削除: DELETE が1回。その日が「未入力」へ戻り、総稼働時間が応答の値になる", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(body())),
      [`DELETE ${MONTH}/daily-records/2026-07-01`]: () =>
        jsonResponse(workMonthBody(body({ totalHours: hm(8, 45), dailyRecords: [JULY_RECORDS[1]] }))),
    });
    await renderLoaded();
    fireEvent.click(dayButton(/^7月1日/));
    fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(dayButton(/^7月1日（水） 未入力/)).toBeTruthy());
    expect(fake.count("DELETE", `${MONTH}/daily-records/2026-07-01`)).toBe(1);
    expect(screen.getByText(/^\s*8\.75\s*(時間)?\s*$/)).toBeTruthy();
  });

  it("削除の成功の後、フォームは NEW_TIME_ENTRY_FORM の値へ戻る（M-7。削除の前は実績の値＝休憩なしの 9:00〜17:45）", async () => {
    backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(body())),
      [`DELETE ${MONTH}/daily-records/2026-07-02`]: () =>
        jsonResponse(workMonthBody(body({ totalHours: hm(8, 0), dailyRecords: [JULY_RECORDS[0]] }))),
    });
    await renderLoaded();
    fireEvent.click(dayButton(/^7月2日/));
    const before = readFormValues();
    expect(before).not.toEqual(NEW_TIME_ENTRY_FORM);
    expect(before.start).toEqual({ hour: "9", minute: "0" });
    expect(before.hasBreak).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(dayButton(/^7月2日（木） 未入力/)).toBeTruthy());
    expect(readFormValues()).toEqual(NEW_TIME_ENTRY_FORM);
  });

  it("締め: 「締める」→ 確認 → 確定で POST /close が1回。状態バッジが「締め済」・超過／不足の数値が出て、入力の button が消える", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(body())),
      [`POST ${MONTH}/close`]: () =>
        jsonResponse(workMonthBody(body({ state: "PendingApproval", excess: hm(0, 0), shortfall: hm(123, 15) }))),
    });
    await renderLoaded();
    expect(screen.getByText("下書き")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    expect(fake.count("POST", `${MONTH}/close`)).toBe(0);
    await confirmDialog("締める");
    await waitFor(() => expect(screen.getByText("締め済")).toBeTruthy());
    expect(fake.count("POST", `${MONTH}/close`)).toBe(1);
    expect(screen.getByText(/^\s*123\.25\s*(時間)?\s*$/)).toBeTruthy();
    expect(screen.getByRole("heading", { name: "超過" })).toBeTruthy();
    expect(dayButtons()).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "締める" })).toBeNull();
    expect(screen.getByText("締めました。承認されるまで稼働実績は編集できません。")).toBeTruthy();
    // 総稼働時間・精算幅は引き続き表示する（締め UI 3-4）
    expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
    expect(screen.getByText(/140\.0〜180\.0/)).toBeTruthy();
  });

  it("確認で取消すと POST を送らず、Draft のまま", async () => {
    const fake = getOnly(body());
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "取消" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(fake.nonGetCount()).toBe(0);
    expect(screen.getByText("下書き")).toBeTruthy();
  });

  it("空月（generated 真・実績0件）で「締める」がある（締め UI 5-2）", async () => {
    getOnly(body({ dailyRecords: [], totalHours: hm(0, 0) }));
    await renderLoaded();
    expect(screen.getByRole("button", { name: "締める" })).toBeTruthy();
  });

  it("実績0件で日を選んでいないとき、空状態（title・description・「稼働実績を入力する」）が出る（入力 UI 2-1）", async () => {
    getOnly(body({ dailyRecords: [], totalHours: hm(0, 0) }));
    await renderLoaded();
    expect(screen.getByText("まだ稼働実績がありません")).toBeTruthy();
    expect(
      screen.getByText("カレンダーの日を選んで、稼働時間を入力してください。最初の入力でこの勤務月が作られます。"),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "稼働実績を入力する" })).toBeTruthy();
  });

  it("isHoliday は props で受け、祝日のマスに札「祝」が出る（M-1・M-6）", async () => {
    getOnly(body());
    await renderLoaded({ isHoliday: (date) => date === "2026-07-20" });
    expect(screen.getAllByText("祝")).toHaveLength(1);
    expect(dayButton(/^7月20日（祝）/)).toBeTruthy();
  });
});

describe("未生成の当月（generated 偽。10-9-e・入力 UI 2-2・2-3）", () => {
  const ungenerated = (): WorkMonthBodyOptions => ({
    ownership: true,
    generated: false,
    dailyRecords: [],
    totalHours: hm(0, 0),
  });

  it("空状態の title と「稼働実績を入力する」が出る。「締める」が無い。開いただけでは GET 以外を送らない", async () => {
    const fake = getOnly(ungenerated());
    await renderLoaded();
    expect(screen.getByText("まだ稼働実績がありません")).toBeTruthy();
    expect(screen.getByRole("button", { name: "稼働実績を入力する" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "締める" })).toBeNull();
    expect(screen.queryByRole("button", { name: /月を開く/ })).toBeNull();
    expect(fake.requests.map((r) => `${r.method} ${r.pathname}`)).toEqual([`GET ${MONTH}`]);
  });

  it("最初の保存の応答が generated 真なら「締める」が出る", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(ungenerated())),
      [`PUT ${MONTH}/daily-records/2026-07-01`]: () =>
        jsonResponse(
          workMonthBody({
            ownership: true,
            generated: true,
            dailyRecords: [JULY_RECORDS[0]],
            totalHours: hm(8, 0),
          }),
        ),
    });
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "稼働実績を入力する" }));
    enterWorkingDay();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "締める" })).toBeTruthy());
    expect(fake.count("PUT", `${MONTH}/daily-records/2026-07-01`)).toBe(1);
  });
});

describe("② 他人の契約（Engineer・真偽 false。c-other を開く。M-10）", () => {
  const cases: Array<[TestWorkMonthState, string]> = [
    ["Draft", "下書き"],
    ["PendingApproval", "締め済"],
    ["Approved", "承認済"],
  ];

  it.each(cases)("%s: 操作要素が1つも無く、閲覧の要素が出る。GET 以外を送らない", async (state, label) => {
    const fake = getOnly(
      { ownership: false, state, contractId: "c-other", contractDisplayName: "B社 / 開発" },
      OTHER_MONTH,
    );
    await renderLoaded({ contractId: "c-other" });
    expectNoOperations();
    expectReadable(label, "B社 / 開発");
    expect(fake.nonGetCount()).toBe(0);
    expect(fake.requests.map((r) => `${r.method} ${r.pathname}`)).toEqual([`GET ${OTHER_MONTH}`]);
  });
});

describe("③ 承認者（Approver・真偽 false）", () => {
  const pending = (): WorkMonthBodyOptions => ({ ownership: false, state: "PendingApproval" });

  it("PendingApproval: 「承認」「差戻し」が出る。入力・「締める」は無い", async () => {
    getOnly(pending());
    await renderLoaded({ viewer: APPROVER });
    expect(screen.getByRole("button", { name: "承認" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "差戻し" })).toBeTruthy();
    expect(dayButtons()).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "締める" })).toBeNull();
    expect(screen.getByText("技術者が締めた勤務月です。")).toBeTruthy();
  });

  it("承認の確定で POST /approve が1回。状態バッジが応答の「承認済」へ変わり、「承認」「差戻し」が消える", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(pending())),
      [`POST ${MONTH}/approve`]: () => jsonResponse(workMonthBody({ ownership: false, state: "Approved" })),
    });
    await renderLoaded({ viewer: APPROVER });
    fireEvent.click(screen.getByRole("button", { name: "承認" }));
    expect(fake.count("POST", `${MONTH}/approve`)).toBe(0);
    await confirmDialog("承認する");
    await waitFor(() => expect(screen.getByText("承認済")).toBeTruthy());
    expect(fake.count("POST", `${MONTH}/approve`)).toBe(1);
    expect(screen.queryByRole("button", { name: "承認" })).toBeNull();
    expect(screen.queryByRole("button", { name: "差戻し" })).toBeNull();
    // 承認 UI 4-3: 総稼働時間を締め時の値のまま表示する
    expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
  });

  it("差戻しの確定で POST /reject が1回。状態バッジが応答の「下書き」へ変わる", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(pending())),
      [`POST ${MONTH}/reject`]: () => jsonResponse(workMonthBody({ ownership: false, state: "Draft" })),
    });
    await renderLoaded({ viewer: APPROVER });
    fireEvent.click(screen.getByRole("button", { name: "差戻し" }));
    await confirmDialog("差し戻す");
    await waitFor(() => expect(screen.getByText("下書き")).toBeTruthy());
    expect(fake.count("POST", `${MONTH}/reject`)).toBe(1);
    // 差戻しの理由・回数を表示しない（承認 UI 5-3・5-4）
    expect(screen.queryByText(/理由|回数/)).toBeNull();
    expect(screen.queryByRole("button", { name: "承認" })).toBeNull();
    // 承認者は本人でないので、差戻し後の Draft でも入力・「締める」は出ない
    expect(dayButtons()).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "締める" })).toBeNull();
  });

  it("Draft: 入力の button・「締める」が無い", async () => {
    getOnly({ ownership: false, state: "Draft" });
    await renderLoaded({ viewer: APPROVER });
    expectNoOperations();
  });
});

describe("④ 承認者かつ本人（Approver・真偽 true）", () => {
  it("Draft: 入力の button と「締める」がある（AC-3 ④）", async () => {
    getOnly({ ownership: true, state: "Draft" });
    await renderLoaded({ viewer: APPROVER });
    expect(dayButtons()).toHaveLength(31);
    expect(screen.getByRole("button", { name: "締める" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "承認" })).toBeNull();
  });

  it("PendingApproval: 「承認」「差戻し」が無い（承認 UI 3-1）。注記が出る", async () => {
    getOnly({ ownership: true, state: "PendingApproval" });
    await renderLoaded({ viewer: APPROVER });
    expectNoOperations();
    expect(screen.getByText("ご自身の勤務月のため、承認・差戻しはできません。")).toBeTruthy();
  });
});

describe("真偽が無い（キーを持たない本文。M-3）", () => {
  const states: Array<[TestWorkMonthState, string]> = [
    ["Draft", "下書き"],
    ["PendingApproval", "締め済"],
  ];

  describe.each([
    ["Engineer", ENGINEER],
    ["Approver", APPROVER],
  ])("%s", (_role, viewer) => {
    it.each(states)("%s: 操作要素が1つも無く、閲覧の要素が出て、取得の失敗にしない", async (state, label) => {
      const fake = getOnly({ state });
      await renderLoaded({ viewer });
      expectNoOperations();
      expectReadable(label);
      expect(fake.nonGetCount()).toBe(0);
    });
  });
});

describe("取得中・read-only・ゲスト", () => {
  it("取得中（R-1 の応答を保留した間）: 操作要素が無く、内容の領域が aria-busy=true", async () => {
    const pending = deferredResponse();
    backend({ [`GET ${MONTH}`]: () => pending.promise });
    const { container } = render(view());
    expect(container.querySelector('[aria-busy="true"]')).not.toBeNull();
    expectNoOperations();
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    pending.resolve(jsonResponse(workMonthBody({ ownership: true })));
    await screen.findByRole("heading", { level: 1 });
  });

  it.each<[TestWorkMonthState]>([["PendingApproval"], ["Approved"]])(
    "%s の本人（Engineer・真偽 true）: 操作要素が無い（5-1・5-2）",
    async (state) => {
      getOnly({ ownership: true, state });
      await renderLoaded();
      expectNoOperations();
    },
  );

  it.each<[TestWorkMonthState]>([["Draft"], ["PendingApproval"], ["Approved"]])(
    "ゲスト・%s（真偽 true の本文でも）: 操作要素が無く、ゲストの注記が出る",
    async (state) => {
      const fake = getOnly({ ownership: true, state });
      await renderLoaded({ viewer: GUEST });
      expectNoOperations();
      const note = screen.getByRole("note");
      expect(note.textContent).toBe("デモ用のサンプルです（閲覧のみ・毎日リセットされます）");
      expect(fake.nonGetCount()).toBe(0);
    },
  );
});

describe("取得の結果（M-9）", () => {
  const VIEWERS: Array<[string, Viewer]> = [
    ["技術者", ENGINEER],
    ["承認者", APPROVER],
    ["ゲスト", GUEST],
  ];

  describe.each([
    [404, "CONTRACT_NOT_FOUND"],
    [400, "INVALID_REQUEST"],
  ])("R-1 が %i（%s）", (status, code) => {
    it.each(VIEWERS)("%s: 1つの EmptyState にまとめ、一覧へのリンクを出す。見出し・カレンダー・再試行・alert が無い", async (_name, viewer) => {
      backend({ [`GET ${MONTH}`]: () => jsonResponse(errorBody(code), status) });
      render(view({ viewer }));
      await screen.findByText("勤務月を表示できません");
      expect(screen.getByText("契約が見つからないか、URL の形式が正しくありません。")).toBeTruthy();
      const link = screen.getByRole("link", { name: "勤務月の一覧を開く" });
      expect(link.getAttribute("href")).toBe(defaultScreenPath(viewer));
      expect(screen.queryAllByRole("alert")).toHaveLength(0);
      expect(screen.queryByRole("button", { name: "再試行" })).toBeNull();
      expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
      expect(screen.queryByRole("heading", { name: "日次の稼働実績" })).toBeNull();
    });
  });

  describe.each([
    ["500", () => jsonResponse({}, 500)],
    ["例外", () => Promise.reject(new Error("network"))],
    ["403", () => jsonResponse(errorBody("FORBIDDEN_NOT_OWNER"), 403)],
  ] as Array<[string, RoutedHandler]>)("R-1 が %s", (_name, failing) => {
    it("alert が取得の失敗の文言で出て、見出し・カレンダーが無い。再試行で GET がもう1回送られ、ok なら表現が出る", async () => {
      let calls = 0;
      const fake = backend({
        [`GET ${MONTH}`]: (request) => {
          calls += 1;
          return calls === 1 ? failing(request) : jsonResponse(workMonthBody({ ownership: true }));
        },
      });
      render(view());
      const alert = await screen.findByRole("alert");
      expect(within(alert).getByText(LOAD_ERROR)).toBeTruthy();
      expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
      expect(screen.queryByRole("heading", { name: "日次の稼働実績" })).toBeNull();
      expect(screen.queryByRole("heading", { name: "勤務月を表示できません" })).toBeNull();
      fireEvent.click(within(alert).getByRole("button", { name: "再試行" }));
      await screen.findByRole("heading", { level: 1 });
      expect(fake.count("GET", MONTH)).toBe(2);
      expect(screen.queryAllByRole("alert")).toHaveLength(0);
    });
  });

  it("R-1 が 401: router.refresh がちょうど1回", async () => {
    backend({ [`GET ${MONTH}`]: () => jsonResponse(errorBody("UNAUTHENTICATED"), 401) });
    render(view());
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(router.refresh).toHaveBeenCalledTimes(1);
  });
});

describe("操作の結果（M-9）", () => {
  const PUT_PATH = `${MONTH}/daily-records/2026-07-03`;
  const own = (extra: WorkMonthBodyOptions = {}): WorkMonthBodyOptions => ({ ownership: true, ...extra });

  /** 日を選んで保存を押すところまで進める。 */
  async function save() {
    await renderLoaded();
    fireEvent.click(dayButton(/^7月3日/));
    enterWorkingDay();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
  }

  it("401: router.refresh がちょうど1回で、表示値が変わらない", async () => {
    backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(own())),
      [`PUT ${PUT_PATH}`]: () => jsonResponse(errorBody("UNAUTHENTICATED"), 401),
    });
    await save();
    await waitFor(() => expect(router.refresh).toHaveBeenCalledTimes(1));
    expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
    expect(screen.queryAllByRole("alert")).toHaveLength(0);
  });

  it("400: invalid-input の文が alert で出て、選んだ値が残り、総稼働時間が変わらない（4-4・4-10）", async () => {
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(own())),
      [`PUT ${PUT_PATH}`]: () => jsonResponse(errorBody("INVALID_TIME_COMBINATION"), 400),
    });
    await save();
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(MESSAGES.invalidInput)).toBeTruthy();
    expect((screen.getByRole("combobox", { name: "開始（時）" }) as HTMLSelectElement).selectedOptions[0].textContent).toBe("9");
    expect((screen.getByRole("combobox", { name: "終了（時）" }) as HTMLSelectElement).selectedOptions[0].textContent).toBe("18");
    expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
    expect(fake.count("GET", MONTH)).toBe(1);
  });

  it.each([403, 404])("%i: not-allowed の文が出て、表示値が変わらない", async (status) => {
    backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(own())),
      [`PUT ${PUT_PATH}`]: () => jsonResponse(errorBody("FORBIDDEN_NOT_OWNER"), status),
    });
    await save();
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(MESSAGES.notAllowed)).toBeTruthy();
    expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
  });

  it("409（保存）: R-1 が取り直され、取り直した表現（PendingApproval）で入力の button が消え、state-changed の文が出る（6-4）", async () => {
    let gets = 0;
    const fake = backend({
      [`GET ${MONTH}`]: () => {
        gets += 1;
        return jsonResponse(workMonthBody(own(gets === 1 ? {} : { state: "PendingApproval" })));
      },
      [`PUT ${PUT_PATH}`]: () => jsonResponse(errorBody("STATE_CONFLICT"), 409),
    });
    await save();
    await waitFor(() => expect(screen.getByText("締め済")).toBeTruthy());
    expect(fake.count("GET", MONTH)).toBe(2);
    expect(dayButtons()).toHaveLength(0);
    expect(within(screen.getByRole("alert")).getByText(MESSAGES.stateChanged)).toBeTruthy();
  });

  it("409（締め）: R-1 が取り直され、2回目の POST が無い（締め UI 6-1）", async () => {
    let gets = 0;
    const fake = backend({
      [`GET ${MONTH}`]: () => {
        gets += 1;
        return jsonResponse(workMonthBody(own(gets === 1 ? {} : { state: "PendingApproval" })));
      },
      [`POST ${MONTH}/close`]: () => jsonResponse(errorBody("STATE_CONFLICT"), 409),
    });
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    await confirmDialog("締める");
    await waitFor(() => expect(screen.getByText("締め済")).toBeTruthy());
    expect(fake.count("GET", MONTH)).toBe(2);
    expect(fake.count("POST", `${MONTH}/close`)).toBe(1);
  });

  it("409（承認）: R-1 が取り直される（承認 UI 10-8）", async () => {
    let gets = 0;
    const fake = backend({
      [`GET ${MONTH}`]: () => {
        gets += 1;
        return jsonResponse(workMonthBody({ ownership: false, state: gets === 1 ? "PendingApproval" : "Approved" }));
      },
      [`POST ${MONTH}/approve`]: () => jsonResponse(errorBody("STATE_CONFLICT"), 409),
    });
    await renderLoaded({ viewer: APPROVER });
    fireEvent.click(screen.getByRole("button", { name: "承認" }));
    await confirmDialog("承認する");
    await waitFor(() => expect(screen.getByText("承認済")).toBeTruthy());
    expect(fake.count("GET", MONTH)).toBe(2);
    expect(fake.count("POST", `${MONTH}/approve`)).toBe(1);
    expect(screen.queryByRole("button", { name: "承認" })).toBeNull();
  });

  it("409 の取り直しが 500 なら、取得の失敗の文言が出る", async () => {
    let gets = 0;
    backend({
      [`GET ${MONTH}`]: () => {
        gets += 1;
        return gets === 1 ? jsonResponse(workMonthBody(own())) : jsonResponse({}, 500);
      },
      [`PUT ${PUT_PATH}`]: () => jsonResponse(errorBody("STATE_CONFLICT"), 409),
    });
    await save();
    await waitFor(() => expect(screen.getByText(LOAD_ERROR)).toBeTruthy());
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
  });

  describe.each([
    ["500", () => jsonResponse({}, 500)],
    ["例外", () => Promise.reject(new Error("network"))],
  ] as Array<[string, RoutedHandler]>)("保存が %s", (_name, failing) => {
    it("communication の文が出て、表示値が変わらず、PUT は1回だけ（自動で送り直さない）。ErrorBanner に再試行の button が無い", async () => {
      const fake = backend({
        [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(own())),
        [`PUT ${PUT_PATH}`]: failing,
      });
      await save();
      const alert = await screen.findByRole("alert");
      expect(within(alert).getByText(MESSAGES.communication)).toBeTruthy();
      expect(within(alert).queryByRole("button", { name: "再試行" })).toBeNull();
      expect(screen.getByText(/^\s*16\.75\s*(時間)?\s*$/)).toBeTruthy();
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(fake.count("PUT", PUT_PATH)).toBe(1);
    });
  });

  it("締めの 500: 状態バッジが「下書き」のまま（6-3）", async () => {
    backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(own())),
      [`POST ${MONTH}/close`]: () => jsonResponse({}, 500),
    });
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "締める" }));
    await confirmDialog("締める");
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(MESSAGES.communication)).toBeTruthy();
    expect(screen.getByText("下書き")).toBeTruthy();
    expect(screen.queryByText("締め済")).toBeNull();
  });

  it("操作の失敗の ErrorBanner に再試行の button が無い（invalid-input でも）", async () => {
    backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody(own())),
      [`PUT ${PUT_PATH}`]: () => jsonResponse(errorBody("INVALID_REQUEST"), 400),
    });
    await save();
    const alert = await screen.findByRole("alert");
    expect(within(alert).queryByRole("button")).toBeNull();
  });
});

describe("ロール切替・通信中", () => {
  it("viewer の props をロール Engineer から Approver へ替えて描き直すと、R-1 の GET がもう1回送られる（M-2）", async () => {
    const fake = getOnly({ ownership: true });
    const { rerender } = await renderLoaded({ viewer: ENGINEER });
    expect(fake.count("GET", MONTH)).toBe(1);
    rerender(view({ viewer: APPROVER }));
    await waitFor(() => expect(fake.count("GET", MONTH)).toBe(2));
  });

  // M-2「締めた直後を戻す」（AI 起案）。取り直した表現の上で「締めました。」を使い回さない。
  describe("締めた直後を取り直しで偽へ戻す", () => {
    const JUST_CLOSED = "締めました。承認されるまで稼働実績は編集できません。";

    /** 最初の GET は Draft・本人、2回目以降の GET は PendingApproval。締めの POST は PendingApproval を返す。 */
    function closableBackend(afterOwnership: boolean, path = MONTH, yearMonth = "2026-07") {
      let gets = 0;
      return backend({
        [`GET ${path}`]: () => {
          gets += 1;
          return jsonResponse(
            gets === 1
              ? workMonthBody({ ownership: true, state: "Draft", yearMonth })
              : workMonthBody({ ownership: afterOwnership, state: "PendingApproval", yearMonth }),
          );
        },
        [`POST ${path}/close`]: () =>
          jsonResponse(workMonthBody({ ownership: true, state: "PendingApproval", yearMonth })),
      });
    }

    async function closeAsEngineer() {
      const result = await renderLoaded({ viewer: ENGINEER });
      fireEvent.click(screen.getByRole("button", { name: "締める" }));
      await confirmDialog("締める");
      await waitFor(() => expect(screen.getByText(JUST_CLOSED)).toBeTruthy());
      return result;
    }

    it.each([
      ["本人（真偽 true）", true, "ご自身の勤務月のため、承認・差戻しはできません。"],
      ["本人でない（真偽 false）", false, "技術者が締めた勤務月です。"],
    ])("締めた後にロールを Approver へ替えると、取り直した表現（%s）の注記は「締めました。」でない", async (_name, own, note) => {
      const fake = closableBackend(own);
      const { rerender } = await closeAsEngineer();
      rerender(view({ viewer: APPROVER }));
      await waitFor(() => expect(fake.count("GET", MONTH)).toBe(2));
      await waitFor(() => expect(screen.getByText(note)).toBeTruthy());
      expect(screen.queryByText(JUST_CLOSED)).toBeNull();
      expect(screen.queryByText(/締めました。/)).toBeNull();
    });

    it("締めた後に Approver へ替え、Engineer へ戻して同じキーへ戻っても、注記は「締め済のため編集できません。」で「締めました。」でない", async () => {
      const fake = closableBackend(true);
      const { rerender } = await closeAsEngineer();
      rerender(view({ viewer: APPROVER }));
      await waitFor(() => expect(fake.count("GET", MONTH)).toBe(2));
      await waitFor(() =>
        expect(screen.getByText("ご自身の勤務月のため、承認・差戻しはできません。")).toBeTruthy(),
      );
      rerender(view({ viewer: ENGINEER }));
      await waitFor(() => expect(fake.count("GET", MONTH)).toBe(3));
      await waitFor(() => expect(screen.getByText("締め済のため編集できません。")).toBeTruthy());
      expect(screen.queryByText(/締めました。/)).toBeNull();
    });

    it("締めた後に yearMonth の props を替えると、取り直した PendingApproval の注記は「締め済のため編集できません。」", async () => {
      const next = workMonthApi("c-001", "2026-08");
      let gets = 0;
      const fake = backend({
        [`GET ${MONTH}`]: () => jsonResponse(workMonthBody({ ownership: true, state: "Draft" })),
        [`POST ${MONTH}/close`]: () => jsonResponse(workMonthBody({ ownership: true, state: "PendingApproval" })),
        [`GET ${next}`]: () => {
          gets += 1;
          return jsonResponse(workMonthBody({ ownership: true, state: "PendingApproval", yearMonth: "2026-08" }));
        },
      });
      const { rerender } = await closeAsEngineer();
      rerender(view({ viewer: ENGINEER, yearMonth: "2026-08" }));
      await waitFor(() => expect(fake.count("GET", next)).toBe(1));
      await waitFor(() => expect(screen.getByText("締め済のため編集できません。")).toBeTruthy());
      expect(gets).toBe(1);
      expect(screen.queryByText(/締めました。/)).toBeNull();
    });
  });

  it("通信中: 保存の応答を待つ間に保存をもう一度押しても PUT は1回", async () => {
    const pending = deferredResponse();
    const putPath = `${MONTH}/daily-records/2026-07-03`;
    const fake = backend({
      [`GET ${MONTH}`]: () => jsonResponse(workMonthBody({ ownership: true })),
      [`PUT ${putPath}`]: () => pending.promise,
    });
    await renderLoaded();
    fireEvent.click(dayButton(/^7月3日/));
    enterWorkingDay();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(fake.count("PUT", putPath)).toBe(1));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fake.count("PUT", putPath)).toBe(1);
    pending.resolve(jsonResponse(workMonthBody({ ownership: true, dailyRecords: [...JULY_RECORDS, SAVED_RECORD] })));
    await waitFor(() => expect(dayButton(/^7月3日（金） 7\.75 時間/)).toBeTruthy());
    expect(fake.count("PUT", putPath)).toBe(1);
  });
});
