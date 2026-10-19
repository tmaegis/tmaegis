import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import en from "../messages/en-US.json";
import { DEVICE_GUIDES, GUIDE_MODES, NETWORK_SAMPLES, NOTES, WHERE, buildGuideRows, guideCommand } from "./device-guides";
import { TARGETS } from "./targets";

interface Row { id: string; count: number; manual: number; auto: number; device?: number }

function generatedTargets(): Row[] {
  const file = path.join(import.meta.dirname, "..", "generated", "coverage.json");
  if (!fs.existsSync(file)) throw new Error("generated/coverage.json is missing: run `npm run gen` before the tests");
  return (JSON.parse(fs.readFileSync(file, "utf8")) as { rows: Row[] }).rows;
}

const devices = (en as unknown as { downloads: { devices: Record<string, Record<string, unknown>> } }).downloads.devices;

describe("device guides", () => {
  const rows = generatedTargets();

  it("every target in the generated rule data has a guide (add it to lib/device-guides.ts)", () => {
    const missing = rows.filter((r) => !DEVICE_GUIDES[r.id]).map((r) => r.id);
    expect(missing).toEqual([]);
    expect(buildGuideRows(rows).missing).toEqual([]);
    expect(buildGuideRows(rows).rows).toHaveLength(rows.length);
  });

  it("has no guide for a target that does not exist", () => {
    for (const id of Object.keys(DEVICE_GUIDES)) expect(TARGETS[id], id).toBeDefined();
  });

  it("uses known modes, places and notes, and every one has an English message", () => {
    for (const [id, g] of Object.entries(DEVICE_GUIDES)) {
      expect(GUIDE_MODES, id).toContain(g.mode);
      expect(WHERE, id).toContain(g.where);
      if (g.note) expect(NOTES, id).toContain(g.note);
    }
    for (const m of GUIDE_MODES) expect((devices.modes as Record<string, unknown>)[m], m).toBeDefined();
    for (const w of WHERE) expect(typeof devices.where[w], w).toBe("string");
    for (const n of NOTES) expect(typeof devices.notes[n], n).toBe("string");
  });

  it("points AIX, Solaris and FreeBSD at the experimental builds", () => {
    for (const id of ["aix", "solaris", "freebsd"]) expect(DEVICE_GUIDES[id].note, id).toBe("experimental");
  });

  it("tells network devices how to export the configuration", () => {
    for (const id of NETWORK_SAMPLES) {
      const g = DEVICE_GUIDES[id];
      expect(g.mode, id).toBe("device-config");
      expect((g.how?.length ?? 0) > 0 || !!g.ui, id).toBe(true);
    }
  });

  it("every target whose rules read an exported file is a device-config guide", () => {
    for (const r of rows.filter((x) => (x.device ?? 0) > 0)) expect(DEVICE_GUIDES[r.id].mode, r.id).toBe("device-config");
  });

  it("cloud and Kubernetes guides say how to sign in first", () => {
    for (const [id, g] of Object.entries(DEVICE_GUIDES)) if (g.mode === "cloud" || g.mode === "k8s") expect(g.how?.length, id).toBeGreaterThan(0);
  });

  it("targets with no automated check are checklist-only or read an exported file", () => {
    for (const r of rows.filter((x) => x.auto === 0)) expect(["manual", "device-config"], r.id).toContain(DEVICE_GUIDES[r.id].mode);
  });

  it("lists the 17 sample targets and matches samples/network when the folder is present", () => {
    expect(NETWORK_SAMPLES).toHaveLength(17);
    const dir = path.join(import.meta.dirname, "..", "..", "samples", "network");
    if (!fs.existsSync(dir)) return;
    for (const d of fs.readdirSync(dir)) {
      if (fs.statSync(path.join(dir, d)).isDirectory()) expect(NETWORK_SAMPLES as readonly string[], d).toContain(d);
    }
  });

  it("builds the scan and sample commands", () => {
    const { rows: built } = buildGuideRows(rows);
    const ios = built.find((r) => r.id === "cisco-ios")!;
    expect(ios.command).toBe("tmaegis scan --target cisco-ios --device-config <file>");
    expect(ios.sample).toEqual({
      path: "samples/network/cisco-ios",
      command: "./tmaegis scan --target cisco-ios --device-config samples/network/cisco-ios/cisco-ios-noncompliant.txt",
      windowsCommand: ".\\tmaegis.exe scan --target cisco-ios --device-config samples\\network\\cisco-ios\\cisco-ios-noncompliant.txt",
    });
    expect(built.find((r) => r.id === "aws")?.sample).toBeUndefined();
    expect(guideCommand({ id: "android", count: 5, auto: 1, manual: 4 }, DEVICE_GUIDES.android)).toBe("tmaegis scan --target android --include-manual");
    expect(guideCommand({ id: "ios", count: 5, auto: 0, manual: 5 }, DEVICE_GUIDES.ios)).toBe("tmaegis scan --target ios --include-manual");
    expect(built.map((r) => r.mode)).toEqual([...built.map((r) => r.mode)].sort((a, b) => GUIDE_MODES.indexOf(a) - GUIDE_MODES.indexOf(b)));
  });
});
