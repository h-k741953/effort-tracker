/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { thisMonthEntry, type ThisMonthEntry } from "@/lib/this-month-entry";
import { thisMonthPath } from "@/lib/screen-routes";
import { CONTRACT_A, CONTRACT_B, CONTRACT_C } from "@/test-support/work-month-list";
import { InitialEntryAction } from "./initial-entry-action";

// docs/specs/work-month-listing-ui.md AC-6-2・7-1・10-9-b・10-9-c。設計 W-4・W-8・T-2。
// props は entry（ThisMonthEntry）だけ。entry は thisMonthEntry（契約の配列から作る）の結果を渡す。
// 開閉は部品の内側の状態で、通信しない。

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const LABEL = "稼働実績を入力する";

describe("hidden（6-2・7-1。契約が0件）", () => {
  it("何も出さない（container.innerHTML が空）", () => {
    const { container } = render(<InitialEntryAction entry={thisMonthEntry([])} />);
    expect(container.innerHTML).toBe("");
  });
});

describe("link（7-1・10-9-a。契約が1件）", () => {
  it("リンク「稼働実績を入力する」がその契約の thisMonthPath を指す", () => {
    render(<InitialEntryAction entry={thisMonthEntry([CONTRACT_A])} />);
    const link = screen.getByRole("link", { name: LABEL });
    expect(link.getAttribute("href")).toBe(thisMonthPath(CONTRACT_A.contractId));
    expect(screen.queryByRole("button")).toBeNull();
  });
});

describe("choice（10-9-b・10-9-c。契約が2件以上）", () => {
  const contracts = [CONTRACT_C, CONTRACT_A, CONTRACT_B]; // 入力の順

  function entry(): ThisMonthEntry {
    return thisMonthEntry(contracts);
  }

  it("button「稼働実績を入力する」が aria-expanded=false で出て、押す前に選択肢のリンクが文書に無い", () => {
    render(<InitialEntryAction entry={entry()} />);
    const button = screen.getByRole("button", { name: LABEL });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("link", { name: LABEL })).toBeNull();
  });

  it("押すと aria-expanded=true になり、契約表示名の link が契約の数だけ入力の順で、button より後に現れ、それぞれの thisMonthPath を指す", () => {
    const { container } = render(<InitialEntryAction entry={entry()} />);
    const button = screen.getByRole("button", { name: LABEL });
    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");

    const links = screen.getAllByRole("link");
    expect(links.map((l) => l.textContent)).toEqual(contracts.map((c) => c.contractDisplayName));
    contracts.forEach((c, i) => {
      expect(within(container).getByRole("link", { name: c.contractDisplayName }).getAttribute("href")).toBe(
        thisMonthPath(c.contractId),
      );
      expect(links[i].getAttribute("href")).toBe(thisMonthPath(c.contractId));
      // 文書の順で button より後。
      expect(button.compareDocumentPosition(links[i]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });
  });

  it("再び押すと aria-expanded=false に戻り、選択肢のリンクが文書から消える", () => {
    render(<InitialEntryAction entry={entry()} />);
    const button = screen.getByRole("button", { name: LABEL });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("開閉で fetch を呼ばない", () => {
    const calls: unknown[] = [];
    vi.stubGlobal("fetch", (...args: unknown[]) => {
      calls.push(args);
      return Promise.reject(new Error("fetch must not be called"));
    });
    render(<InitialEntryAction entry={entry()} />);
    const button = screen.getByRole("button", { name: LABEL });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(calls).toHaveLength(0);
  });
});
