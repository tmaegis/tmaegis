// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package discovery detects the host OS and the hardening-relevant software installed on it,
// so `scan --auto-detect` only runs rules that apply to this machine.
package discovery

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Host describes what was detected.
type Host struct {
	OS       string   `json:"os"`       // linux, windows, darwin
	Distro   string   `json:"distro"`   // ubuntu, rocky, ...
	Version  string   `json:"version"`  // 22.04
	Family   string   `json:"family"`   // distro-version, e.g. ubuntu-22.04; matches rule os_family
	Software []string `json:"software"` // rule targets present on the host (nginx, docker, ...)
}

// Targets returns every rule target that applies to this host: its OS plus detected software.
func (h Host) Targets() []string {
	osTarget := h.OS
	if osTarget == "darwin" {
		osTarget = "macos" // rules use the product name
	}
	targets := append([]string{osTarget}, h.Software...)
	if h.Distro == "flatcar" {
		targets = append(targets, "azure-container-linux") // Azure Container Linux is Flatcar-based
	}
	if h.Distro == "cos" {
		targets = append(targets, "cos") // Google Container-Optimized OS
	}
	return targets
}

// probes maps a rule target to the binaries (looked up on PATH) or install paths (environment
// variables expanded, e.g. %ProgramFiles% on Windows) whose presence implies it is installed.
var probes = []struct {
	target string
	bins   []string
	paths  []string
}{
	{"nginx", []string{"nginx"}, nil},
	{"apache", []string{"apache2", "httpd"}, nil},
	{"tomcat", []string{"catalina.sh", "tomcat", "tomcat10", "tomcat11"}, []string{"/opt/tomcat/bin/catalina.sh", "/usr/share/tomcat/bin/catalina.sh", "/usr/share/tomcat10/bin/catalina.sh"}},
	{"iis", nil, []string{"${windir}/System32/inetsrv/appcmd.exe"}},
	{"websphere", []string{"wsadmin.sh"}, []string{"/opt/IBM/WebSphere/AppServer/bin/wsadmin.sh", "/opt/ibm/wlp/bin/server", "/opt/IBM/WebSphere/Liberty/bin/server", "/opt/ol/wlp/bin/server"}},
	{"docker", []string{"docker", "dockerd"}, nil},
	{"kubernetes", []string{"kubectl", "kubelet"}, nil},
	{"lxd", []string{"lxd", "lxc"}, nil},
	{"postgres", []string{"psql", "postgres"}, nil},
	{"mysql", []string{"mysqld", "mysql"}, nil},
	{"mariadb", []string{"mariadbd", "mariadb"}, nil},
	{"mongodb", []string{"mongod", "mongosh"}, nil},
	{"redis", []string{"redis-server", "redis-cli"}, nil},
	{"cockroachdb", []string{"cockroach"}, nil},
	{"yugabytedb", []string{"yugabyted", "yb-tserver", "ysqlsh"}, nil},
	{"oceanbase", []string{"observer", "obd"}, nil},
	{"openshift", []string{"oc"}, nil},
	{"vscode", []string{"code"}, []string{"${LOCALAPPDATA}/Programs/Microsoft VS Code/Code.exe", "${ProgramFiles}/Microsoft VS Code/Code.exe"}},
	{"defender", nil, []string{"${ProgramFiles}/Windows Defender/MpCmdRun.exe"}},
	{"mssql", []string{"sqlcmd", "sqlservr"}, nil},
	{"cassandra", []string{"cassandra", "nodetool"}, nil},
	{"oracle-db", []string{"sqlplus", "tnslsnr"}, nil},
	{"db2", []string{"db2", "db2level"}, nil},
	{"bind", []string{"named"}, nil},
	{"kerberos", []string{"krb5kdc", "kadmind"}, nil},
	{"exchange", nil, []string{"${ExchangeInstallPath}bin/ExSetup.exe"}},
	{"sharepoint", nil, []string{"${CommonProgramFiles}/microsoft shared/Web Server Extensions/16/BIN/psconfig.exe"}},
	{"chrome", []string{"google-chrome", "google-chrome-stable"}, []string{"${ProgramFiles}/Google/Chrome/Application/chrome.exe", "/Applications/Google Chrome.app"}},
	{"edge", []string{"microsoft-edge"}, []string{"${ProgramFiles(x86)}/Microsoft/Edge/Application/msedge.exe", "/Applications/Microsoft Edge.app"}},
	{"firefox", []string{"firefox"}, []string{"${ProgramFiles}/Mozilla Firefox/firefox.exe", "/Applications/Firefox.app"}},
	{"safari", nil, []string{"/Applications/Safari.app"}},
	{"office", nil, []string{"${ProgramFiles}/Microsoft Office/root/Office16/WINWORD.EXE", "/Applications/Microsoft Word.app"}},
	{"zoom", nil, []string{"${APPDATA}/Zoom/bin/Zoom.exe", "${ProgramFiles}/Zoom/bin/Zoom.exe", "/Applications/zoom.us.app"}},
	{"vmware", []string{"esxcli", "vmware-toolbox-cmd"}, nil},
	{"ros", []string{"ros2", "roscore"}, nil},
	{"bottlerocket", []string{"apiclient"}, []string{"/.bottlerocket"}},
	{"aws", []string{"aws"}, nil},
	{"azure", []string{"az"}, nil},
	{"gcp", []string{"gcloud"}, nil},
	{"alibaba", []string{"aliyun"}, nil},
	{"oci", []string{"oci"}, nil},
	{"ibmcloud", []string{"ibmcloud"}, nil},
}

