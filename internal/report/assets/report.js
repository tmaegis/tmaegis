(function () {
  "use strict";
  var root = document.documentElement;
  var $ = function (s, c) { return (c || document).querySelector(s); };
  var $$ = function (s, c) { return Array.prototype.slice.call((c || document).querySelectorAll(s)); };
  var store = {
    get: function (k) { try { return localStorage.getItem(k); } catch (e) { return null; } },
    set: function (k, v) { try { localStorage.setItem(k, v); } catch (e) {} },
    del: function (k) { try { localStorage.removeItem(k); } catch (e) {} }
  };

  /*PLURAL-BEGIN*/
  function pluralCategory(tag, n) {
    if (n < 0) n = -n;
    switch (tag) {
      case "ja": case "ko": case "zh-Hans": case "zh-Hant": case "vi": case "id":
        return "other";
      case "fr": case "pt-BR":
        return (n === 0 || n === 1) ? "one" : "other";
      case "ru": case "sr-Latn": case "sr-Cyrl": {
        var m10 = n % 10, m100 = n % 100;
        if (m10 === 1 && m100 !== 11) return "one";
        if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return "few";
        if (tag === "ru") return "many";
        return "other";
      }
    }
    return n === 1 ? "one" : "other";
  }
  /*PLURAL-END*/

  // ---- i18n ------------------------------------------------------------------------------
  var DICT = { default: "en-US", tags: [], names: {}, m: { "en-US": {} } };
  try { DICT = JSON.parse($("#i18n-data").textContent); } catch (e) {}
  var lang = root.getAttribute("lang") || DICT.default;
  var titleHost = document.title.indexOf(" - ") > -1 ? document.title.split(" - ").slice(1).join(" - ") : "";

  function intlTag() { try { Intl.getCanonicalLocales(lang); return lang; } catch (e) { return "en-US"; } }
  function nf(opts) { try { return new Intl.NumberFormat(intlTag(), opts); } catch (e) { return new Intl.NumberFormat("en-US", opts); } }
  function lookup(k) {
    var d = DICT.m[lang];
    if (d && d[k] !== undefined) return d[k];
    var b = DICT.m[DICT["default"]];
    return b && b[k] !== undefined ? b[k] : k;
  }
  function fill(s, args) {
    if (!args || s.indexOf("{") < 0) return s;
    return s.replace(/\{(\w+)\}/g, function (m, k) {
      if (!(k in args)) return m;
      var v = args[k];
      return typeof v === "number" ? nf().format(v) : String(v);
    });
  }
  function t(k, args) { return fill(lookup(k), args); }
  function tn(k, n, args) {
    var a = {};
    for (var p in args) a[p] = args[p];
    a.n = n;
    var cat = pluralCategory(lang, n);
    var keys = [k + "." + cat, k + ".other", k];
    for (var i = 0; i < keys.length; i++) {
      var d = DICT.m[lang], b = DICT.m[DICT["default"]];
      if (d && d[keys[i]] !== undefined) return fill(d[keys[i]], a);
      if (b && b[keys[i]] !== undefined) return fill(b[keys[i]], a);
    }
    return k;
  }
  function elArgs(el) {
    try { return el.hasAttribute("data-args") ? JSON.parse(el.getAttribute("data-args")) : null; } catch (e) { return null; }
  }
  var dynamic = []; // callbacks that rebuild JS-generated text on a language change

  function applyNumbers() {
    $$("[data-num]").forEach(function (el) {
      var v = parseFloat(el.getAttribute("data-num"));
      if (isNaN(v)) return;
      var dec = parseInt(el.getAttribute("data-dec") || "0", 10);
      var pct = el.getAttribute("data-suffix") === "%";
      var o = { minimumFractionDigits: dec, maximumFractionDigits: dec };
      if (pct) { o.style = "percent"; v = v / 100; }
      el.textContent = nf(o).format(v);
    });
    $$("time[data-ts]").forEach(function (el) {
      var d = new Date(el.getAttribute("data-ts"));
      if (isNaN(d)) return;
      try { el.textContent = new Intl.DateTimeFormat(intlTag(), { dateStyle: "medium", timeStyle: "short" }).format(d); } catch (e) {}
    });
    var g = $(".gauge[data-i18n-aria-label]"), gv = g && $("[data-num]", g);
    if (g && gv) g.setAttribute("aria-label", t("report.score.aria", { score: gv.textContent }));
  }

  function applyLang(tag) {
    lang = tag;
    root.setAttribute("lang", tag);
    $$("[data-i18n]").forEach(function (el) {
      var k = el.getAttribute("data-i18n"), a = elArgs(el);
      el.textContent = el.hasAttribute("data-n") ? tn(k, parseInt(el.getAttribute("data-n"), 10), a) : t(k, a);
    });
    ["aria-label", "placeholder", "title"].forEach(function (at) {
      $$("[data-i18n-" + at + "]").forEach(function (el) {
        el.setAttribute(at, t(el.getAttribute("data-i18n-" + at), elArgs(el)));
      });
    });
    applyNumbers();
    var nm = DICT.names && DICT.names[tag];
    var al = $("#about-lang");
    if (al) al.textContent = nm ? nm + " (" + tag + ")" : tag;
    document.title = t("report.app.title") + (titleHost ? " - " + titleHost : "");
    dynamic.forEach(function (fn) { fn(); });
  }

  var langSel = $("#lang");
  if (langSel) langSel.addEventListener("change", function () { applyLang(langSel.value); });

  // ---- theme / print ---------------------------------------------------------------------
  var saved = store.get("tmaegis-theme");
  if (saved === "dark" || saved === "light") root.setAttribute("data-theme", saved);
  var themeBtn = $("#theme");
  if (themeBtn) themeBtn.addEventListener("click", function () {
    var dark = root.getAttribute("data-theme") === "dark" ||
      (!root.getAttribute("data-theme") && window.matchMedia && matchMedia("(prefers-color-scheme: dark)").matches);
    var next = dark ? "light" : "dark";
    root.setAttribute("data-theme", next);
    store.set("tmaegis-theme", next);
  });
  var printBtn = $("#print");
  if (printBtn) printBtn.addEventListener("click", function () { window.print(); });

  var fg = $(".gauge .fg");
  if (fg) {
    var target = fg.getAttribute("data-offset");
    fg.style.strokeDashoffset = fg.getAttribute("data-full");
    requestAnimationFrame(function () { requestAnimationFrame(function () { fg.style.strokeDashoffset = target; }); });
  }

  // ---- tabs ------------------------------------------------------------------------------
  var tabs = $$('[role="tab"]');
  var names = tabs.map(function (b) { return b.getAttribute("data-tab"); });
  var current = null;

  function showTab(name, o) {
    o = o || {};
    if (names.indexOf(name) < 0) name = "overview";
    tabs.forEach(function (b) {
      var on = b.getAttribute("data-tab") === name;
      b.setAttribute("aria-selected", on ? "true" : "false");
      b.setAttribute("tabindex", on ? "0" : "-1");
      if (on && o.focus) b.focus();
    });
    names.forEach(function (n) {
      var p = document.getElementById("p-" + n);
      if (p) { if (n === name) p.removeAttribute("hidden"); else p.setAttribute("hidden", ""); }
    });
    if (current !== name) {
      current = name;
      store.set("tmaegis-tab", name);
      if (o.hash !== false && location.hash !== "#" + name) {
        try { history.pushState(null, "", "#" + name); } catch (e) { try { location.hash = name; } catch (e2) {} }
      }
    }
  }
  tabs.forEach(function (b, i) {
    b.addEventListener("click", function () { showTab(b.getAttribute("data-tab")); });
    b.addEventListener("keydown", function (e) {
      var j = -1;
      if (e.key === "ArrowRight") j = (i + 1) % tabs.length;
      else if (e.key === "ArrowLeft") j = (i - 1 + tabs.length) % tabs.length;
      else if (e.key === "Home") j = 0;
      else if (e.key === "End") j = tabs.length - 1;
      if (j >= 0) { e.preventDefault(); showTab(names[j], { focus: true }); }
    });
  });
  $$("[data-tab-link]").forEach(function (a) {
    a.addEventListener("click", function (e) { e.preventDefault(); showTab(a.getAttribute("data-tab-link")); window.scrollTo(0, 0); });
  });

  // ---- results table ---------------------------------------------------------------------
  var tbody = $("#rows");
  var pairs = tbody ? $$("tr.rule", tbody).map(function (r) { return { rule: r, detail: r.nextElementSibling }; }) : [];
  var q = $("#q"), sev = $("#sev"), tgt = $("#tgt"), bm = $("#bm"), grp = $("#grp"), count = $("#count"), none = $("#none"), clearBtn = $("#clear");
  var chips = $$(".chip[data-status]");
  var active = {};
  var sortKey = null, dir = 1;
  var rank = { critical: 0, high: 1, medium: 2, low: 3 };
  var srank = { FAIL: 0, ERROR: 0, SKIP: 1, PASS: 2 };

  function sevLabel(s) { return t("report.sev." + (rank[s] === undefined ? "other" : s)); }
  function val(p, k) {
    var d = p.rule.dataset;
    if (k === "sev") return rank[d.sev] === undefined ? 4 : rank[d.sev];
    if (k === "status") return srank[d.status] === undefined ? 3 : srank[d.status];
    if (k === "level") return +d.level;
    if (k === "n") return +d.n;
    return (k === "id" ? d.id : d.title).toLowerCase();
  }
  function groupOf(p) {
    var g = grp ? grp.value : "";
    var d = p.rule.dataset;
    if (g === "bench") return { k: d.benchname.toLowerCase(), label: d.benchname };
    if (g === "target") return { k: (d.target || "~").toLowerCase(), label: d.target || "-" };
    if (g === "sev") return { k: String(val(p, "sev")), label: sevLabel(d.sev) };
    return null;
  }
  function cmp(a, b) {
    var ga = groupOf(a), gb = groupOf(b);
    if (ga && ga.k !== gb.k) return ga.k < gb.k ? -1 : 1;
    if (sortKey === null) { // default: failures first, then severity, then rule order
      return (val(a, "status") - val(b, "status")) || (val(a, "sev") - val(b, "sev")) || (val(a, "n") - val(b, "n"));
    }
    var x = val(a, sortKey), y = val(b, sortKey);
    return (x < y ? -1 : x > y ? 1 : val(a, "n") - val(b, "n")) * dir;
  }
  function render() {
    if (!tbody) return;
    $$("tr.grp", tbody).forEach(function (r) { r.parentNode.removeChild(r); });
    pairs.sort(cmp);
    var last = null;
    pairs.forEach(function (p) {
      var g = groupOf(p);
      if (g && g.k !== last) {
        var tr = document.createElement("tr");
        tr.className = "grp";
        var th = document.createElement("th");
        th.setAttribute("colspan", "6");
        th.setAttribute("scope", "colgroup");
        var name = document.createElement("span");
        name.textContent = g.label;
        var c = document.createElement("span");
        c.className = "gn";
        th.appendChild(name); th.appendChild(c); tr.appendChild(th);
        tbody.appendChild(tr);
        last = g.k;
      }
      tbody.appendChild(p.rule);
      tbody.appendChild(p.detail);
    });
    apply();
  }

  function filtersOn() {
    return !!((q.value || "").trim() || sev.value || tgt.value || bm.value || Object.keys(active).length);
  }
  function apply() {
    if (!tbody) return;
    var text = (q.value || "").trim().toLowerCase();
    var anyStatus = Object.keys(active).length > 0;
    var shown = 0;
    pairs.forEach(function (p) {
      var d = p.rule.dataset;
      var st = d.status === "ERROR" ? "FAIL" : d.status;
      var ok = (!text || d.search.indexOf(text) !== -1) &&
        (!sev.value || d.sev === sev.value) &&
        (!tgt.value || d.target === tgt.value) &&
        (!bm.value || d.bench === bm.value) &&
        (!anyStatus || active[st]);
      p.rule.classList.toggle("hidden", !ok);
      p.detail.classList.toggle("hidden", !ok || !p.rule.classList.contains("open"));
      if (ok) shown++;
    });
    // group header rows show how many of their rules are visible
    var head = null, n = 0;
    function close() { if (head) { head.classList.toggle("hidden", n === 0); $(".gn", head).textContent = " (" + nf().format(n) + ")"; } }
    $$("tr", tbody).forEach(function (r) {
      if (r.classList.contains("grp")) { close(); head = r; n = 0; }
      else if (r.classList.contains("rule") && !r.classList.contains("hidden")) n++;
    });
    close();
    count.textContent = t("report.results.showing", { shown: shown, total: pairs.length });
    none.classList.toggle("hidden", shown !== 0);
    if (clearBtn) clearBtn.classList.toggle("hidden", !filtersOn());
  }
  dynamic.push(function () { if (tbody) render(); });

  function setOpen(p, open) {
    p.rule.classList.toggle("open", open);
    var b = $(".tgl", p.rule);
    if (b) b.setAttribute("aria-expanded", open ? "true" : "false");
    p.detail.classList.toggle("hidden", !open || p.rule.classList.contains("hidden"));
  }
  function pairOf(tr) { return pairs.filter(function (x) { return x.rule === tr; })[0]; }
  function resetFilters() {
    chips.forEach(function (c) { c.setAttribute("aria-pressed", "false"); });
    active = {}; q.value = ""; sev.value = ""; tgt.value = ""; bm.value = "";
  }

  if (tbody) {
    tbody.addEventListener("click", function (e) {
      var tr = e.target.closest("tr.rule");
      if (!tr || e.target.closest("a")) return;
      var p = pairOf(tr);
      if (p) setOpen(p, !tr.classList.contains("open"));
    });
    chips.forEach(function (c) {
      c.addEventListener("click", function () {
        var s = c.getAttribute("data-status");
        var on = c.getAttribute("aria-pressed") !== "true";
        c.setAttribute("aria-pressed", on ? "true" : "false");
        if (on) active[s] = true; else delete active[s];
        apply();
      });
    });
    [q, sev, tgt, bm].forEach(function (el) { el.addEventListener("input", apply); });
    if (grp) grp.addEventListener("change", render);
    if (clearBtn) clearBtn.addEventListener("click", function () { resetFilters(); apply(); });
    var expand = $("#expand"), collapse = $("#collapse");
    if (expand) expand.addEventListener("click", function () { pairs.forEach(function (p) { if (!p.rule.classList.contains("hidden")) setOpen(p, true); }); });
    if (collapse) collapse.addEventListener("click", function () { pairs.forEach(function (p) { setOpen(p, false); }); });
    $$("th[data-sort]").forEach(function (th) {
      th.addEventListener("click", function () {
        var k = th.getAttribute("data-sort");
        dir = (sortKey === k) ? -dir : 1; sortKey = k;
        $$("th[data-sort]").forEach(function (o) { o.removeAttribute("aria-sort"); });
        th.setAttribute("aria-sort", dir === 1 ? "ascending" : "descending");
        render();
      });
    });
  }

  // jump to one rule in the results tab (from "fix these first" and the action plan)
  function jumpToRule(n) {
    var tr = document.getElementById("r-" + n);
    if (!tr) return;
    resetFilters();
    showTab("results");
    apply();
    var p = pairOf(tr);
    if (p) setOpen(p, true);
    tr.scrollIntoView({ block: "center" });
  }
  $$("a[data-jump]").forEach(function (a) {
    a.addEventListener("click", function (e) { e.preventDefault(); jumpToRule(a.getAttribute("data-jump")); });
  });

  // ---- by benchmark ----------------------------------------------------------------------
  $$(".bcard[data-bench]").forEach(function (c) {
    c.addEventListener("click", function () {
      resetFilters();
      bm.value = c.getAttribute("data-bench");
      apply();
      showTab("results");
      window.scrollTo(0, 0);
    });
  });

  // ---- action plan -----------------------------------------------------------------------
  var main = $("#main");
  var planKey = "tmaegis-plan-" + (main ? main.getAttribute("data-report") : "");
  var boxes = $$("input.pchk");
  var done = {};
  try { (JSON.parse(store.get(planKey) || "[]") || []).forEach(function (id) { done[id] = true; }); } catch (e) { done = {}; }
  var progText = $("#prog-text"), progFill = $("#prog-fill"), prog = $("#prog"), toast = $("#toast"), toastTimer = null;

  function saveDone() { store.set(planKey, JSON.stringify(Object.keys(done))); }
  function progress() {
    if (!progText) return;
    var total = boxes.length, d = boxes.filter(function (b) { return b.checked; }).length;
    progText.textContent = tn("report.plan.progress", total, { done: d });
    progFill.style.width = total ? Math.round(d * 100 / total) + "%" : "0";
    if (prog) prog.setAttribute("aria-valuenow", String(d));
  }
  boxes.forEach(function (b) {
    b.checked = !!done[b.getAttribute("data-id")];
    b.closest("li").classList.toggle("done", b.checked);
    b.addEventListener("change", function () {
      var id = b.getAttribute("data-id");
      if (b.checked) done[id] = true; else delete done[id];
      b.closest("li").classList.toggle("done", b.checked);
      saveDone(); progress();
    });
  });
  progress();
  dynamic.push(progress);

  function say(msg) {
    if (!toast) return;
    toast.textContent = msg;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { toast.textContent = ""; }, 3500);
  }
  function copyText(text, ok) {
    function fallback() {
      var ta = $("#copy-buffer");
      try {
        ta.value = text; ta.removeAttribute("aria-hidden"); ta.select();
        var res = document.execCommand("copy");
        ta.setAttribute("aria-hidden", "true"); ta.value = "";
        if (res) { ok(); return; }
      } catch (e) {}
      say(t("report.plan.copyfail"));
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(ok, fallback);
    } else fallback();
  }
  function checklist() {
    var host = titleHost || "-";
    var sc = $(".gauge [data-num]");
    var when = "";
    var tm = $("time[data-ts]");
    if (tm) when = tm.textContent;
    var out = ["# " + t("report.plan.checklist.title", { host: host }), "", t("report.plan.checklist.meta", { score: sc ? sc.textContent : "", date: when }), ""];
    $$(".pgroup").forEach(function (g) {
      var items = $$("li.pi", g);
      if (!items.length) return;
      out.push("## " + t("report.plan." + g.getAttribute("data-group") + ".title") + " (" + items.length + ")", "");
      items.forEach(function (li) {
        var cb = $("input.pchk", li);
        var sv = $(".pill[class*='sev-']", li);
        var rem = $(".prem", li);
        out.push("- [" + (cb.checked ? "x" : " ") + "] **[" + (sv ? sv.textContent : "") + "]** " + $(".ptitle2", li).textContent + " (`" + li.getAttribute("data-id") + "`)");
        if (rem) {
          out.push("  ```");
          rem.textContent.split("\n").forEach(function (l) { out.push("  " + l); });
          out.push("  ```");
        }
      });
      out.push("");
    });
    return out.join("\n");
  }
  var copyList = $("#copy-list");
  if (copyList) copyList.addEventListener("click", function () {
    copyText(checklist(), function () { say(t("report.plan.copied")); });
  });
  $$(".copyfix").forEach(function (b) {
    b.addEventListener("click", function () {
      var rem = $(".prem", b.closest("li"));
      if (!rem) return;
      copyText(rem.textContent, function () {
        b.textContent = t("report.plan.copyfixed");
        setTimeout(function () { b.textContent = t("report.plan.copyfix"); }, 1800);
      });
    });
  });
  var resetPlan = $("#reset-plan");
  if (resetPlan) resetPlan.addEventListener("click", function () {
    if (!window.confirm(t("report.plan.resetconfirm"))) return;
    done = {}; store.del(planKey);
    boxes.forEach(function (b) { b.checked = false; b.closest("li").classList.remove("done"); });
    progress();
  });

  // ---- routing ---------------------------------------------------------------------------
  function route(first) {
    var h = decodeURIComponent((location.hash || "").slice(1));
    if (names.indexOf(h) >= 0) { showTab(h, { hash: false }); return; }
    var m = /^r-(\d+)$/.exec(h);
    if (m && document.getElementById(h)) {
      resetFilters(); showTab("results", { hash: false }); apply();
      var p = pairOf(document.getElementById(h));
      if (p) setOpen(p, true);
      document.getElementById(h).scrollIntoView({ block: "center" });
      return;
    }
    if (first) {
      var s = store.get("tmaegis-tab");
      showTab(s && names.indexOf(s) >= 0 ? s : "overview", { hash: false });
    }
  }
  window.addEventListener("hashchange", function () { route(false); });

  document.addEventListener("keydown", function (e) {
    if (e.key === "/" && q && document.activeElement !== q && !/input|select|textarea/i.test(document.activeElement.tagName)) {
      e.preventDefault(); showTab("results"); q.focus();
    }
  });

  if (tbody) render();
  applyLang(lang);
  route(true);
})();
