// Generates lib/platform-icons.ts and the sprite in public/: the platform logos the site shows, and nothing else.
//
//   node scripts/gen-icons.mjs        (re-run after changing ICONS below or upgrading an icon package)
//
// Every platform shows the brand's own logo, taken from an open icon collection (devDependencies only: no icon
// package ships to the browser, the build reads them here and writes one content-hashed sprite). Sources, in the
// order they are preferred for a target:
//
//   1. logos         @iconify-json/logos, the "SVG Logos" collection by Gil Barbara (CC0 1.0). Official
//                    full-colour marks. Used where the brand's mark has a palette of its own (the Microsoft
//                    squares, Windows, Azure, Edge, VS Code, AWS, Chrome, Google, Google Cloud, YugabyteDB) and
//                    the drawing stays small (no 10 KB+ gradient artwork such as Firefox, Safari or Tux).
//   2. si            simple-icons, current release (CC0 1.0). Single-colour marks.
//   3. si-legacy     simple-icons 9.21.0 (npm alias simple-icons-legacy, CC0 1.0): marks that later releases
//                    removed (IBM, IBM Cloud, Oracle). Icons that simple-icons itself flags with a licence of their
//                    own are not used (Exchange and SharePoint carry the Fluent UI assets licence there).
//   4. vendor        scripts/logos/<name>.svg, for products none of the packages has (or has only in an outdated or
//                    restricted version): Wikimedia Commons files whose licence allows reuse (public domain as
//                    PD-textlogo / PD-simple, or CC0; never Commons' "non-free" files), Google Cloud Icons and
//                    Grommet Icons (Apache-2.0), selfh.st/icons (CC BY 4.0) and theSVG (MIT), found through the Iconify API.
//                    scripts/logos/logos.json records each file's source URL, collection, licence, retrieval date
//                    and any change (svgo clean-up always; a lockup reduced to its symbol). Each is credited in
//                    NOTICE. Fitted and checked like an SVG Logos mark.
//   Also searched, with nothing usable for these targets (2026-10): every Iconify set (logos, simple-icons,
//   devicon, vscode-icons, thesvg, selfhst, streamline-logos, cib, fa-brands, mdi ...) and Wikimedia Commons: no
//   openly licensed Bottlerocket, AKS, OKE, LXD, CICS, Db2, WebSphere, IIS, OceanBase, current Extreme Networks or
//   MIT Kerberos mark.
//
// A product without a mark of its own in any of them shows its vendor's logo (IIS: Microsoft; AIX, IBM i, IBM Z,
// CICS, Db2, WebSphere: IBM; Solaris and the Oracle services: Oracle; Bottlerocket: AWS; Azure Linux: Azure; LXD:
// Canonical; HPE Aruba CX: HPE; TDSQL: Tencent Cloud); AKS and OKE show the Kubernetes wheel. Only platforms whose vendor has no usable
// logo fall back to a monogram tile ("mono") or a generic glyph ("glyph"), drawn by components/platform-icon.tsx in
// the same style.
//
// Rendering: single-colour marks (si, si-legacy) follow currentColor, and show their brand colour on hover or for a
// selected item, only where it reaches 3:1 against the page and card backgrounds of that theme (WCAG 1.4.11).
// Full-colour marks (logos, vendor) always keep their own colours. One whose main colours would vanish on a theme's
// background (under 1.5:1, e.g. the dark AWS lettering on the dark theme) is drawn on a small contrasting chip in
// that theme ("chip" below, .picon[data-chip] in app/globals.css).
//
// Product names, logos and brands are property of their respective owners and are used only to identify
// supported platforms. Never draw a logo by hand here: every mark comes from one of the sources above, unchanged but for the
// clean-up recorded in scripts/logos/logos.json.
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import * as si from "simple-icons";

const here = path.dirname(fileURLToPath(import.meta.url));
const out = path.join(here, "..", "lib", "platform-icons.ts");

const brand = (slug) => ({ src: "si", name: slug });
const legacy = (slug) => ({ src: "si-legacy", name: slug });
const logo = (name, title) => ({ src: "logos", name, title });
const vendored = (name) => ({ src: "vendor", name });
const mono = (text) => ({ mono: text });
const glyph = (name) => ({ glyph: name });

