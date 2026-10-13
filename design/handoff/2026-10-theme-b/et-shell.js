// effort-tracker 共通: テーマ（ライト／ダーク）の保存と、ヘッダー右側の切替部品
(function () {
  var KEY = 'et-theme';
  function getTheme() { try { var v = localStorage.getItem(KEY); return v === 'dark' ? 'dark' : 'light'; } catch (e) { return 'light'; } }
  function saveTheme(t) { try { localStorage.setItem(KEY, t); } catch (e) {} }
  function svg(paths) {
    var h = window.React.createElement;
    return h('svg', { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round', strokeLinejoin: 'round', 'aria-hidden': 'true' },
      paths.map(function (d, i) { return d.c ? h('circle', { key: i, cx: d.c[0], cy: d.c[1], r: d.c[2] }) : h('path', { key: i, d: d }); }));
  }
  var SUN = [{ c: [12, 12, 4] }, 'M12 2v2', 'M12 20v2', 'M4.93 4.93l1.41 1.41', 'M17.66 17.66l1.41 1.41', 'M2 12h2', 'M20 12h2', 'M4.93 19.07l1.41-1.41', 'M17.66 6.34l1.41-1.41'];
  var MOON = ['M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z'];
  function themeToggle(theme, onTheme) {
    var h = window.React.createElement;
    function seg(v, label, icon) {
      return h('button', { key: v, type: 'button', 'data-seg': '', 'aria-pressed': theme === v, onClick: function () { onTheme(v); } }, svg(icon), label);
    }
    return h('div', { role: 'group', 'aria-label': 'テーマ', 'data-segwrap': '' }, seg('light', 'ライト', SUN), seg('dark', 'ダーク', MOON));
  }
  // opts: { theme, onTheme, role?, onRole?, extra? }
  function headerRight(opts) {
    var h = window.React.createElement; var DS = window.EffortTrackerDS;
    var kids = [];
    if (opts.role && DS) kids.push(h(DS.RoleSwitcher, { key: 'role', role: opts.role, onChange: opts.onRole }));
    kids.push(h(window.React.Fragment, { key: 'theme' }, themeToggle(opts.theme, opts.onTheme)));
    if (opts.extra) kids.push(h(window.React.Fragment, { key: 'extra' }, opts.extra));
    return h('div', { style: { display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap', justifyContent: 'flex-end' } }, kids);
  }
  window.ETShell = { getTheme: getTheme, saveTheme: saveTheme, headerRight: headerRight };
})();
