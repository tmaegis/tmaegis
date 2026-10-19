// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"net/http"
	"sort"
	"strings"

	"github.com/tmaegis/tmaegis/internal/rules"
)

// The platform catalogue mirrors web-portal/lib/targets.ts so the GUI groups targets exactly
// like the public website. Product names are proper nouns and are not translated; category
// and group titles are gui.cat.* and gui.group.* messages. A rule target missing here is
// listed under the "other" category instead of being hidden.

// Categories in display order.
var categories = []string{"os", "server", "database", "containers", "desktop", "network", "cloud", "mobile", "devsecops", "ai", "hardware"}

// Groups inside each category, in display order.
var categoryGroups = map[string][]string{
	"os":         {"windows", "linux", "macos", "unix", "mainframe"},
	"server":     {"web", "identity", "virtualization", "other"},
	"database":   {"databases"},
	"containers": {"runtime", "kubernetes"},
	"desktop":    {"browsers", "apps"},
	"network":    {"firewalls", "switching", "adc"},
	"cloud":      {"iaas", "saas"},
	"mobile":     {"devices", "mdm"},
	"devsecops":  {"pipeline"},
	"ai":         {"agents"},
	"hardware":   {"printers", "storage"},
	"other":      {"other"},
}

type platformMeta struct{ name, category, group string }