// Vendor logos used by several products.
const MICROSOFT = logo("microsoft-icon", "Microsoft");
const AZURE = logo("microsoft-azure", "Microsoft Azure");
const AWS = logo("aws", "Amazon Web Services");
const IBM = legacy("ibm");
const ORACLE = legacy("oracle");
const GOOGLE_CLOUD = logo("google-cloud", "Google Cloud");

/**
 * Every scan target of lib/targets.ts (lib/platform-icons.test.ts fails when one is missing), plus the systems
 * named on the home page and the download tabs. Keys are target ids or short names; values say which mark to draw.
 */
const ICONS = {
  // Operating systems
  windows: logo("microsoft-windows-icon", "Windows"),
  linux: brand("linux"),
  "azure-container-linux": AZURE, // vendor: Azure Linux has no mark of its own
  bottlerocket: AWS, // vendor: no openly licensed Bottlerocket mark
  cos: vendored("container-optimized-os"),
  macos: brand("apple"),
  aix: IBM, // vendor
  solaris: ORACLE, // vendor
  freebsd: brand("freebsd"),
  "ibm-i": IBM,
  "ibm-z": IBM,
  cics: IBM,
  "db2-zos": IBM,
  // Server software
  apache: brand("apache"),
  nginx: brand("nginx"),
  iis: MICROSOFT, // vendor
  tomcat: brand("apachetomcat"),
  websphere: IBM,
  exchange: vendored("microsoft-exchange"), // Commons (public domain), not simple-icons' Fluent UI-licensed copy
  sharepoint: vendored("microsoft-sharepoint"), // likewise
  bind: vendored("bind9"),
  kerberos: glyph("key"), // no openly licensed MIT Kerberos mark
  vmware: brand("vmware"),
  ros: brand("ros"),
  // Databases
  cassandra: brand("apachecassandra"),
  cockroachdb: brand("cockroachlabs"),
  db2: IBM,
  mariadb: brand("mariadb"),
  mongodb: brand("mongodb"),
  mssql: vendored("microsoft-sql-server"),
  mysql: brand("mysql"),
  oceanbase: mono("OB"), // no openly licensed OceanBase or Ant Group mark
  "oracle-db": ORACLE,
  postgres: brand("postgresql"),
  redis: brand("redis"),
  tdsql: vendored("tencent-cloud"), // vendor: TDSQL is a Tencent Cloud database
  yugabytedb: logo("yugabyte-icon", "YugabyteDB"),
  // Containers and Kubernetes (managed clusters without a mark of their own show the Kubernetes wheel)
  docker: brand("docker"),
  lxd: brand("canonical"), // vendor: LXD is Canonical's; the Linux Containers mark now stands for LXC and Incus
  kubernetes: brand("kubernetes"),
  aks: brand("kubernetes"),
  "aks-automatic": brand("kubernetes"),
  eks: logo("aws-eks", "Amazon EKS"),
  "eks-auto": logo("aws-eks", "Amazon EKS"),
  gke: vendored("google-kubernetes-engine"),
  "gke-autopilot": vendored("google-kubernetes-engine"),
  oke: brand("kubernetes"),
  openshift: brand("redhatopenshift"),
  "openshift-virt": brand("redhatopenshift"),
  // Desktop, browsers and Office
  chrome: logo("chrome", "Google Chrome"),
  edge: logo("microsoft-edge", "Microsoft Edge"),
  firefox: brand("firefoxbrowser"),
  safari: brand("safari"),
  office: vendored("microsoft-365"), // the Office app became the Microsoft 365 app (2022)
  defender: vendored("microsoft-defender"),
  vscode: logo("visual-studio-code", "Visual Studio Code"),
  zoom: brand("zoom"),
  // Network devices
  "cisco-asa": brand("cisco"),
  "cisco-ftd": brand("cisco"),
  "cisco-ios": brand("cisco"),
  "cisco-iosxr": brand("cisco"),
  "cisco-nxos": brand("cisco"),
  paloalto: brand("paloaltonetworks"),
  fortigate: brand("fortinet"),
  fortiswitch: brand("fortinet"),
  checkpoint: vendored("check-point"),
  pfsense: brand("pfsense"),
  opnsense: brand("opnsense"),
  sophos: vendored("sophos"),
  "juniper-junos": brand("junipernetworks"),
  "arista-eos": vendored("arista"),
  "aruba-cx": vendored("hpe"), // vendor: HPE Aruba Networking carries the HPE element
  "extreme-slx": mono("EX"), // no openly licensed current Extreme Networks mark
  "f5-bigip": brand("f5"),
  // Cloud and SaaS
  aws: AWS,
  azure: AZURE,
  gcp: GOOGLE_CLOUD,
  oci: ORACLE,
  alibaba: brand("alibabacloud"),
  ibmcloud: legacy("ibmcloud"),
  m365: vendored("microsoft-365"),
  "google-workspace": logo("google-icon", "Google"),
  github: brand("github"),
  "oracle-saas": ORACLE,
  "power-platform": vendored("microsoft-power-platform"),
  // Mobile and device management
  ios: brand("apple"),
  android: brand("android"),
  "intune-ios": vendored("microsoft-intune"),
  "intune-macos": vendored("microsoft-intune"),
  "intune-office": vendored("microsoft-intune"),
  "intune-defender": vendored("microsoft-intune"),
  // DevSecOps, AI tooling, printers and storage
  gitlab: brand("gitlab"),
  "supply-chain": glyph("link"), // a practice, not a product
  "mcp-server": brand("modelcontextprotocol"),
  printer: glyph("printer"), // printers of every make
  flasharray: vendored("pure-storage"),

  // Not scan targets: distributions and systems named on the home page and the download tabs
  ubuntu: brand("ubuntu"),
  debian: brand("debian"),
  rhel: brand("redhat"),
  rocky: brand("rockylinux"),
  almalinux: brand("almalinux"),
  suse: brand("suse"),
  riscv: brand("riscv"),
  s390x: IBM,
  unix: glyph("terminal"),
  source: glyph("code"),
};

