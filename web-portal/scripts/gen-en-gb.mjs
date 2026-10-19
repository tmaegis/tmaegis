// Derives messages/en-GB.json (British spelling overrides only) from en-US.json.
import fs from "node:fs";
const us = JSON.parse(fs.readFileSync(new URL("../messages/en-US.json", import.meta.url), "utf8"));
const swaps = [
  [/\bPublic License\b/g, "Public \u0000L"],
  [/\blicense\b/g, "licence"], [/\bLicense\b/g, "Licence"],
  [/\borganizations?\b/g, (m) => m.replace("z", "s")],
  [/\bcatalog\b/g, "catalogue"],
  [/\bcolor/g, "colour"],
  [/\bbehavior/g, "behaviour"],
  [/\bprogram\b/g, "programme"],
  [/\u0000L/g, "License"],
];
const out = {};
(function walk(o, path, dst) {
  for (const [k, v] of Object.entries(o)) {
    if (typeof v === "object") { dst[k] = {}; walk(v, [...path, k], dst[k]); if (!Object.keys(dst[k]).length) delete dst[k]; }
    else { let s = v; for (const [a, b] of swaps) s = s.replace(a, b); if (s !== v) dst[k] = s; }
  }
})(us, [], out);
fs.writeFileSync(new URL("../messages/en-GB.json", import.meta.url), JSON.stringify(out, null, 2) + "\n");
console.log("en-GB overrides written");
