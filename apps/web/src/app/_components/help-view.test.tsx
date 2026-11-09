/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Card } from "@/components/card";
import { StatusBadge } from "@/components/status-badge";
import { installFakeFetch } from "@/test-support/work-month-list";
import { HelpView } from "./help-view";

// docs/specs/help-ui.md AC-2（2-1〜2-4）・AC-3・AC-4・5-1・5-2（本文）・5-3。設計 H-2・H-4。
// 文言の期待値は help-ui.md AC-3 の字面を直書きする。help-view.tsx の定数は輸入しない（H-4）。
// HelpView は props も外部依存も持たない。fetch は手書きの Fake（T-0-1）。

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const H1 = "ヘルプ";

const SECTIONS = [
  {
    id: "3-1",
    heading: "勤務月の流れ",
    lines: {
      "3-1-b": "技術者が日ごとの稼働実績を入力・編集・削除します。入力が済んだら締めます。",
      "3-1-c": "締めると承認待ちになります。承認者が、承認するか差し戻すかを選びます。この間、技術者は稼働実績を編集できません。",
      "3-1-d": "承認されると承認済になります。承認済の勤務月は、以降だれも状態を変えられません。",
      "3-1-e": "差し戻されると下書きに戻り、技術者が稼働実績を修正できるようになります。",
      "3-1-f": "締めた勤務月を、技術者が自分で下書きに戻す操作はありません。下書きに戻るのは、承認者が差し戻したときだけです。",
      "3-1-g": "自分が技術者である勤務月は、承認者であっても承認・差戻しできません。",
    },
  },
  {
    id: "3-2",
    heading: "稼働実績の入力",
    lines: {
      "3-2-b": "開始時刻と終了時刻、休憩の有無を入力します。休憩ありのときは、休憩の開始と終了の時刻も入力します。稼働時間はこれらの時刻から算出され、直接は入力しません。",
      "3-2-c": "時は 0〜23、分は 00・15・30・45 から選びます。終了時刻は開始時刻より後で、23:45 までです。日をまたぐ入力はできません。",
      "3-2-d": "休憩は1日に1回までです。休憩は開始時刻から終了時刻までの間に収めます。",
      "3-2-e": "稼働時間が 0 になる入力はできません。稼働の無い日は入力しません。",
      "3-2-f": "1日に入力できる稼働実績は1件です。同じ日に入力し直すと、前の入力を置き換えます。",
      "3-2-g": "今日より後の日には入力できません。今日の判断は日本時間で行います。",
      "3-2-h": "下書きの間は、入力済みの日の稼働実績を削除できます。",
      "3-2-i": "月の総稼働時間は、日ごとの稼働時間を15分単位で切り捨ててから合計した値です。",
    },
  },
  {
    id: "3-3",
    heading: "精算幅と超過／不足",
    lines: {
      "3-3-b": "精算幅は、契約が定める月の稼働時間の下限と上限です。",
      "3-3-c": "総稼働時間が上限を上回った分を超過、下限を下回った分を不足といいます。",
      "3-3-d": "超過と不足は、締めたときの総稼働時間から算出されます。下書きの間は表示されません。",
      "3-3-e": "勤務月の精算幅は、その勤務月が作られたときの契約の値です。後から契約が変わっても、作られた勤務月の精算幅は変わりません。",
    },
  },
] as const;

const ALL_LINES: Array<[string, string]> = SECTIONS.flatMap((s) => Object.entries<string>(s.lines));

function lineElements(text: string): HTMLElement[] {
  return screen.getAllByText(text, { exact: true });
}

function oneLine(text: string): HTMLElement {
  const found = lineElements(text);
  expect(found).toHaveLength(1);
  return found[0];
}

describe("見出し（2-1・2-2）", () => {
  it("2-1: h1 はちょうど1つで「ヘルプ」", () => {
    render(<HelpView />);
    const h1s = screen.getAllByRole("heading", { level: 1 });
    expect(h1s.map((h) => h.textContent)).toEqual([H1]);
  });

  it("2-2: h2 はちょうど3つで、文言と順は 3-1-a・3-2-a・3-3-a のとおり", () => {
    render(<HelpView />);
    expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual([
      "勤務月の流れ",
      "稼働実績の入力",
      "精算幅と超過／不足",
    ]);
  });
});

describe("節の器（2-3）", () => {
  it("各 h2 は Card の見出しとして描かれ、その Card に自分の節の行だけが入る", () => {
    // Card の器の印は、Card を単体で描いて得る（実装の写しを持たない）。
    const sample = render(
      <Card title="sample">
        <p>x</p>
      </Card>,
    );
    const cardClass = sample.container.firstElementChild!.className;
    expect(cardClass).not.toBe("");
    sample.unmount();

    render(<HelpView />);
    const h2s = screen.getAllByRole("heading", { level: 2 });
    expect(h2s).toHaveLength(SECTIONS.length);
    SECTIONS.forEach((section, i) => {
      const card = h2s[i].parentElement!;
      expect(card.className).toBe(cardClass);
      expect(card.firstElementChild).toBe(h2s[i]);
      for (const other of SECTIONS) {
        for (const text of Object.values<string>(other.lines)) {
          expect(card.contains(oneLine(text))).toBe(other === section);
        }
      }
    });
  });
});

