/**
 * "Test your device" on /downloads: one guide per scan target (`tmaegis scan --target <id>`).
 *
 * A guide says HOW a target is checked (`mode`), WHERE to run TMAegis (`where`, a message key under
 * downloads.devices.where.*) and what to do first (`how`: vendor commands to export the configuration,
 * sign in to a cloud CLI or point kubectl at a cluster). Commands, menu paths and file names are code and
 * are never translated; every sentence around them is a message (downloads.devices.*).
 *
 * Commands are the vendors' documented CLI commands, the same ones the rules themselves name in their
 * descriptions ("Audited against the output of ..."). Where a command or menu differs between versions,
 * `verify: true` makes the card say so.
 *
 * lib/device-guides.test.ts fails when generated/coverage.json has a target without a guide.
 */
import { TARGETS, scanCommand, type CategoryId, type CoverageTargetRow } from "./targets";
import { sampleCommand } from "./install-commands";

export const GUIDE_MODES = ["host", "device-config", "cloud", "k8s", "manual"] as const;
/**
 * - host:          run TMAegis on the machine itself (operating systems, servers, databases, desktop apps)
 * - device-config: export the device's configuration as text, then scan the file on any computer
 * - cloud:         run it where the provider's CLI (or PowerShell module) is signed in with a read-only role
 * - k8s:           run it where kubectl reaches the cluster with a read-only kubeconfig
 * - manual:        nothing can be installed on the target; TMAegis prints a checklist
 */
export type GuideMode = (typeof GUIDE_MODES)[number];

export const WHERE = [
  "linux", "containerOs", "windows", "macos", "unix", "mainframe", "server", "windowsServer", "database", "esxi", "docker",
  "endpoint", "repo", "device", "cloud", "powershell", "saas", "kube", "phone", "mdm", "printer", "storage",
] as const;
export type WhereKey = (typeof WHERE)[number];

/** Extra notes (downloads.devices.notes.*). */
export const NOTES = ["noBuild", "experimental", "containerOs", "ftd", "sophos", "flasharray", "workspace", "m365", "vmware", "mysql", "intune", "readOnly"] as const;
export type NoteKey = (typeof NOTES)[number];

export interface DeviceGuide {
  mode: GuideMode;
  where: WhereKey;
  /** Commands to run first, one per line (code, never translated). Lines starting with # are comments. */
  how?: string[];
  /** Shell the `how` lines are typed into: the device's own CLI, a POSIX shell or PowerShell. */
  howLang?: "cli" | "shell" | "powershell";
  /** A documented menu path in the device's web interface, when that is the way to export (code-like, English UI labels). */
  ui?: string;
  /** Commands or menus differ between versions: the card adds "check your version's documentation". */
  verify?: boolean;
  note?: NoteKey;
}

/** The 17 network targets that ship `samples/network/<id>/<id>-{compliant,noncompliant}.txt` in every archive and in the repository. */
export const NETWORK_SAMPLES = [
  "arista-eos", "aruba-cx", "checkpoint", "cisco-asa", "cisco-ftd", "cisco-ios", "cisco-iosxr", "cisco-nxos", "extreme-slx",
  "f5-bigip", "fortigate", "fortiswitch", "juniper-junos", "opnsense", "paloalto", "pfsense", "sophos",
] as const;

const host = (where: WhereKey, extra: Partial<DeviceGuide> = {}): DeviceGuide => ({ mode: "host", where, ...extra });
const device = (how: string[] | undefined, extra: Partial<DeviceGuide> = {}): DeviceGuide => ({ mode: "device-config", where: "device", how, howLang: "cli", ...extra });
const cloud = (how: string[], extra: Partial<DeviceGuide> = {}): DeviceGuide => ({ mode: "cloud", where: "cloud", how, howLang: "shell", note: "readOnly", ...extra });
const k8s = (how: string[], extra: Partial<DeviceGuide> = {}): DeviceGuide => ({ mode: "k8s", where: "kube", how, howLang: "shell", ...extra });
const manual = (where: WhereKey, extra: Partial<DeviceGuide> = {}): DeviceGuide => ({ mode: "manual", where, ...extra });

