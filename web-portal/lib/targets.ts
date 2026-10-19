/**
 * Where every scan target (`tmaegis scan --target <id>`) appears in the "What it hardens" browser on /downloads.
 * Product names are proper nouns, so they are not translated; category and group titles are messages
 * (downloads.hardens.categories.* and downloads.hardens.groups.*).
 *
 * lib/targets.test.ts fails when the generated rule data contains a target that is not listed here.
 */
export const CATEGORIES = ["os", "server", "database", "containers", "desktop", "network", "cloud", "mobile", "devsecops", "ai", "hardware"] as const;
export type CategoryId = (typeof CATEGORIES)[number];

/** Groups inside a category, in display order. */
export const GROUPS: Record<CategoryId, readonly string[]> = {
  os: ["windows", "linux", "macos", "unix", "mainframe"],
  server: ["web", "identity", "virtualization", "other"],
  database: ["databases"],
  containers: ["runtime", "kubernetes"],
  desktop: ["browsers", "apps"],
  network: ["firewalls", "switching", "adc"],
  cloud: ["iaas", "saas"],
  mobile: ["devices", "mdm"],
  devsecops: ["pipeline"],
  ai: ["agents"],
  hardware: ["printers", "storage"],
};

export interface TargetInfo {
  name: string;
  category: CategoryId;
  group: string;
}

const t = (name: string, category: CategoryId, group: string): TargetInfo => ({ name, category, group });

export const TARGETS: Record<string, TargetInfo> = {
  // Operating systems
  windows: t("Windows (desktop and server)", "os", "windows"),
  linux: t("Linux distributions", "os", "linux"),
  "azure-container-linux": t("Azure Container Linux", "os", "linux"),
  bottlerocket: t("Bottlerocket", "os", "linux"),
  cos: t("Container-Optimized OS", "os", "linux"),
  macos: t("macOS", "os", "macos"),
  aix: t("IBM AIX", "os", "unix"),
  solaris: t("Oracle Solaris", "os", "unix"),
  freebsd: t("FreeBSD", "os", "unix"),
  "ibm-i": t("IBM i", "os", "mainframe"),
  "ibm-z": t("IBM Z (z/OS)", "os", "mainframe"),
  cics: t("IBM CICS", "os", "mainframe"),
  "db2-zos": t("Db2 for z/OS", "os", "mainframe"),

  // Server software
  apache: t("Apache HTTP Server", "server", "web"),
  nginx: t("NGINX", "server", "web"),
  iis: t("Microsoft IIS", "server", "web"),
  tomcat: t("Apache Tomcat", "server", "web"),
  websphere: t("IBM WebSphere", "server", "web"),
  exchange: t("Microsoft Exchange Server", "server", "web"),
  sharepoint: t("Microsoft SharePoint Server", "server", "web"),
  cassandra: t("Apache Cassandra", "database", "databases"),
  cockroachdb: t("CockroachDB", "database", "databases"),
  db2: t("IBM Db2", "database", "databases"),
  mariadb: t("MariaDB", "database", "databases"),
  mongodb: t("MongoDB", "database", "databases"),
  mssql: t("Microsoft SQL Server", "database", "databases"),
  mysql: t("MySQL", "database", "databases"),
  oceanbase: t("OceanBase", "database", "databases"),
  "oracle-db": t("Oracle Database", "database", "databases"),
  postgres: t("PostgreSQL", "database", "databases"),
  redis: t("Redis", "database", "databases"),
  tdsql: t("TDSQL", "database", "databases"),
  yugabytedb: t("YugabyteDB", "database", "databases"),
  bind: t("BIND DNS", "server", "identity"),
  kerberos: t("Kerberos", "server", "identity"),
  vmware: t("VMware ESXi", "server", "virtualization"),
  docker: t("Docker", "containers", "runtime"),
  lxd: t("LXD", "containers", "runtime"),
  kubernetes: t("Kubernetes", "containers", "kubernetes"),
  aks: t("Azure Kubernetes Service (AKS)", "containers", "kubernetes"),
  "aks-automatic": t("AKS Automatic", "containers", "kubernetes"),
  eks: t("Amazon EKS", "containers", "kubernetes"),
  "eks-auto": t("Amazon EKS Auto Mode", "containers", "kubernetes"),
  gke: t("Google Kubernetes Engine (GKE)", "containers", "kubernetes"),
  "gke-autopilot": t("GKE Autopilot", "containers", "kubernetes"),
  oke: t("Oracle Kubernetes Engine (OKE)", "containers", "kubernetes"),
  openshift: t("Red Hat OpenShift", "containers", "kubernetes"),
  "openshift-virt": t("OpenShift Virtualization", "containers", "kubernetes"),
  ros: t("ROS (Robot Operating System)", "server", "other"),

  // Desktop, browsers, Office
  chrome: t("Google Chrome", "desktop", "browsers"),
  edge: t("Microsoft Edge", "desktop", "browsers"),
  firefox: t("Mozilla Firefox", "desktop", "browsers"),
  safari: t("Apple Safari", "desktop", "browsers"),
  office: t("Microsoft Office", "desktop", "apps"),
  defender: t("Microsoft Defender Antivirus", "desktop", "apps"),
  vscode: t("Visual Studio Code", "desktop", "apps"),
  zoom: t("Zoom", "desktop", "apps"),

  // Network devices (audited from an exported configuration with --device-config)
  "cisco-asa": t("Cisco ASA", "network", "firewalls"),
  "cisco-ftd": t("Cisco Secure Firewall (FTD)", "network", "firewalls"),
  paloalto: t("Palo Alto Networks", "network", "firewalls"),
  fortigate: t("Fortinet FortiGate", "network", "firewalls"),
  checkpoint: t("Check Point", "network", "firewalls"),
  pfsense: t("pfSense", "network", "firewalls"),
  opnsense: t("OPNsense", "network", "firewalls"),
  sophos: t("Sophos Firewall", "network", "firewalls"),
  "cisco-ios": t("Cisco IOS", "network", "switching"),
  "cisco-iosxr": t("Cisco IOS XR", "network", "switching"),
  "cisco-nxos": t("Cisco NX-OS", "network", "switching"),
  "juniper-junos": t("Juniper Junos", "network", "switching"),
  "arista-eos": t("Arista EOS", "network", "switching"),
  "aruba-cx": t("HPE Aruba CX", "network", "switching"),
  "extreme-slx": t("Extreme SLX-OS", "network", "switching"),
  fortiswitch: t("Fortinet FortiSwitch", "network", "switching"),
  "f5-bigip": t("F5 BIG-IP", "network", "adc"),

  // Cloud and SaaS
  aws: t("Amazon Web Services", "cloud", "iaas"),
  azure: t("Microsoft Azure", "cloud", "iaas"),
  gcp: t("Google Cloud", "cloud", "iaas"),
  oci: t("Oracle Cloud Infrastructure", "cloud", "iaas"),
  alibaba: t("Alibaba Cloud", "cloud", "iaas"),
  ibmcloud: t("IBM Cloud", "cloud", "iaas"),
  m365: t("Microsoft 365", "cloud", "saas"),
  "google-workspace": t("Google Workspace", "cloud", "saas"),
  github: t("GitHub", "cloud", "saas"),
  "oracle-saas": t("Oracle Fusion Cloud (SaaS)", "cloud", "saas"),
  "power-platform": t("Microsoft Power Platform", "cloud", "saas"),

  // Mobile and device management
  ios: t("Apple iOS", "mobile", "devices"),
  android: t("Google Android", "mobile", "devices"),
  "intune-ios": t("Intune: iOS/iPadOS", "mobile", "mdm"),
  "intune-macos": t("Intune: macOS", "mobile", "mdm"),
  "intune-office": t("Intune: Microsoft Office", "mobile", "mdm"),
  "intune-defender": t("Intune: Microsoft Defender", "mobile", "mdm"),

  // DevSecOps and supply chain
  gitlab: t("GitLab", "devsecops", "pipeline"),
  "supply-chain": t("Software supply chain", "devsecops", "pipeline"),
  "mcp-server": t("MCP servers (AI tooling)", "ai", "agents"),

  // Printers and storage
  printer: t("Network printers", "hardware", "printers"),
  flasharray: t("Pure Storage FlashArray", "hardware", "storage"),
};