// ---------- contrast (WCAG 2.x relative luminance) ----------
const lum = (hex) => {
  const c = [0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
};
const ratio = (a, b) => {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
};
// The page background and the solid card colour of each theme (app/globals.css).
const DARK = ["0b0f19", "111827"];
const LIGHT = ["f5f9fd", "ffffff"];
const okOn = (hex, bgs) => bgs.every((bg) => ratio(hex, bg) >= 3);

// ---------- path minifier ----------
// simple-icons paths carry three decimals. At the sizes the site draws them (16 to 48 px), two are plenty, but
// rounding relative coordinates naively lets the error add up along a path. So every point is tracked twice
// (exact and as written) and each relative step is rounded towards the exact position: the error never grows.
const PARAMS = { m: "xy", l: "xy", t: "xy", h: "x", v: "y", c: "xyxyxy", s: "xyxy", q: "xyxy", a: "rrnffxy", z: "" };
export function minifyPath(d, digits = 2, box = null) {
  const r = (n, dg = digits) => Math.round(n * 10 ** dg) / 10 ** dg;
  let i = 0;
  const ws = () => {
    while (i < d.length && /[\s,]/.test(d[i])) i++;
  };
  const num = () => {
    ws();
    const m = /^[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?/.exec(d.slice(i));
    if (!m) throw new Error(`bad path near "${d.slice(i, i + 12)}"`);
    i += m[0].length;
    return parseFloat(m[0]);
  };
  const flag = () => {
    ws();
    const c = d[i++];
    if (c !== "0" && c !== "1") throw new Error("bad arc flag");
    return +c;
  };
  const fmt = (n, dg = digits) => {
    let s = String(r(n, dg) === 0 ? 0 : r(n, dg));
    if (s.startsWith("0.")) s = s.slice(1);
    else if (s.startsWith("-0.")) s = "-" + s.slice(2);
    return s;
  };
  let out = "";
  let last = ""; // last emitted token, to decide whether a separator is needed
  const emit = (tok, isNum) => {
    if (isNum && last && /[\d.]$/.test(last) && !tok.startsWith("-") && !(tok.startsWith(".") && last.includes("."))) out += " ";
    out += tok;
    last = isNum ? tok : "";
  };
  let ex = 0, ey = 0, wx = 0, wy = 0; // exact and written current point
  let sx = 0, sy = 0, swx = 0, swy = 0; // subpath start
  let cmd = "";
  while (true) {
    ws();
    if (i >= d.length) break;
    if (/[a-zA-Z]/.test(d[i])) {
      cmd = d[i++];
      emit(cmd, false);
      if (cmd.toLowerCase() === "z") {
        ex = sx; ey = sy; wx = swx; wy = swy;
        continue;
      }
    } else if (!cmd || cmd.toLowerCase() === "z") throw new Error("number without a command");
    const lc = cmd.toLowerCase();
    const rel = cmd === lc;
    const spec = PARAMS[lc];
    const vals = [];
    for (const p of spec) vals.push(p === "f" ? flag() : num());
    // exact end point of this segment
    let nx = ex, ny = ey;
    const xi = spec.lastIndexOf("x"), yi = spec.lastIndexOf("y");
    if (xi >= 0) nx = rel ? ex + vals[xi] : vals[xi];
    if (yi >= 0) ny = rel ? ey + vals[yi] : vals[yi];
    // Arcs keep one more decimal: a half circle whose chord grows past twice its radius would be redrawn larger.
    const dg = lc === "a" ? digits + 1 : digits;
    const written = vals.map((v, k) => {
      const p = spec[k];
      if (p === "f") return String(v);
      if (p === "r" || p === "n") return fmt(v, 3);
      if (!rel) return fmt(v, dg);
      // relative coordinate: aim at the exact absolute position from the written current point
      const exact = (p === "x" ? ex : ey) + v;
      return fmt(exact - (p === "x" ? wx : wy), dg);
    });
    for (let k = 0; k < written.length; k++) emit(written[k], true);
    // advance the written point by what was actually written
    if (xi >= 0) wx = rel ? r(wx + parseFloat(written[xi]), 3) : parseFloat(written[xi]);
    if (yi >= 0) wy = rel ? r(wy + parseFloat(written[yi]), 3) : parseFloat(written[yi]);
    ex = nx; ey = ny;
    if (box) Object.assign(box, { x0: Math.min(box.x0 ?? ex, ex), x1: Math.max(box.x1 ?? ex, ex), y0: Math.min(box.y0 ?? ey, ey), y1: Math.max(box.y1 ?? ey, ey) });
    if (lc === "m") {
      sx = ex; sy = ey; swx = wx; swy = wy;
      cmd = rel ? "l" : "L"; // further pairs after a moveto are linetos
    }
  }
  return out;
}

// ---------- sources ----------
const modDir = (name) => path.join(here, "..", "node_modules", name);
const pkgVersion = (name) => JSON.parse(fs.readFileSync(path.join(modDir(name), "package.json"), "utf8")).version;
const SOURCES = {
  si: { label: `simple-icons ${pkgVersion("simple-icons")}`, license: "CC0 1.0" },
  "si-legacy": { label: `simple-icons ${pkgVersion("simple-icons-legacy")}`, license: "CC0 1.0" },
  logos: { label: `SVG Logos (@iconify-json/logos ${pkgVersion("@iconify-json/logos")})`, license: "CC0 1.0" },
  vendor: { label: "logos vendored in scripts/logos (Wikimedia Commons, Google Cloud Icons, Grommet Icons, selfh.st/icons, theSVG)", license: "public domain, CC0 1.0, Apache-2.0, CC BY 4.0 or MIT as listed in scripts/logos/logos.json" },
};
const siCurrent = new Map(Object.values(si).filter((x) => x && typeof x === "object" && x.slug).map((x) => [x.slug, x]));
const legacyDir = modDir("simple-icons-legacy");
const legacyData = JSON.parse(fs.readFileSync(path.join(legacyDir, "_data", "simple-icons.json"), "utf8"));
const legacyByTitle = new Map((legacyData.icons ?? legacyData).map((x) => [x.title, x]));
const logosSet = JSON.parse(fs.readFileSync(path.join(modDir("@iconify-json/logos"), "icons.json"), "utf8"));
const vendorDir = path.join(here, "logos");
const vendorMeta = JSON.parse(fs.readFileSync(path.join(vendorDir, "logos.json"), "utf8"));

/** A single-colour simple-icons mark: { title, hex, path } in a 24 x 24 box. */
function siIcon(src, slug) {
  if (src === "si") {
    const icon = siCurrent.get(slug);
    if (!icon) throw new Error(`simple-icons has no icon "${slug}" (renamed or removed?)`);
    if (icon.license && icon.license.type === "custom") throw new Error(`simple-icons "${slug}" has a custom licence: ${icon.license.url}`);
    return { title: icon.title, hex: icon.hex, path: icon.path };
  }
  const file = path.join(legacyDir, "icons", slug + ".svg");
  if (!fs.existsSync(file)) throw new Error(`simple-icons-legacy has no icon "${slug}"`);
  const svg = fs.readFileSync(file, "utf8");
  const title = /<title>([^<]+)<\/title>/.exec(svg)[1].replace(/&amp;/g, "&");
  const meta = legacyByTitle.get(title);
  if (!meta) throw new Error(`simple-icons-legacy: no metadata for "${title}"`);
  // Icons with a licence of their own (e.g. the Fluent UI assets licence) may not be redistributed here.
  if (meta.license) throw new Error(`simple-icons-legacy "${slug}" carries its own licence (${JSON.stringify(meta.license)}): not usable`);
  return { title, hex: meta.hex, path: /<path d="([^"]+)"/.exec(svg)[1] };
}

const hex6 = (c) => {
  const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(c.trim());
  if (!m) return null;
  return (m[1].length === 3 ? m[1].replace(/./g, "$&$&") : m[1]).toUpperCase();
};

/**
 * A full-colour SVG Logos mark, fitted (aspect kept, centred) into a 24 x 24 box: { title, hex, body, chip }.
 * Paths are minified to the precision the box needs and ids are made unique within the sprite.
 */
function logosIcon(name, title, symId) {
  const ic = logosSet.icons[name];
  if (!ic) throw new Error(`@iconify-json/logos has no icon "${name}"`);
  return fitIcon({ ...ic, width: ic.width ?? logosSet.width ?? 16, height: ic.height ?? logosSet.height ?? 16 }, `logos "${name}"`, title, symId);
}

/** A vendored logo (scripts/logos/<name>.svg, documented in scripts/logos/logos.json), fitted like an SVG Logos mark. */
function vendorIcon(name, symId) {
  const meta = vendorMeta[name];
  if (!meta || !meta.title || !meta.source || !meta.license) throw new Error(`scripts/logos/logos.json: no title, source and licence for "${name}"`);
  const svg = fs.readFileSync(path.join(vendorDir, name + ".svg"), "utf8").trim();
  const root = /^<svg\b([^>]*)>([\s\S]*)<\/svg>$/.exec(svg);
  if (!root) throw new Error(`scripts/logos/${name}.svg: not a single <svg> element`);
  const [left, top, width, height] = (/\sviewBox="([^"]+)"/.exec(root[1])?.[1] ?? "").trim().split(/[\s,]+/).map(Number);
  if (!(width > 0 && height > 0)) throw new Error(`scripts/logos/${name}.svg: no viewBox`);
  // presentation attributes of the root (fill="none", fill-rule ...) move to a group, so the body keeps its meaning
  const attrs = [...root[1].matchAll(/\s((?:fill|fill-rule|clip-rule|stroke[a-z-]*)="[^"]*")/g)].map((m) => " " + m[1]).join("");
  const body = attrs ? `<g${attrs}>${root[2]}</g>` : root[2];
  return fitIcon({ body, width, height, left, top }, `scripts/logos/${name}.svg`, meta.title, symId);
}

/** Fits a drawing { body, width, height, left, top } into the 24 x 24 box: { title, hex, body, chip, aspect }. */
function fitIcon(ic, label, title, symId) {
  const { width: w, height: h } = ic;
  const left = ic.left ?? 0, top = ic.top ?? 0;
  const k = 24 / Math.max(w, h);
  const r4 = (n) => +n.toFixed(4);
  const tx = r4((24 - w * k) / 2 - left * k), ty = r4((24 - h * k) / 2 - top * k);
  const digits = Math.max(0, Math.round(Math.log10(2400 / Math.max(w, h))));
  let body = ic.body.replace(/\sd="([^"]+)"/g, (_, d) => ` d="${minifyPath(d, digits)}"`);
  // unique ids: <symbol id="l-aws"> owns l-aws-0, l-aws-1 ...
  const ids = [...body.matchAll(/\sid="([^"]+)"/g)].map((m) => m[1]);
  ids.forEach((id, i) => {
    const to = `${symId}-${i}`;
    const esc = id.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    body = body.replace(new RegExp(`id="${esc}"`, "g"), `id="${to}"`).replace(new RegExp(`#${esc}(?=[")])`, "g"), `#${to}`);
  });
  body = body.replace(/xlink:href=/g, "href=");
  // no active content and nothing fetched from elsewhere
  if (/<(script|image|foreignObject|style|a)\b|\son\w+=|href="(?!#)|url\((?!#)/i.test(body)) throw new Error(`${label}: unexpected markup`);
  // Colour weight: the bounding-box area each colour is painted over, a cheap proxy for its share of the logo. A
  // shape drawn inside an earlier one (the letter on an app tile) sits on the logo itself, not on the page, so it
  // cannot vanish against the background and does not count. Fills are inherited from groups; a gradient counts
  // as its opaque stops; translucent shapes and stops are highlights or shadows over the mark.
  const stops = new Map();
  for (const m of body.matchAll(/<(linearGradient|radialGradient)\b([^>]*)>([\s\S]*?)<\/\1>/g)) {
    const id = /\sid="([^"]+)"/.exec(m[2])?.[1];
    const cs = [...m[3].matchAll(/<stop\b([^>]*)>/g)].filter((t) => +(/stop-opacity="([^"]+)"/.exec(t[1])?.[1] ?? 1) >= 0.5).map((t) => hex6(/stop-color="([^"]+)"/.exec(t[1])?.[1] ?? "#000")).filter(Boolean);
    if (id) stops.set(id, cs);
  }
  const weight = new Map();
  const outer = new Map(); // the part of each colour's weight that lies on the page background
  const boxes = [];
  const fills = []; // inherited fill of each open element
  const drawn = body.replace(/<(defs|mask|clipPath)\b[\s\S]*?<\/\1>/g, ""); // not painted by themselves
  for (const [, close, tag, attrs, selfClosing] of drawn.matchAll(/<(\/?)([a-zA-Z]+)\b([^>]*?)(\/?)>/g)) {
    if (close) {
      fills.pop();
      continue;
    }
    const fill = /\sfill="([^"]+)"/.exec(attrs)?.[1] ?? fills.at(-1) ?? "#000";
    if (!selfClosing) fills.push(fill);
    if (!/^(path|circle|rect|ellipse|polygon)$/.test(tag)) continue;
    if (+(/\s(?:fill-)?opacity="([^"]+)"/.exec(attrs)?.[1] ?? 1) < 0.5) continue; // a translucent highlight or shadow
    const num = (a) => +(new RegExp(`\\s${a}="([^"]+)"`).exec(attrs)?.[1] ?? 0);
    let b = {};
    const d = /\sd="([^"]+)"/.exec(attrs)?.[1];
    if (d) minifyPath(d, 3, b);
    else if (tag === "rect") b = { x0: num("x"), y0: num("y"), x1: num("x") + num("width"), y1: num("y") + num("height") };
    else if (tag === "circle" || tag === "ellipse") {
      const rx = num("r") || num("rx"), ry = num("r") || num("ry");
      b = { x0: num("cx") - rx, y0: num("cy") - ry, x1: num("cx") + rx, y1: num("cy") + ry };
    }
    const area = Math.max(1e-6, ((b.x1 ?? 0) - (b.x0 ?? 0)) * ((b.y1 ?? 0) - (b.y0 ?? 0)));
    const cs = /^url\(#([^)]+)\)$/.test(fill) ? stops.get(/^url\(#([^)]+)\)$/.exec(fill)[1]) ?? [] : [hex6(fill)].filter(Boolean);
    const inside = boxes.some((o) => b.x0 >= o.x0 - 1e-3 && b.y0 >= o.y0 - 1e-3 && b.x1 <= o.x1 + 1e-3 && b.y1 <= o.y1 + 1e-3);
    for (const c of cs) {
      weight.set(c, (weight.get(c) ?? 0) + area / cs.length);
      if (!inside) outer.set(c, (outer.get(c) ?? 0) + area / cs.length);
    }
    if (cs.length && b.x1 !== undefined) boxes.push(b);
  }
  const total = [...outer.values()].reduce((a, b) => a + b, 0) || 1;
  const vanishes = (bgs) => [...outer].filter(([c]) => bgs.some((bg) => ratio(c, bg) < 1.5)).reduce((a, [, v]) => a + v, 0) / total > 0.25;
  const chip = [...(vanishes(DARK) ? ["dark"] : []), ...(vanishes(LIGHT) ? ["light"] : [])].join(" ");
  const main = [...weight].sort((a, b) => b[1] - a[1])[0]?.[0] ?? "000000";
  // fill="#000" restores SVG's default black for paths without a fill of their own (the <use> would pass on currentColor).
  const markup = `<g fill="#000" transform="matrix(${r4(k)} 0 0 ${r4(k)} ${tx} ${ty})">${body}</g>`;
  return { title, hex: main, body: markup, chip, aspect: w / h };
}

