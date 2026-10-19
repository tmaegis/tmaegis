import { describe, expect, it } from "vitest";
import { detectFromUserAgent, detectPlatform, linuxFamily, runSnippet } from "./detect-platform";

describe("linuxFamily", () => {
  it("guesses the package family only when the browser names the distribution", () => {
    expect(linuxFamily("Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0")).toBe("debian");
    expect(linuxFamily("Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0")).toBe("fedora");
    expect(linuxFamily("Mozilla/5.0 (X11; openSUSE; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0")).toBe("suse");
    expect(linuxFamily("Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0")).toBeNull();
    expect(linuxFamily("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Ubuntu")).toBeNull();
    expect(linuxFamily("Mozilla/5.0 (Linux; Android 14; Pixel 8) Chrome/130.0.0.0 Mobile Safari/537.36")).toBeNull();
    expect(linuxFamily("")).toBeNull();
  });
});

const UA = {
  winChrome: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
  winArm: "Mozilla/5.0 (Windows NT 10.0; ARM64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
  macSafari: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
  linuxFirefox: "Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0",
  linuxArm: "Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
  chromeOs: "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
  android: "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36",
  iphone: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
};

describe("detectFromUserAgent", () => {
  it("Windows x64", () => expect(detectFromUserAgent(UA.winChrome)).toEqual({ os: "windows", arch: "x64", archSure: true, mobile: false }));
  it("Windows on ARM", () => expect(detectFromUserAgent(UA.winArm)).toMatchObject({ os: "windows", arch: "arm64", archSure: true }));
  it("Linux x86_64 and aarch64", () => {
    expect(detectFromUserAgent(UA.linuxFirefox)).toMatchObject({ os: "linux", arch: "x64", archSure: true });
    expect(detectFromUserAgent(UA.linuxArm)).toMatchObject({ os: "linux", arch: "arm64", archSure: true });
    expect(detectFromUserAgent(UA.chromeOs)).toMatchObject({ os: "linux" });
  });
  it("macOS hides the CPU, so it is a guess for Apple silicon", () => expect(detectFromUserAgent(UA.macSafari)).toEqual({ os: "macos", arch: "arm64", archSure: false, mobile: false }));
  it("phones are mobile with no recommendation", () => {
    expect(detectFromUserAgent(UA.android)).toMatchObject({ os: null, arch: null, mobile: true });
    expect(detectFromUserAgent(UA.iphone)).toMatchObject({ os: null, mobile: true });
  });
  it("unknown agents give nothing", () => expect(detectFromUserAgent("curl/8.0")).toEqual({ os: null, arch: null, archSure: false, mobile: false }));
  it("empty input is safe", () => expect(detectFromUserAgent("")).toMatchObject({ os: null }));
});

describe("detectPlatform with Client Hints", () => {
  it("Apple silicon via hints beats the Intel user agent", () => {
    expect(detectPlatform(UA.macSafari, { platform: "macOS", architecture: "arm", bitness: "64" })).toEqual({ os: "macos", arch: "arm64", archSure: true, mobile: false });
  });
  it("Intel Mac via hints", () => {
    expect(detectPlatform(UA.macSafari, { platform: "macOS", architecture: "x86", bitness: "64" })).toMatchObject({ os: "macos", arch: "x64", archSure: true });
  });
  it("Windows ARM64 via hints while the user agent claims x64", () => {
    expect(detectPlatform(UA.winChrome, { platform: "Windows", architecture: "arm", bitness: "64" })).toMatchObject({ os: "windows", arch: "arm64" });
  });
  it("Linux x64 via hints", () => {
    expect(detectPlatform(UA.linuxFirefox, { platform: "Linux", architecture: "x86", bitness: "64" })).toMatchObject({ os: "linux", arch: "x64" });
  });
  it("32-bit x86 still maps to the x64 build, 32-bit ARM has no build", () => {
    expect(detectPlatform(UA.winChrome, { platform: "Windows", architecture: "x86", bitness: "32" }).arch).toBe("x64");
    expect(detectPlatform(UA.linuxArm, { platform: "Linux", architecture: "arm", bitness: "32" })).toMatchObject({ os: "linux", arch: "arm64", archSure: true });
  });
  it("empty hints (a user-agent override) fall back to the user agent", () => {
    expect(detectPlatform(UA.winChrome, { platform: "", architecture: "", bitness: "" })).toMatchObject({ os: "windows", arch: "x64" });
    expect(detectPlatform(UA.linuxFirefox, null)).toMatchObject({ os: "linux", arch: "x64" });
  });
  it("mobile hint wins", () => expect(detectPlatform(UA.android, { platform: "Android", mobile: true })).toMatchObject({ os: null, mobile: true }));
  it("unknown platform names keep the user-agent result", () => expect(detectPlatform(UA.winChrome, { platform: "Plan9" })).toMatchObject({ os: "windows" }));
});

describe("runSnippet", () => {
  it("PowerShell on Windows, tar on Linux with sudo, tar on macOS without", () => {
    expect(runSnippet("windows", "t.zip")).toBe("Expand-Archive .\\t.zip -DestinationPath tmaegis; cd tmaegis; .\\tmaegis.exe scan --auto-detect");
    expect(runSnippet("linux", "t.tar.gz")).toBe("mkdir tmaegis && tar -xzf t.tar.gz -C tmaegis && cd tmaegis && sudo ./tmaegis scan --auto-detect");
    expect(runSnippet("macos", "t.tar.gz")).toBe("mkdir tmaegis && tar -xzf t.tar.gz -C tmaegis && cd tmaegis && ./tmaegis scan --auto-detect");
  });
});