const CISCO_PAGED = ["terminal length 0", "show running-config"];
const KUBE = ["kubectl config use-context <context>", "kubectl auth can-i --list   # check that the role is read-only"];

export const DEVICE_GUIDES: Record<string, DeviceGuide> = {
  // Operating systems
  windows: host("windows"),
  linux: host("linux"),
  "azure-container-linux": host("containerOs", { note: "containerOs", verify: true }),
  bottlerocket: host("containerOs", { note: "containerOs", verify: true }),
  cos: host("containerOs", { note: "containerOs", verify: true }),
  macos: host("macos"),
  // Experimental builds are in every release (lib/release-assets.ts EXPERIMENTAL); the card links to them.
  aix: host("unix", { note: "experimental" }),
  solaris: host("unix", { note: "experimental" }),
  freebsd: host("unix", { note: "experimental" }),
  "ibm-i": manual("mainframe"),
  "ibm-z": manual("mainframe"),
  cics: manual("mainframe"),
  "db2-zos": manual("mainframe"),

  // Server software
  apache: host("server"),
  nginx: host("server"),
  iis: host("windowsServer"),
  tomcat: host("server", { how: ["export CATALINA_BASE=/opt/tomcat   # your Tomcat folder (default /opt/tomcat)"], howLang: "shell" }),
  websphere: host("server", { how: ["export WLP_USER_DIR=/opt/ibm/wlp/usr   # your Liberty user directory"], howLang: "shell" }),
  exchange: host("windowsServer"),
  sharepoint: host("windowsServer"),
  bind: host("server"),
  kerberos: host("server"),
  vmware: host("esxi", { note: "vmware", verify: true }),
  ros: host("linux"),

  // Databases (run on the database server with the database's own client)
  cassandra: host("database"),
  cockroachdb: host("database", { how: ["export COCKROACH_CERTS_DIR=/var/lib/cockroach/certs"], howLang: "shell" }),
  db2: host("database"),
  mariadb: host("database", { note: "mysql" }),
  mongodb: host("database"),
  mssql: host("database"),
  mysql: host("database", { note: "mysql" }),
  oceanbase: host("database", { how: ['export OB_CONN="-h127.0.0.1 -P2881 -uroot@sys -p<password>"'], howLang: "shell" }),
  "oracle-db": host("database", { how: ["export ORACLE_HOME=<oracle-home>", "export ORACLE_SID=<sid>"], howLang: "shell" }),
  postgres: host("database", { how: ["export PGUSER=postgres PGDATABASE=postgres   # the defaults"], howLang: "shell" }),
  redis: host("database"),
  tdsql: host("database", { how: ["export TDSQL_HOME=/data/tdsql   # the default"], howLang: "shell" }),
  yugabytedb: host("database", { how: ["export PGHOST=127.0.0.1 PGPORT=5433 PGUSER=yugabyte   # the defaults"], howLang: "shell" }),

  // Containers and Kubernetes
  docker: host("docker"),
  lxd: host("linux"),
  kubernetes: k8s(KUBE),
  aks: k8s(["az login", "az aks get-credentials --resource-group <resource-group> --name <cluster>", "export AKS_RG=<resource-group> AKS_NAME=<cluster>"]),
  "aks-automatic": k8s(["az login", "az aks get-credentials --resource-group <resource-group> --name <cluster>", "export AKS_RG=<resource-group> AKS_NAME=<cluster>"]),
  eks: k8s(["aws sso login --profile <profile>", "aws eks update-kubeconfig --region <region> --name <cluster> --profile <profile>", "export AWS_PROFILE=<profile> EKS_CLUSTER=<cluster>"]),
  "eks-auto": k8s(["aws sso login --profile <profile>", "aws eks update-kubeconfig --region <region> --name <cluster> --profile <profile>", "export AWS_PROFILE=<profile> EKS_CLUSTER=<cluster>"]),
  gke: k8s(["gcloud auth login", "gcloud container clusters get-credentials <cluster> --location <location> --project <project-id>", "export GKE_CLUSTER=<cluster> GKE_LOCATION=<location> GKE_PROJECT=<project-id>"]),
  "gke-autopilot": k8s(["gcloud auth login", "gcloud container clusters get-credentials <cluster> --location <location> --project <project-id>", "export GKE_CLUSTER=<cluster> GKE_LOCATION=<location> GKE_PROJECT=<project-id>"]),
  oke: k8s(["oci ce cluster create-kubeconfig --cluster-id <cluster-ocid> --file $HOME/.kube/config --region <region> --token-version 2.0.0", "export OKE_CLUSTER_ID=<cluster-ocid>"]),
  openshift: k8s(["oc login --server=https://api.<cluster-domain>:6443", "oc whoami"]),
  "openshift-virt": k8s(["oc login --server=https://api.<cluster-domain>:6443", "oc whoami"]),

  // Desktop, browsers and Office (policy checks on each computer)
  chrome: host("endpoint"),
  edge: host("endpoint"),
  firefox: host("endpoint"),
  safari: host("macos"),
  office: host("endpoint"),
  defender: host("endpoint"),
  vscode: host("endpoint"),
  zoom: host("endpoint"),

  // Network devices: export the configuration, then scan the file. TMAegis never connects to the device.
  "cisco-ios": device(CISCO_PAGED),
  "cisco-iosxr": device(CISCO_PAGED),
  "cisco-nxos": device(CISCO_PAGED),
  "cisco-asa": device(["terminal pager 0", "show running-config"]),
  "cisco-ftd": device(["show running-config   # on the FTD CLI (the > prompt)"], { note: "ftd" }),
  "juniper-junos": device(["show configuration | display set | no-more"]),
  "arista-eos": device(CISCO_PAGED),
  "aruba-cx": device(["show running-config"]),
  "extreme-slx": device(CISCO_PAGED),
  checkpoint: device(["# in clish, on each gateway and management server", "set clienv rows 0", "show configuration"]),
  fortigate: device(["config global   # only on units with VDOMs enabled", "show full-configuration"]),
  fortiswitch: device(["show full-configuration"]),
  paloalto: device(["set cli pager off", "set cli config-output-format set", "configure", "show"]),
  "f5-bigip": device(
    [
      "# in the BIG-IP advanced shell (bash)",
      'for c in "auth password-policy" "auth source" "auth remote-user" "auth radius" "sys httpd" "sys sshd" "sys global-settings" "cli global-settings" "sys snmp" "net self"; do tmsh list $c all-properties; done > f5-bigip.txt',
      "tmsh list sys ntp >> f5-bigip.txt",
      "tmsh list sys syslog >> f5-bigip.txt",
    ],
    { howLang: "shell" },
  ),
  pfsense: device(["# or, from the pfSense shell:", "cat /cf/conf/config.xml"], { ui: "Diagnostics > Backup & Restore > Download configuration as XML", howLang: "shell" }),
  opnsense: device(["# or, from the OPNsense shell:", "cat /conf/config.xml"], { ui: "System > Configuration > Backups > Download configuration", howLang: "shell" }),
  sophos: device(undefined, { ui: "Backup & firmware > Import export > Export full configuration", verify: true, note: "sophos" }),
  flasharray: device(
    ['for c in "purearray list --ntp" "purepolicy password list" "purelog syslog list" "puresnmp list" "puresmtp list" "pureds list" "purealert watcher list"; do ssh pureuser@<array> "$c --nvp"; done > flasharray.txt'],
    { where: "storage", howLang: "shell", verify: true, note: "flasharray" },
  ),

  // Cloud and SaaS (read-only)
  aws: cloud(["aws sso login --profile <profile>   # or aws configure", "export AWS_PROFILE=<profile>", "aws sts get-caller-identity"]),
  azure: cloud(["az login", "az account set --subscription <subscription-id>"]),
  gcp: cloud(["gcloud auth login", "gcloud config set project <project-id>"]),
  oci: cloud(["oci setup config   # or: oci session authenticate"]),
  alibaba: cloud(["aliyun configure"]),
  ibmcloud: cloud(["ibmcloud login --sso", "ibmcloud plugin install vpc-infrastructure", "ibmcloud target -r <region>"]),
  m365: cloud(["Connect-MgGraph -Scopes Policy.Read.All,Directory.Read.All", "Connect-ExchangeOnline"], { where: "powershell", howLang: "powershell", note: "m365", verify: true }),
  "power-platform": cloud(["Install-Module Microsoft.PowerApps.Administration.PowerShell -Scope CurrentUser", "Add-PowerAppsAccount", '$env:PP_TENANT_ID = "<tenant-id>"'], { where: "powershell", howLang: "powershell", verify: true }),
  "google-workspace": cloud(["export DOMAIN=example.com   # your primary Google Workspace domain"], { where: "saas", note: "workspace" }),
  github: cloud(["gh auth login", "export GH_ORG=<organization>"]),
  gitlab: cloud(["glab auth login"]),
  "oracle-saas": manual("saas"),

  // Mobile and device management: nothing to install, checklist only (Intune endpoint policies are read on the PC)
  ios: manual("phone"),
  android: manual("phone"),
  "intune-ios": manual("mdm"),
  "intune-macos": manual("mdm"),
  "intune-office": host("endpoint", { note: "intune" }),
  "intune-defender": host("endpoint", { note: "intune" }),

  // DevSecOps and AI tooling
  "supply-chain": host("repo", { how: ["cd <your-repository>", "gh auth login   # only for the checks that read GitHub settings"], howLang: "shell" }),
  "mcp-server": host("server", { how: ["export MCP_CONFIG=/etc/mcp/server.json   # the server's configuration file"], howLang: "shell" }),

  // Printers and storage
  printer: manual("printer"),
};

