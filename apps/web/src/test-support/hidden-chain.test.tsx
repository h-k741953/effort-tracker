/** @vitest-environment jsdom */
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { hiddenChain } from "./hidden-chain";

// 陽性対照: hiddenChain は、表を隠す形ごとに、その形を検出する。恒空へ潰すと「隠れていない」の主張が空虚に真になる。
afterEach(cleanup);

const T = (
  <table>
    <tbody>
      <tr>
        <td>x</td>
      </tr>
    </tbody>
  </table>
);

describe("hiddenChain の陽性対照（表を隠す形ごと）", () => {
  it.each<[string, React.ReactElement, string]>([
    ["sr-only の祖先", <div key="k1" className="sr-only">{T}</div>, "sr-only"],
    ["hidden 属性の祖先", <div key="k2" hidden>{T}</div>, "hidden-attr"],
    ["aria-hidden の祖先", <div key="k3" aria-hidden="true">{T}</div>, "aria-hidden"],
    ["遠い祖先の sr-only", <section key="k4" className="sr-only"><div><div>{T}</div></div></section>, "sr-only"],
    ["クラス hidden の祖先", <div key="k5" className="hidden">{T}</div>, "hidden"],
    ["クラス invisible の祖先", <div key="k6" className="invisible h-0 overflow-hidden">{T}</div>, "invisible"],
    ["h-0 の祖先", <div key="k7" className="h-0 overflow-hidden">{T}</div>, "h-0"],
    ["クラス opacity-0 の祖先", <div key="k8" className="opacity-0">{T}</div>, "opacity-0"],
    ["クラス collapse の祖先", <div key="k9" className="collapse">{T}</div>, "collapse"],
    ["接頭辞つき md:hidden の祖先", <div key="k10" className="md:hidden">{T}</div>, "hidden"],
    ["style display:none の祖先", <div key="k11" style={{ display: "none" }}>{T}</div>, "display:none"],
    ["style visibility:hidden の祖先", <div key="k12" style={{ visibility: "hidden" }}>{T}</div>, "visibility:hidden"],
    ["style opacity:0 の祖先", <div key="k13" style={{ opacity: 0 }}>{T}</div>, "opacity:0"],
  ])("%s を検出する", (_name, element, kind) => {
    const { container } = render(<div>{element}</div>);
    expect(hiddenChain(container.querySelector("table")!)).toContain(kind);
  });

  it("表自身の sr-only・hidden も検出する", () => {
    const { container } = render(
      <div>
        <table className="sr-only"><tbody><tr><td>x</td></tr></tbody></table>
      </div>,
    );
    expect(hiddenChain(container.querySelector("table")!)).toContain("sr-only");
  });

  it("陰性対照: overflow-hidden・通常のクラス・style は隠す形に数えない", () => {
    const { container } = render(
      <div className="overflow-hidden flex gap-3" style={{ display: "block", opacity: 1 }}>
        {T}
      </div>,
    );
    expect(hiddenChain(container.querySelector("table")!)).toEqual([]);
  });
});
