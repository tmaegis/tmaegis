import { describe, expect, it } from "vitest";
import { archiveCommand, crossBuildCommand, dockerCiCommand, dockerDeviceCommand, dockerPullCommand, packageCommand, removeCommand, sampleCommand, samplePath } from "./install-commands";

describe("install commands", () => {
  it("unpacks archives with the right tool per system", () => {
    expect(archiveCommand("windows", "tmaegis_0.1.0_windows_amd64.zip")).toMatch(/^Expand-Archive \.\\tmaegis_0\.1\.0_windows_amd64\.zip -DestinationPath tmaegis\n/);
    expect(archiveCommand("macos", "a.tar.gz")).toContain("xattr -d com.apple.quarantine ./tmaegis");
    expect(archiveCommand("linux", "a.tar.gz")).toContain("sudo ./tmaegis scan --auto-detect");
  });
  it("installs local packages with apt, dnf or zypper", () => {
    expect(packageCommand("deb", "t.deb")).toBe("sudo apt install ./t.deb\nsudo tmaegis scan --auto-detect");
    expect(packageCommand("rpm", "t.rpm", "dnf")).toBe("sudo dnf install ./t.rpm\nsudo tmaegis scan --auto-detect");
    expect(packageCommand("rpm", "t.rpm", "zypper")).toBe("sudo zypper install ./t.rpm\nsudo tmaegis scan --auto-detect");
    const both = packageCommand("rpm", "t.rpm");
    expect(both).toContain("sudo dnf install ./t.rpm");
    expect(both).toContain("sudo zypper install ./t.rpm");
    expect(removeCommand("deb")).toBe("sudo apt remove tmaegis");
  });
  it("uses the image reference for docker", () => {
    const ref = "ghcr.io/tmaegis/tmaegis:0.1.0";
    expect(dockerPullCommand(ref)).toBe(`docker pull ${ref}\ndocker run --rm ${ref} --help`);
    expect(dockerDeviceCommand(ref)).toContain(`-v "$PWD:/data:ro" ${ref}`);
    expect(dockerDeviceCommand(ref)).toContain("--device-config /data/running-config.txt");
    expect(dockerCiCommand(ref)).toContain("--format sarif > tmaegis.sarif");
    // Never suggest mounting the host into the container.
    for (const c of [dockerPullCommand(ref), dockerDeviceCommand(ref), dockerCiCommand(ref)]) expect(c).not.toMatch(/--pid=host|-v \/:/);
  });
  it("points at the sample file inside the archive", () => {
    expect(samplePath("fortigate")).toBe("samples/network/fortigate/fortigate-noncompliant.txt");
    expect(samplePath("fortigate", "compliant")).toBe("samples/network/fortigate/fortigate-compliant.txt");
    expect(sampleCommand("paloalto")).toBe("./tmaegis scan --target paloalto --device-config samples/network/paloalto/paloalto-noncompliant.txt");
  });
});

describe("experimental Unix builds", () => {
  it("unpacks with gzip on AIX and Solaris, and runs as root without assuming sudo", () => {
    for (const os of ["aix", "solaris"] as const) {
      const c = archiveCommand(os, `tmaegis_0.1.0_${os}_x.tar.gz`);
      expect(c).toContain(`gzip -dc ../tmaegis_0.1.0_${os}_x.tar.gz | tar -xf -`);
      expect(c).not.toContain("sudo");
      expect(c).not.toContain("tar -xzf");
    }
    expect(archiveCommand("freebsd", "f.tar.gz")).toContain("tar -xzf f.tar.gz -C tmaegis");
    expect(archiveCommand("freebsd", "f.tar.gz")).not.toMatch(/^sudo /m);
  });
  it("cross-compiles with the Go names of the system and processor", () => {
    expect(crossBuildCommand("aix", "ppc64")).toContain("CGO_ENABLED=0 GOOS=aix GOARCH=ppc64 go build -trimpath -o tmaegis .");
    expect(crossBuildCommand("solaris", "x64")).toContain("GOOS=solaris GOARCH=amd64");
    expect(crossBuildCommand("linux", "s390x")).toContain("GOOS=linux GOARCH=s390x");
    expect(crossBuildCommand("freebsd", "arm64")).toMatch(/^git clone https:\/\/github\.com\/tmaegis\/tmaegis\.git\n/);
  });
});