/** Category of a target, or null when the target is not in the table (a bug the unit test catches). */
export function categoryOf(target: string): CategoryId | null {
  return TARGETS[target]?.category ?? null;
}

export interface CoverageTargetRow {
  id: string;
  count: number;
  manual: number;
  auto: number;
  device?: number;
}

export interface GroupedTargets {
  category: CategoryId;
  rules: number;
  targets: number;
  groups: { id: string; rows: (CoverageTargetRow & { name: string })[] }[];
}

/** Groups coverage rows by category and group. Unlisted targets are returned in `unmapped`. */
export function groupTargets(rows: CoverageTargetRow[]): { categories: GroupedTargets[]; unmapped: CoverageTargetRow[] } {
  const unmapped: CoverageTargetRow[] = [];
  const byCat = new Map<CategoryId, Map<string, (CoverageTargetRow & { name: string })[]>>();
  for (const r of rows) {
    const info = TARGETS[r.id];
    if (!info) {
      unmapped.push(r);
      continue;
    }
    const groups = byCat.get(info.category) ?? new Map();
    const list = groups.get(info.group) ?? [];
    list.push({ ...r, name: info.name });
    groups.set(info.group, list);
    byCat.set(info.category, groups);
  }
  const categories: GroupedTargets[] = [];
  for (const category of CATEGORIES) {
    const groups = byCat.get(category);
    if (!groups) continue;
    const ordered = GROUPS[category].flatMap((g) => (groups.has(g) ? [{ id: g, rows: groups.get(g)!.sort((a, b) => a.name.localeCompare(b.name, "en")) }] : []));
    const flat = ordered.flatMap((g) => g.rows);
    categories.push({ category, rules: flat.reduce((n, r) => n + r.count, 0), targets: flat.length, groups: ordered });
  }
  return { categories, unmapped };
}

/** The scan command for a target, with the extra flags that apply to it. */
export function scanCommand(row: CoverageTargetRow): string {
  let cmd = `tmaegis scan --target ${row.id}`;
  if ((row.device ?? 0) > 0) cmd += " --device-config <file>";
  if (row.auto === 0) cmd += " --include-manual";
  return cmd;
}