// ---------- build ----------
// A mark at least this much wider than tall (a wordmark such as IBM, Cisco or Zoom) gets more of a tile, so it
// carries the same visual weight as a square logo.
const WIDE = 1.6;
const symIdOf = (v) => (v.src === "logos" ? "l-" + v.name : v.src === "vendor" ? "v-" + v.name : v.name);
const entries = Object.values(ICONS).filter((v) => v.src);
const used = [...new Set(entries.map(symIdOf))].sort();
const brands = {};
for (const id of used) {
  const v = entries.find((e) => symIdOf(e) === id);
  if (v.src === "logos" || v.src === "vendor") {
    const l = v.src === "logos" ? logosIcon(v.name, v.title, id) : vendorIcon(v.name, id);
    brands[id] = { title: l.title, hex: l.hex, color: true, ...(l.chip ? { chip: l.chip } : {}), ...(l.aspect >= WIDE ? { wide: true } : {}), dark: false, light: false, source: `${v.src}:${v.name}`, svg: l.body };
  } else {
    const s = siIcon(v.src, v.name);
    const hex = s.hex.toUpperCase();
    const box = {};
    const d = minifyPath(s.path, 2, box);
    const wide = (box.x1 - box.x0) / Math.max(1, box.y1 - box.y0) >= WIDE;
    brands[id] = { title: s.title, hex, color: false, ...(wide ? { wide: true } : {}), dark: okOn(hex, DARK), light: okOn(hex, LIGHT), source: `${v.src}:${v.name}`, svg: `<path d="${d}"/>` };
  }
}
const usedSources = [...new Set(entries.map((v) => v.src))].sort((a, b) => Object.keys(SOURCES).indexOf(a) - Object.keys(SOURCES).indexOf(b));
const credit = usedSources.map((s) => `${SOURCES[s].label}, ${SOURCES[s].license}`).join("; ");
const TM = "Product names, logos and brands are property of their respective owners and are used only to identify supported platforms.";