/** What the page needs for one card. Everything except the message keys is code or a proper noun. */
export interface GuideRow extends DeviceGuide {
  id: string;
  name: string;
  category: CategoryId;
  total: number;
  auto: number;
  manual: number;
  /** Rules that read an exported configuration (--device-config). */
  device: number;
  command: string;
  /** Present for the 17 network targets with sample files. */
  sample?: { path: string; command: string; windowsCommand: string };
}

/** The scan command for a guide: checklist-only targets always list their manual checks. */
export function guideCommand(row: CoverageTargetRow, guide: DeviceGuide): string {
  const cmd = scanCommand(row);
  return guide.mode === "manual" && !cmd.includes("--include-manual") ? `${cmd} --include-manual` : cmd;
}

/** Joins coverage rows with their guide, in display order (mode, then name). Rows without a guide are returned in `missing`. */
export function buildGuideRows(rows: (CoverageTargetRow & { device?: number })[]): { rows: GuideRow[]; missing: string[] } {
  const out: GuideRow[] = [];
  const missing: string[] = [];
  for (const r of rows) {
    const g = DEVICE_GUIDES[r.id];
    const info = TARGETS[r.id];
    if (!g || !info) {
      missing.push(r.id);
      continue;
    }
    const hasSample = (NETWORK_SAMPLES as readonly string[]).includes(r.id);
    out.push({
      ...g,
      id: r.id,
      name: info.name,
      category: info.category,
      total: r.count,
      auto: r.auto,
      manual: r.manual,
      device: r.device ?? 0,
      command: guideCommand(r, g),
      ...(hasSample ? { sample: { path: `samples/network/${r.id}`, command: sampleCommand(r.id), windowsCommand: sampleCommand(r.id, "windows") } } : {}),
    });
  }
  const order = (m: GuideMode) => GUIDE_MODES.indexOf(m);
  out.sort((a, b) => order(a.mode) - order(b.mode) || a.name.localeCompare(b.name, "en"));
  return { rows: out, missing };
}
