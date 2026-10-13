/* @ds-bundle: {"namespace":"EffortTrackerDS","components":[{"name":"AppHeader","sourcePath":"components/general/AppHeader/AppHeader.jsx"},{"name":"Button","sourcePath":"components/general/Button/Button.jsx"},{"name":"Card","sourcePath":"components/general/Card/Card.jsx"},{"name":"ConfirmDialog","sourcePath":"components/general/ConfirmDialog/ConfirmDialog.jsx"},{"name":"EmptyState","sourcePath":"components/general/EmptyState/EmptyState.jsx"},{"name":"ErrorBanner","sourcePath":"components/general/ErrorBanner/ErrorBanner.jsx"},{"name":"FieldError","sourcePath":"components/general/FieldError/FieldError.jsx"},{"name":"NumberField","sourcePath":"components/general/NumberField/NumberField.jsx"},{"name":"Pagination","sourcePath":"components/general/Pagination/Pagination.jsx"},{"name":"RoleSwitcher","sourcePath":"components/general/RoleSwitcher/RoleSwitcher.jsx"},{"name":"StatusBadge","sourcePath":"components/general/StatusBadge/StatusBadge.jsx"}],"sourceHashes":{"components/general/AppHeader/AppHeader.jsx":"731281fb7283","components/general/AppHeader/AppHeader.d.ts":"d87d4815b543","components/general/AppHeader/AppHeader.prompt.md":"e9ab30c8f422","components/general/Button/Button.jsx":"8d36eeb299b8","components/general/Button/Button.d.ts":"307a5cb53faf","components/general/Button/Button.prompt.md":"d39c0c1641c7","components/general/Card/Card.jsx":"e78acb4f5851","components/general/Card/Card.d.ts":"de8f5cd3faa0","components/general/Card/Card.prompt.md":"b556cd33f71b","components/general/ConfirmDialog/ConfirmDialog.jsx":"ce2423489093","components/general/ConfirmDialog/ConfirmDialog.d.ts":"9438c06ccbef","components/general/ConfirmDialog/ConfirmDialog.prompt.md":"7617c9dc9487","components/general/EmptyState/EmptyState.jsx":"e99cecc7c406","components/general/EmptyState/EmptyState.d.ts":"eea8ab0dcd0f","components/general/EmptyState/EmptyState.prompt.md":"b4703ba8aa4e","components/general/ErrorBanner/ErrorBanner.jsx":"88e180cbf150","components/general/ErrorBanner/ErrorBanner.d.ts":"42e62509a1b2","components/general/ErrorBanner/ErrorBanner.prompt.md":"d0f8ec78c026","components/general/FieldError/FieldError.jsx":"ddda1ea4c733","components/general/FieldError/FieldError.d.ts":"478ae3527fcb","components/general/FieldError/FieldError.prompt.md":"ab28a4aae3fd","components/general/NumberField/NumberField.jsx":"8a1ced343b44","components/general/NumberField/NumberField.d.ts":"19ef27e35310","components/general/NumberField/NumberField.prompt.md":"1dad7fa56343","components/general/Pagination/Pagination.jsx":"13604160529f","components/general/Pagination/Pagination.d.ts":"db14b0bb9ed6","components/general/Pagination/Pagination.prompt.md":"04366a51a80b","components/general/RoleSwitcher/RoleSwitcher.jsx":"98e79a50fa7a","components/general/RoleSwitcher/RoleSwitcher.d.ts":"1fe07b1e1bc2","components/general/RoleSwitcher/RoleSwitcher.prompt.md":"b79a218cfe99","components/general/StatusBadge/StatusBadge.jsx":"fbfb9dc8bea4","components/general/StatusBadge/StatusBadge.d.ts":"576360d9ca8e","components/general/StatusBadge/StatusBadge.prompt.md":"d926a6b157f7"},"inlinedExternals":[],"builtBy":"cc-design-sync"} */
var EffortTrackerDS = (() => {
  var __create = Object.create;
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __getProtoOf = Object.getPrototypeOf;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __esm = (fn, res, err) => function __init() {
    if (err) throw err[0];
    try {
      return fn && (res = (0, fn[__getOwnPropNames(fn)[0]])(fn = 0)), res;
    } catch (e) {
      throw err = [e], e;
    }
  };
  var __commonJS = (cb, mod) => function __require() {
    try {
      return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
    } catch (e) {
      throw mod = 0, e;
    }
  };
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
    // If the importer is in node compatibility mode or this is not an ESM
    // file that has been converted to a CommonJS file using a Babel-
    // compatible transform (i.e. "__esModule" has not been set), then set
    // "default" to the CommonJS "module.exports" for node compatibility.
    isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
    mod
  ));
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

  // <define:import.meta.env>
  var init_define_import_meta_env = __esm({
    "<define:import.meta.env>"() {
    }
  });

  // shim:react-shim
  var require_react_shim = __commonJS({
    "shim:react-shim"(exports, module) {
      init_define_import_meta_env();
      var R = window.React;
      function np(p, k) {
        var o = {};
        for (var x in p) if (x !== "children") o[x] = p[x];
        if (k !== void 0) o.key = k;
        return o;
      }
      function jsx12(t, p, k) {
        var c = p && p.children;
        return c === void 0 ? R.createElement(t, np(p, k)) : R.createElement(t, np(p, k), c);
      }
      function jsxs9(t, p, k) {
        return R.createElement.apply(R, [t, np(p, k)].concat(p.children));
      }
      module.exports = R;
      module.exports.jsx = jsx12;
      module.exports.jsxs = jsxs9;
      module.exports.jsxDEV = function(t, p, k, s) {
        return (s ? jsxs9 : jsx12)(t, p, k);
      };
      module.exports.Fragment = R.Fragment;
    }
  });

  // ds-bundle/.pkg-entry.mjs
  var pkg_entry_exports = {};
  __export(pkg_entry_exports, {
    AppHeader: () => AppHeader,
    Button: () => Button,
    Card: () => Card,
    ConfirmDialog: () => ConfirmDialog,
    EmptyState: () => EmptyState,
    ErrorBanner: () => ErrorBanner,
    FieldError: () => FieldError,
    NumberField: () => NumberField,
    Pagination: () => Pagination,
    RoleSwitcher: () => RoleSwitcher,
    StatusBadge: () => StatusBadge
  });
  init_define_import_meta_env();

  // apps/web/src/components/app-header.tsx
  init_define_import_meta_env();
  var import_jsx_runtime = __toESM(require_react_shim());
  function AppHeader({ right }) {
    return /* @__PURE__ */ (0, import_jsx_runtime.jsxs)("header", { className: "flex items-center justify-between border-b border-border bg-background px-4 py-3 text-foreground", children: [
      /* @__PURE__ */ (0, import_jsx_runtime.jsx)("h1", { className: "text-lg font-semibold", children: "effort-tracker" }),
      right !== void 0 ? /* @__PURE__ */ (0, import_jsx_runtime.jsx)("div", { children: right }) : null
    ] });
  }

  // apps/web/src/components/button.tsx
  init_define_import_meta_env();
  var import_jsx_runtime2 = __toESM(require_react_shim());
  var VARIANT_CLASSES = {
    primary: "bg-primary text-primary-foreground",
    secondary: "bg-surface text-surface-foreground border border-border",
    danger: "bg-danger text-danger-foreground"
  };
  function Button({ variant = "secondary", type = "button", className, ...rest }) {
    const classes = [
      "inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium",
      "focus-visible:ring-2 focus-visible:ring-focus-ring",
      VARIANT_CLASSES[variant],
      className
    ].filter(Boolean).join(" ");
    return /* @__PURE__ */ (0, import_jsx_runtime2.jsx)("button", { type, className: classes, ...rest });
  }

  // apps/web/src/components/card.tsx
  init_define_import_meta_env();
  var import_jsx_runtime3 = __toESM(require_react_shim());
  function Card({ children, title }) {
    return /* @__PURE__ */ (0, import_jsx_runtime3.jsxs)("div", { className: "rounded-md border border-border bg-surface p-4 text-surface-foreground", children: [
      title !== void 0 ? /* @__PURE__ */ (0, import_jsx_runtime3.jsx)("h2", { className: "mb-2 text-base font-semibold", children: title }) : null,
      children
    ] });
  }

  // apps/web/src/components/confirm-dialog.tsx
  init_define_import_meta_env();
  var import_react = __toESM(require_react_shim());
  var import_jsx_runtime4 = __toESM(require_react_shim());
  function ConfirmDialog({
    open,
    title,
    description,
    confirmLabel,
    confirmVariant = "primary",
    onConfirm,
    onCancel
  }) {
    const titleId = (0, import_react.useId)();
    const dialogRef = (0, import_react.useRef)(null);
    (0, import_react.useEffect)(() => {
      if (open) {
        dialogRef.current?.focus();
      }
    }, [open]);
    if (!open) {
      return null;
    }
    const handleKeyDown = (event) => {
      if (event.key === "Escape") {
        onCancel();
      }
    };
    return /* @__PURE__ */ (0, import_jsx_runtime4.jsxs)(
      "div",
      {
        ref: dialogRef,
        role: "dialog",
        "aria-modal": "true",
        "aria-labelledby": titleId,
        tabIndex: -1,
        onKeyDown: handleKeyDown,
        className: "rounded-md border border-border bg-surface p-4 text-surface-foreground outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
        children: [
          /* @__PURE__ */ (0, import_jsx_runtime4.jsx)("h2", { id: titleId, className: "text-base font-semibold", children: title }),
          /* @__PURE__ */ (0, import_jsx_runtime4.jsx)("div", { children: description }),
          /* @__PURE__ */ (0, import_jsx_runtime4.jsxs)("div", { className: "mt-4 flex justify-end gap-2", children: [
            /* @__PURE__ */ (0, import_jsx_runtime4.jsx)(Button, { variant: "secondary", onClick: onCancel, children: "\u53D6\u6D88" }),
            /* @__PURE__ */ (0, import_jsx_runtime4.jsx)(Button, { variant: confirmVariant, onClick: onConfirm, children: confirmLabel })
          ] })
        ]
      }
    );
  }

  // apps/web/src/components/empty-state.tsx
  init_define_import_meta_env();
  var import_jsx_runtime5 = __toESM(require_react_shim());
  function EmptyState({ title, description, action }) {
    return /* @__PURE__ */ (0, import_jsx_runtime5.jsxs)("div", { className: "flex flex-col items-center gap-2 rounded-md border border-border p-6 text-center", children: [
      /* @__PURE__ */ (0, import_jsx_runtime5.jsx)("p", { className: "text-base font-medium text-foreground", children: title }),
      description !== void 0 ? /* @__PURE__ */ (0, import_jsx_runtime5.jsx)("p", { className: "text-sm text-muted-foreground", children: description }) : null,
      action !== void 0 ? action : null
    ] });
  }

  // apps/web/src/components/error-banner.tsx
  init_define_import_meta_env();
  var import_jsx_runtime6 = __toESM(require_react_shim());
  function ErrorBanner({ children, onRetry }) {
    return /* @__PURE__ */ (0, import_jsx_runtime6.jsxs)("div", { role: "alert", className: "rounded-md border border-danger bg-background p-3 text-danger", children: [
      /* @__PURE__ */ (0, import_jsx_runtime6.jsx)("p", { className: "font-semibold", children: "\u30A8\u30E9\u30FC" }),
      /* @__PURE__ */ (0, import_jsx_runtime6.jsx)("p", { children }),
      onRetry !== void 0 ? /* @__PURE__ */ (0, import_jsx_runtime6.jsx)(Button, { variant: "danger", onClick: onRetry, children: "\u518D\u8A66\u884C" }) : null
    ] });
  }

  // apps/web/src/components/field-error.tsx
  init_define_import_meta_env();
  var import_jsx_runtime7 = __toESM(require_react_shim());
  function FieldError({ id, children }) {
    return /* @__PURE__ */ (0, import_jsx_runtime7.jsx)("p", { id, role: "alert", className: "text-sm text-danger", children });
  }

  // apps/web/src/components/number-field.tsx
  init_define_import_meta_env();
  var import_jsx_runtime8 = __toESM(require_react_shim());
  function NumberField({
    id,
    label,
    value,
    onChange,
    min,
    max,
    step,
    errorId,
    invalid
  }) {
    const handleChange = (event) => {
      const raw = event.target.value;
      onChange(raw === "" ? "" : Number(raw));
    };
    return /* @__PURE__ */ (0, import_jsx_runtime8.jsxs)("div", { className: "flex flex-col gap-1", children: [
      /* @__PURE__ */ (0, import_jsx_runtime8.jsx)("label", { htmlFor: id, className: "text-sm font-medium text-foreground", children: label }),
      /* @__PURE__ */ (0, import_jsx_runtime8.jsx)(
        "input",
        {
          id,
          type: "number",
          value,
          onChange: handleChange,
          min,
          max,
          step,
          "aria-invalid": invalid ? "true" : void 0,
          "aria-describedby": errorId,
          className: "rounded-md border border-border bg-background px-2 py-1 text-foreground focus-visible:ring-2 focus-visible:ring-focus-ring"
        }
      )
    ] });
  }

  // apps/web/src/components/pagination.tsx
  init_define_import_meta_env();
  var import_jsx_runtime9 = __toESM(require_react_shim());
  function Pagination({ page, pageCount, onPageChange }) {
    return /* @__PURE__ */ (0, import_jsx_runtime9.jsxs)("nav", { "aria-label": "\u30DA\u30FC\u30B8\u9001\u308A", children: [
      /* @__PURE__ */ (0, import_jsx_runtime9.jsx)(
        "button",
        {
          type: "button",
          disabled: page <= 1,
          onClick: () => onPageChange(page - 1),
          className: "rounded-md px-2 py-1 focus-visible:ring-2 focus-visible:ring-focus-ring",
          children: "\u524D\u3078"
        }
      ),
      /* @__PURE__ */ (0, import_jsx_runtime9.jsx)(
        "button",
        {
          type: "button",
          disabled: page >= pageCount,
          onClick: () => onPageChange(page + 1),
          className: "rounded-md px-2 py-1 focus-visible:ring-2 focus-visible:ring-focus-ring",
          children: "\u6B21\u3078"
        }
      )
    ] });
  }

  // apps/web/src/components/role-switcher.tsx
  init_define_import_meta_env();
  var import_react2 = __toESM(require_react_shim());
  var import_jsx_runtime10 = __toESM(require_react_shim());
  var OPTIONS = [
    { value: "Engineer", label: "\u6280\u8853\u8005" },
    { value: "Approver", label: "\u627F\u8A8D\u8005" }
  ];
  function RoleSwitcher({ role, onChange }) {
    const name = (0, import_react2.useId)();
    const handleChange = (event) => {
      onChange(event.target.value);
    };
    return /* @__PURE__ */ (0, import_jsx_runtime10.jsx)("div", { role: "radiogroup", "aria-label": "\u30ED\u30FC\u30EB\u5207\u66FF", className: "flex items-center gap-3", children: OPTIONS.map((option) => /* @__PURE__ */ (0, import_jsx_runtime10.jsxs)("label", { className: "flex items-center gap-1 text-sm text-foreground", children: [
      /* @__PURE__ */ (0, import_jsx_runtime10.jsx)(
        "input",
        {
          type: "radio",
          name,
          value: option.value,
          checked: role === option.value,
          onChange: handleChange,
          className: "focus-visible:ring-2 focus-visible:ring-focus-ring"
        }
      ),
      option.label
    ] }, option.value)) });
  }

  // apps/web/src/components/status-badge.tsx
  init_define_import_meta_env();
  var import_jsx_runtime11 = __toESM(require_react_shim());
  var LABELS = {
    Draft: "\u4E0B\u66F8\u304D",
    PendingApproval: "\u7DE0\u3081\u6E08",
    Approved: "\u627F\u8A8D\u6E08"
  };
  var TOKEN_CLASSES = {
    Draft: "bg-state-draft text-state-draft-foreground",
    PendingApproval: "bg-state-pending-approval text-state-pending-approval-foreground",
    Approved: "bg-state-approved text-state-approved-foreground"
  };
  function StatusBadge({ state }) {
    return /* @__PURE__ */ (0, import_jsx_runtime11.jsx)("span", { className: `inline-flex items-center rounded-md px-2 py-1 text-sm ${TOKEN_CLASSES[state]}`, children: LABELS[state] });
  }
  return __toCommonJS(pkg_entry_exports);
})();
window.EffortTrackerDS=EffortTrackerDS.__dsMainNs?Object.assign({},EffortTrackerDS,EffortTrackerDS.__dsMainNs,{__dsMainNs:undefined}):EffortTrackerDS;