var platformCatalog = map[string]platformMeta{
	"windows": {"Windows (desktop and server)", "os", "windows"}, "linux": {"Linux distributions", "os", "linux"},
	"azure-container-linux": {"Azure Container Linux", "os", "linux"}, "bottlerocket": {"Bottlerocket", "os", "linux"},
	"cos": {"Container-Optimized OS", "os", "linux"}, "macos": {"macOS", "os", "macos"}, "aix": {"IBM AIX", "os", "unix"},
	"solaris": {"Oracle Solaris", "os", "unix"}, "freebsd": {"FreeBSD", "os", "unix"}, "ibm-i": {"IBM i", "os", "mainframe"},
	"ibm-z": {"IBM Z (z/OS)", "os", "mainframe"}, "cics": {"IBM CICS", "os", "mainframe"}, "db2-zos": {"Db2 for z/OS", "os", "mainframe"},

	"apache": {"Apache HTTP Server", "server", "web"}, "nginx": {"NGINX", "server", "web"}, "iis": {"Microsoft IIS", "server", "web"},
	"tomcat": {"Apache Tomcat", "server", "web"}, "websphere": {"IBM WebSphere", "server", "web"},
	"exchange": {"Microsoft Exchange Server", "server", "web"}, "sharepoint": {"Microsoft SharePoint Server", "server", "web"},
	"bind": {"BIND DNS", "server", "identity"}, "kerberos": {"Kerberos", "server", "identity"},
	"vmware": {"VMware ESXi", "server", "virtualization"}, "ros": {"ROS (Robot Operating System)", "server", "other"},

	"cassandra": {"Apache Cassandra", "database", "databases"}, "cockroachdb": {"CockroachDB", "database", "databases"},
	"db2": {"IBM Db2", "database", "databases"}, "mariadb": {"MariaDB", "database", "databases"},
	"mongodb": {"MongoDB", "database", "databases"}, "mssql": {"Microsoft SQL Server", "database", "databases"},
	"mysql": {"MySQL", "database", "databases"}, "oceanbase": {"OceanBase", "database", "databases"},
	"oracle-db": {"Oracle Database", "database", "databases"}, "postgres": {"PostgreSQL", "database", "databases"},
	"redis": {"Redis", "database", "databases"}, "tdsql": {"TDSQL", "database", "databases"},
	"yugabytedb": {"YugabyteDB", "database", "databases"},

	"docker": {"Docker", "containers", "runtime"}, "lxd": {"LXD", "containers", "runtime"},
	"kubernetes": {"Kubernetes", "containers", "kubernetes"}, "aks": {"Azure Kubernetes Service (AKS)", "containers", "kubernetes"},
	"aks-automatic": {"AKS Automatic", "containers", "kubernetes"}, "eks": {"Amazon EKS", "containers", "kubernetes"},
	"eks-auto": {"Amazon EKS Auto Mode", "containers", "kubernetes"}, "gke": {"Google Kubernetes Engine (GKE)", "containers", "kubernetes"},
	"gke-autopilot": {"GKE Autopilot", "containers", "kubernetes"}, "oke": {"Oracle Kubernetes Engine (OKE)", "containers", "kubernetes"},
	"openshift": {"Red Hat OpenShift", "containers", "kubernetes"}, "openshift-virt": {"OpenShift Virtualization", "containers", "kubernetes"},

	"chrome": {"Google Chrome", "desktop", "browsers"}, "edge": {"Microsoft Edge", "desktop", "browsers"},
	"firefox": {"Mozilla Firefox", "desktop", "browsers"}, "safari": {"Apple Safari", "desktop", "browsers"},
	"office": {"Microsoft Office", "desktop", "apps"}, "defender": {"Microsoft Defender Antivirus", "desktop", "apps"},
	"vscode": {"Visual Studio Code", "desktop", "apps"}, "zoom": {"Zoom", "desktop", "apps"},

	"cisco-asa": {"Cisco ASA", "network", "firewalls"}, "cisco-ftd": {"Cisco Secure Firewall (FTD)", "network", "firewalls"},
	"paloalto": {"Palo Alto Networks", "network", "firewalls"}, "fortigate": {"Fortinet FortiGate", "network", "firewalls"},
	"checkpoint": {"Check Point", "network", "firewalls"}, "pfsense": {"pfSense", "network", "firewalls"},
	"opnsense": {"OPNsense", "network", "firewalls"}, "sophos": {"Sophos Firewall", "network", "firewalls"},
	"cisco-ios": {"Cisco IOS", "network", "switching"}, "cisco-iosxr": {"Cisco IOS XR", "network", "switching"},
	"cisco-nxos": {"Cisco NX-OS", "network", "switching"}, "juniper-junos": {"Juniper Junos", "network", "switching"},
	"arista-eos": {"Arista EOS", "network", "switching"}, "aruba-cx": {"HPE Aruba CX", "network", "switching"},
	"extreme-slx": {"Extreme SLX-OS", "network", "switching"}, "fortiswitch": {"Fortinet FortiSwitch", "network", "switching"},
	"f5-bigip": {"F5 BIG-IP", "network", "adc"},

	"aws": {"Amazon Web Services", "cloud", "iaas"}, "azure": {"Microsoft Azure", "cloud", "iaas"},
	"gcp": {"Google Cloud", "cloud", "iaas"}, "oci": {"Oracle Cloud Infrastructure", "cloud", "iaas"},
	"alibaba": {"Alibaba Cloud", "cloud", "iaas"}, "ibmcloud": {"IBM Cloud", "cloud", "iaas"},
	"m365": {"Microsoft 365", "cloud", "saas"}, "google-workspace": {"Google Workspace", "cloud", "saas"},
	"github": {"GitHub", "cloud", "saas"}, "oracle-saas": {"Oracle Fusion Cloud (SaaS)", "cloud", "saas"},
	"power-platform": {"Microsoft Power Platform", "cloud", "saas"},

	"ios": {"Apple iOS", "mobile", "devices"}, "android": {"Google Android", "mobile", "devices"},
	"intune-ios": {"Intune: iOS/iPadOS", "mobile", "mdm"}, "intune-macos": {"Intune: macOS", "mobile", "mdm"},
	"intune-office": {"Intune: Microsoft Office", "mobile", "mdm"}, "intune-defender": {"Intune: Microsoft Defender", "mobile", "mdm"},

	"gitlab": {"GitLab", "devsecops", "pipeline"}, "supply-chain": {"Software supply chain", "devsecops", "pipeline"},
	"mcp-server": {"MCP servers (AI tooling)", "ai", "agents"},
	"printer":    {"Network printers", "hardware", "printers"}, "flasharray": {"Pure Storage FlashArray", "hardware", "storage"},
}