describe("状態の表示（2-4）", () => {
  it("StatusBadge はちょうど3つ。下書き・締め済・承認済の順に各1回、3-1-b・3-1-c・3-1-d の文言と同じ組の先頭にある", () => {
    // StatusBadge の印（要素名と className）は、各 state を単体で描いて得る（実装の写しを持たない）。
    const marks = (["Draft", "PendingApproval", "Approved"] as const).map((state) => {
      const sample = render(<StatusBadge state={state} />);
      const el = sample.container.firstElementChild!;
      const mark = { tag: el.tagName, className: el.className };
      sample.unmount();
      return mark;
    });
    expect(new Set(marks.map((m) => m.className)).size).toBe(3);

    render(<HelpView />);
    // バッジの文言（design-system 6-1 / work-month-screen-ui 2-2 の呼称）は、要素の文言が全文一致するものだけを数える。
    const labels = ["下書き", "締め済", "承認済"];
    const badges = labels.map((label) => {
      const found = screen.getAllByText(label, { exact: true });
      expect(found).toHaveLength(1);
      return found[0];
    });
    // 見つけた3要素は、それぞれの state の StatusBadge と同じ印を持つ（素の span などでは通らない）。
    badges.forEach((badge, i) => {
      expect({ tag: badge.tagName, className: badge.className }, labels[i]).toEqual(marks[i]);
    });
    // 並びが 下書き → 締め済 → 承認済 の順
    expect(badges[0].compareDocumentPosition(badges[1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(badges[1].compareDocumentPosition(badges[2]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    const expected = [
      SECTIONS[0].lines["3-1-b"],
      SECTIONS[0].lines["3-1-c"],
      SECTIONS[0].lines["3-1-d"],
    ];
    const bLines = ["3-1-b", "3-1-c", "3-1-d"];
    badges.forEach((badge, i) => {
      const group = badge.parentElement!;
      // 組の先頭がバッジ。
      expect(group.firstElementChild).toBe(badge);
      // 組は自分の文言の要素を含み、他の2行・他の2つのバッジを含まない（節の Card 全体ではない）。
      const own = oneLine(expected[i]);
      expect(group.contains(own), bLines[i]).toBe(true);
      expected.forEach((text, j) => {
        if (j !== i) expect(group.contains(oneLine(text))).toBe(false);
      });
      badges.forEach((other, j) => {
        if (j !== i) expect(group.contains(other)).toBe(false);
      });
    });
  });

  it("3-1-e〜3-1-g の行には StatusBadge を置かない（行と同じ組に、バッジの文言の要素が無い）", () => {
    render(<HelpView />);
    const badgeTexts = new Set(["下書き", "締め済", "承認済"]);
    // 陽性対照: 同じ走査が 3-1-b の組では陽性（バッジを見つける）になる。
    const hasBadge = (el: HTMLElement) =>
      [...el.parentElement!.children].some((c) => c !== el && badgeTexts.has(c.textContent ?? ""));
    expect(hasBadge(oneLine(SECTIONS[0].lines["3-1-b"]))).toBe(true);
    for (const id of ["3-1-e", "3-1-f", "3-1-g"] as const) {
      expect(hasBadge(oneLine(SECTIONS[0].lines[id])), id).toBe(false);
    }
  });
});

describe("文言（AC-3）", () => {
  it.each(ALL_LINES)("%s: 字面のとおり、1つの要素として本文に出る", (_id, text) => {
    render(<HelpView />);
    expect(lineElements(text)).toHaveLength(1);
  });

  it("各節の行は、表の記載順に文書内で並ぶ", () => {
    render(<HelpView />);
    const els = ALL_LINES.map(([, text]) => oneLine(text));
    for (let i = 1; i < els.length; i++) {
      expect(els[i - 1].compareDocumentPosition(els[i]) & Node.DOCUMENT_POSITION_FOLLOWING, ALL_LINES[i][0]).toBeTruthy();
    }
  });
});

describe("隠された要素（AC-2・AC-3）", () => {
  const HIDDEN = '[hidden], [aria-hidden="true"]';

  it("陽性対照: 同じ走査が、hidden・aria-hidden の要素を見つける", () => {
    const control = render(
      <div>
        <p hidden>a</p>
        <p aria-hidden="true">b</p>
      </div>,
    );
    expect(control.container.querySelectorAll(HIDDEN)).toHaveLength(2);
  });

  it("HelpView に hidden・aria-hidden の要素が無い（見出しも AC-3 の行も隠さない）", () => {
    const { container } = render(<HelpView />);
    expect(container.textContent).toContain("精算幅");
    expect(container.querySelectorAll(HIDDEN)).toHaveLength(0);
  });
});

describe("書かないこと（AC-4）", () => {
  function bodyText() {
    return document.body.textContent ?? "";
  }

  it("陽性対照: 走査は本文の文言を実際に見つけられる", () => {
    render(<HelpView />);
    expect(bodyText()).toContain("精算幅");
    expect(bodyText()).toContain("23:45");
  });

  it("4-1: 本文に 12:00 と 13:00 が無い", () => {
    render(<HelpView />);
    expect(bodyText()).not.toContain("12:00");
    expect(bodyText()).not.toContain("13:00");
  });

  it("4-1: 文言を持つ要素の textContent が、h1・h2 3つ・状態の表示3つ・AC-3 の18行の計25個と過不足なく一致する（出典の無い文を置かない）", () => {
    // 文言を持つ要素 = 直下に空白以外のテキストノードを持つ要素。期待値は直書きの字面から組む。
    // 母集団に描画の根（render の container）を含める。Fragment で根の直下に置かれた裸のテキストを拾うため。
    const ownText = (root: HTMLElement) =>
      [root, ...root.querySelectorAll<HTMLElement>("*")]
        .filter((el) => [...el.childNodes].some((n) => n.nodeType === Node.TEXT_NODE && (n.textContent ?? "").trim() !== ""))
        .map((el) => el.textContent ?? "");
    // 陽性対照: 同じ走査が、文言を持つ要素を（重複も含めて）見つけ、持たない要素は数えない。
    const control = render(
      <div>
        <p>a</p>
        <p>a</p>
        <div>
          <span>b</span>
        </div>
      </div>,
    );
    expect(ownText(control.container).sort()).toEqual(["a", "a", "b"]);
    control.unmount();
    // 陽性対照: 根の直下の裸のテキストを、根そのものとして拾う。
    const bare = render(
      <>
        c<p>a</p>
      </>,
    );
    expect(ownText(bare.container).sort()).toEqual(["a", "ca"]);
    bare.unmount();

    const { container } = render(<HelpView />);
    const expected = [
      H1,
      ...SECTIONS.map((s) => s.heading),
      "下書き",
      "締め済",
      "承認済",
      ...ALL_LINES.map(([, text]) => text),
    ];
    expect(expected).toHaveLength(25);
    expect(ownText(container).sort()).toEqual([...expected].sort());
  });

  it("4-2: 3-1-d の行に「確定」が無い", () => {
    render(<HelpView />);
    // 3-1-d の行は、承認済のバッジと 3-1-d の文言が並ぶ組。組の DOM を読み、期待値から結果が決まらないようにする。
    const group = oneLine("承認済").parentElement!;
    expect(group.textContent).toBe("承認済" + SECTIONS[0].lines["3-1-d"]);
    expect(group.textContent).not.toContain("確定");
  });

  it("4-3: 本文に「目安」が無い", () => {
    render(<HelpView />);
    expect(bodyText()).not.toContain("目安");
  });

  it.each(["確定した月", "入力中", "精算幅に収まった月", "契約期間", "請求", "精算金額"])(
    "4-4: 本文に「%s」が無い",
    (word) => {
      render(<HelpView />);
      expect(bodyText()).not.toContain(word);
    },
  );
});

describe("状態を変えない・取得しない（AC-5）", () => {
  const OPERATIONS =
    'button, input, select, textarea, form, [role="button"], [role="checkbox"], [role="radio"], [role="combobox"], [role="switch"]';

  it("5-1: ボタン・入力欄・選択欄・チェックボックス・ラジオ・フォームが無い", () => {
    // 陽性対照: 同じ走査が、操作要素を実際に見つける。
    const control = render(
      <div>
        <form>
          <button type="button">b</button>
          <input type="checkbox" />
          <input type="radio" />
          <input type="text" />
          <select />
        </form>
      </div>,
    );
    expect(control.container.querySelectorAll(OPERATIONS).length).toBeGreaterThanOrEqual(6);
    control.unmount();

    const { container } = render(<HelpView />);
    expect(container.textContent).toContain("精算幅");
    expect(container.querySelectorAll(OPERATIONS)).toHaveLength(0);
  });

  it("5-2: 描画しても fetch は0回", async () => {
    const fake = installFakeFetch({});
    render(<HelpView />);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fake.urls).toHaveLength(0);
  });

  it("5-3: role=alert の要素が無い", () => {
    // 陽性対照: 同じ走査が role=alert を見つける。
    const control = render(<p role="alert">e</p>);
    expect(control.container.querySelectorAll('[role="alert"]')).toHaveLength(1);
    control.unmount();

    const { container } = render(<HelpView />);
    expect(container.textContent).toContain("精算幅");
    expect(container.querySelectorAll('[role="alert"]')).toHaveLength(0);
  });
});