// The sprite: one <symbol> per logo, served from /public and cached by the browser, so pages reference a logo
// with <use> instead of repeating its path in the HTML.
const sprite = [
  '<svg xmlns="http://www.w3.org/2000/svg">',
  `<!-- Generated by scripts/gen-icons.mjs from ${credit}. ${TM} -->`,
  ...used.map((s) => `<symbol id="${s}" viewBox="0 0 24 24">${brands[s].svg}</symbol>`),
  "</svg>",
  "",
].join("\n");
// The content hash is in the file name, so the file can be cached for good (next.config.mjs, vercel.json).
const hash = crypto.createHash("sha256").update(sprite).digest("hex").slice(0, 10);
const pub = path.join(here, "..", "public");
for (const f of fs.readdirSync(pub)) if (/^platform-icons\.[0-9a-f]+\.svg$/.test(f)) fs.rmSync(path.join(pub, f));
const spriteName = `platform-icons.${hash}.svg`;
fs.writeFileSync(path.join(pub, spriteName), sprite);

const meta = (s) => {
  const b = brands[s];
  return { title: b.title, hex: b.hex, color: b.color, ...(b.chip ? { chip: b.chip } : {}), ...(b.wide ? { wide: true } : {}), dark: b.dark, light: b.light, source: b.source };
};
const refOf = (v) => (v.src ? { brand: symIdOf(v) } : v);
const lines = [
  `// Generated by scripts/gen-icons.mjs from ${credit}. Do not edit; re-run the script.`,
  `// ${TM}`,
  "",
  "export interface BrandIcon {",
  "  /** The brand's own name for the logo. */",
  "  title: string;",
  "  /** Brand colour (the main colour of a full-colour logo), upper-case hex without #. */",
  "  hex: string;",
  "  /** A full-colour logo: drawn in its own colours, never recoloured. Otherwise a single-colour mark in currentColor. */",
  "  color: boolean;",
  "  /** Themes (\"dark\", \"light\") in which a full-colour logo sits on a contrasting chip, so it does not vanish. */",
  "  chip?: string;",
  "  /** A wordmark much wider than tall: drawn larger on a tile, to carry the weight of a square logo. */",
  "  wide?: boolean;",
  "  /** The brand colour of a single-colour mark reaches 3:1 on the dark / light page and card backgrounds. */",
  "  dark: boolean;",
  "  light: boolean;",
  "  /** Where the logo comes from: <collection>:<icon name> (see scripts/gen-icons.mjs). */",
  "  source: string;",
  "}",
  "",
  'export type GlyphName = "windows" | "sun" | "key" | "shield" | "code" | "link" | "printer" | "terminal";',
  "export type IconRef = { brand: BrandSlug } | { mono: string } | { glyph: GlyphName };",
  "",
  `export type BrandSlug = ${used.map((s) => JSON.stringify(s)).join(" | ")};`,
  "",
  "/** The sprite in public/: one <symbol id=\"<slug>\"> per logo. Reference a logo as `${SPRITE_URL}#<slug>`. */",
  `export const SPRITE_URL = "/${spriteName}";`,
  "",
  "export const BRAND_ICONS: Record<BrandSlug, BrandIcon> = {",
  ...used.map((s) => `  ${JSON.stringify(s)}: ${JSON.stringify(meta(s))},`),
  "};",
  "",
  "/** How each target id (and a few system names) is drawn. */",
  "export const PLATFORM_ICONS: Record<string, IconRef> = {",
  ...Object.entries(ICONS).map(([k, v]) => `  ${JSON.stringify(k)}: ${JSON.stringify(refOf(v))},`),
  "};",
  "",
  "/** Inner SVG of each logo in a 24 x 24 box (the same markup as the sprite), for tests and for drawing outside a page. */",
  "export const BRAND_SVG: Record<BrandSlug, string> = {",
  ...used.map((s) => `  ${JSON.stringify(s)}: ${JSON.stringify(brands[s].svg)},`),
  "};",
  "",
];
fs.writeFileSync(out, lines.join("\n"));
const fallbacks = Object.entries(ICONS).filter(([, v]) => !v.src).map(([k]) => k);
const gz = (await import("node:zlib")).gzipSync(sprite).length;
console.log(`gen-icons: ${used.length} logos (${credit}), ${fallbacks.length} fallbacks (${fallbacks.join(", ")}) -> lib/platform-icons.ts (${fs.statSync(out).size} bytes) and public/${spriteName} (${sprite.length} bytes, ${gz} gzip)`);
if (process.argv.includes("--table")) for (const [k, v] of Object.entries(ICONS)) console.log(`${k}\t${v.src ? brands[symIdOf(v)].source + (brands[symIdOf(v)].chip ? " (chip: " + brands[symIdOf(v)].chip + ")" : "") : v.mono ? "monogram " + v.mono : "glyph " + v.glyph}`);