// Detect inspects the current machine.
func Detect() Host {
	h := Host{OS: runtime.GOOS}
	if runtime.GOOS == "linux" {
		if f, err := os.Open("/etc/os-release"); err == nil {
			defer f.Close()
			h.Distro, h.Version = ParseOSRelease(f)
			h.Family = Family(h.Distro, h.Version)
		}
	}
	if runtime.GOOS == "freebsd" {
		if out, err := exec.Command("freebsd-version", "-u").Output(); err == nil {
			h.Distro, h.Version = "freebsd", strings.TrimSpace(string(out)) // 14.1-RELEASE
			major, _, _ := strings.Cut(h.Version, ".")
			h.Family = "freebsd-" + major
		}
	}
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			h.Distro, h.Version = "macos", strings.TrimSpace(string(out))
			h.Family = Family(h.Distro, h.Version) // macos-15
		}
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command", windowsProbe).Output()
		if err == nil {
			h.Distro, h.Version, h.Family = ParseWindows(string(out))
		}
	}
	h.Software = detectSoftware(exec.LookPath)
	return h
}

// windowsProbe prints "InstallationType|CurrentBuild|DomainRole".
const windowsProbe = `$cv = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion'; ` +
	`$role = (Get-CimInstance Win32_ComputerSystem).DomainRole; "$($cv.InstallationType)|$($cv.CurrentBuild)|$role"`

// windowsServerBuilds maps the base build of each Windows Server release to its name.
var windowsServerBuilds = map[string]string{"14393": "2016", "17763": "2019", "20348": "2022", "26100": "2025"}

// ParseWindows turns the probe output into distro, version and the os_family used by CIS
// Windows rules: windows-server-<year>-dc on domain controllers (DomainRole 4 or 5) and
// windows-server-<year>-ms on member and standalone servers (the CIS "Member Server" profile).
func ParseWindows(probe string) (distro, version, family string) {
	parts := strings.Split(strings.TrimSpace(probe), "|")
	if len(parts) != 3 {
		return "", "", ""
	}
	instType, build, role := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	if !strings.HasPrefix(instType, "Server") {
		// CIS publishes one desktop benchmark per major release; Windows 11 starts at build 22000.
		major := "windows-10"
		if n, err := strconv.Atoi(build); err == nil && n >= 22000 {
			major = "windows-11"
		}
		return "windows-client", build, major
	}
	year, ok := windowsServerBuilds[build]
	if !ok {
		return "windows-server", build, "windows-server-" + build
	}
	suffix := "ms"
	if role == "4" || role == "5" {
		suffix = "dc"
	}
	return "windows-server", year, "windows-server-" + year + "-" + suffix
}

func detectSoftware(lookPath func(string) (string, error)) []string {
	return detectSoftwareWith(lookPath, func(p string) bool { _, err := os.Stat(p); return err == nil })
}

func detectSoftwareWith(lookPath func(string) (string, error), exists func(string) bool) []string {
	var found []string
	for _, p := range probes {
		if hasAny(p.bins, lookPath) || hasPath(p.paths, exists) {
			found = append(found, p.target)
		}
	}
	return found
}

func hasAny(bins []string, lookPath func(string) (string, error)) bool {
	for _, bin := range bins {
		if _, err := lookPath(bin); err == nil {
			return true
		}
	}
	return false
}

// hasPath expands ${VAR} and skips paths whose variables are unset, so a Windows path never
// resolves to a bogus relative path on Linux.
func hasPath(paths []string, exists func(string) bool) bool {
	for _, p := range paths {
		missing := false
		expanded := os.Expand(p, func(k string) string {
			v := os.Getenv(k)
			if v == "" {
				missing = true
			}
			return v
		})
		if !missing && exists(filepath.FromSlash(expanded)) {
			return true
		}
	}
	return false
}

// ParseOSRelease extracts ID and VERSION_ID from /etc/os-release content.
func ParseOSRelease(f io.Reader) (id, version string) {
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		switch k {
		case "ID":
			id = strings.ToLower(v)
		case "VERSION_ID":
			version = v
		}
	}
	return id, version
}

// Family builds the identifier used in rule `os_family`. RHEL-like distros keep only
// the major version (rocky-9.3 -> rocky-9) because benchmarks are per major release.
func Family(distro, version string) string {
	if distro == "" {
		return ""
	}
	if distro == "mariner" {
		distro = "azurelinux" // CBL-Mariner 2.0 is Azure Linux 2
	}
	switch distro {
	case "rocky", "almalinux", "rhel", "centos", "ol", "sles", "sled", "opensuse-leap", "alinux", "macos", "azurelinux", "linuxmint":
		version, _, _ = strings.Cut(version, ".")
	}
	if version == "" {
		return distro
	}
	return distro + "-" + version
}