// metaOf returns the catalogue entry of a target, falling back to the "other" category.
func metaOf(target string) platformMeta {
	if m, ok := platformCatalog[target]; ok {
		return m
	}
	return platformMeta{name: target, category: "other", group: "other"}
}

// TargetStats are the rule counts of one target (all levels, every audit type).
type TargetStats struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Category string         `json:"category"`
	Group    string         `json:"group"`
	Total    int            `json:"total"`
	Level1   int            `json:"level1"`
	Auto     int            `json:"auto"`   // audited automatically on the host
	Manual   int            `json:"manual"` // manual review
	Device   int            `json:"device"` // config_regex: needs an exported device configuration
	Fixable  int            `json:"fixable"`
	Severity map[string]int `json:"severity"`
	Detected bool           `json:"detected,omitempty"`
}

func statsOf(all []rules.Rule) map[string]*TargetStats {
	out := map[string]*TargetStats{}
	for _, r := range all {
		id := strings.ToLower(r.Target)
		st := out[id]
		if st == nil {
			m := metaOf(id)
			st = &TargetStats{ID: id, Name: m.name, Category: m.category, Group: m.group, Severity: map[string]int{}}
			out[id] = st
		}
		st.Total++
		if r.Level <= 1 {
			st.Level1++
		}
		switch r.Audit.Type {
		case "manual":
			st.Manual++
		case "config_regex":
			st.Device++
		default:
			st.Auto++
		}
		if r.Remediation.Type != "" {
			st.Fixable++
		}
		st.Severity[strings.ToLower(r.Severity)]++
	}
	return out
}

// targetStats loads (once per rules directory) the per-target statistics.
func (s *Server) targetStats(dir string) (map[string]*TargetStats, error) {
	s.mu.Lock()
	cached, ok := s.stats[dir]
	s.mu.Unlock()
	if ok {
		return cached, nil
	}
	all, err := s.loadAll(dir, false)
	if err != nil {
		return nil, err
	}
	st := statsOf(all)
	s.mu.Lock()
	s.stats[dir] = st
	s.mu.Unlock()
	return st, nil
}

type platformGroup struct {
	ID      string         `json:"id"`
	Targets []*TargetStats `json:"targets"`
}

type platformCategory struct {
	ID      string          `json:"id"`
	Rules   int             `json:"rules"`
	Groups  []platformGroup `json:"groups"`
	Targets int             `json:"targets"`
}

// groupPlatforms arranges target statistics by category and group in display order.
func groupPlatforms(st map[string]*TargetStats, detected map[string]bool) []platformCategory {
	byCat := map[string]map[string][]*TargetStats{}
	for _, t := range st {
		c := *t
		c.Detected = detected[t.ID]
		if byCat[c.Category] == nil {
			byCat[c.Category] = map[string][]*TargetStats{}
		}
		byCat[c.Category][c.Group] = append(byCat[c.Category][c.Group], &c)
	}
	out := []platformCategory{}
	for _, cat := range append(append([]string{}, categories...), "other") {
		groups := byCat[cat]
		if groups == nil {
			continue
		}
		pc := platformCategory{ID: cat}
		for _, g := range categoryGroups[cat] {
			list := groups[g]
			if len(list) == 0 {
				continue
			}
			sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
			for _, t := range list {
				pc.Rules += t.Total
			}
			pc.Targets += len(list)
			pc.Groups = append(pc.Groups, platformGroup{ID: g, Targets: list})
		}
		out = append(out, pc)
	}
	return out
}

func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("rules_dir")
	if dir == "" {
		dir = s.rulesDir()
	}
	if !isDir(dir) {
		writeErr(w, http.StatusBadRequest, "invalid_dir", "rules directory not found: "+dir)
		return
	}
	st, err := s.targetStats(dir)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "rules_load_failed", err.Error())
		return
	}
	detected := map[string]bool{}
	for _, t := range s.detectHost(false).Targets() {
		detected[t] = true
	}
	total := 0
	for _, t := range st {
		total += t.Total
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": groupPlatforms(st, detected), "rules": total, "targets": len(st), "rules_dir": dir})
}
