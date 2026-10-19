/**
 * The home page platform grid. Product names are proper nouns, so they are not translated.
 * Each item is [visible name, icon id in lib/platform-icons.ts (a target id or a system name)].
 */
export type PlatformItem = readonly [name: string, icon: string];

export const PLATFORM_GROUPS: { id: "os" | "cloud" | "databases" | "web" | "containers" | "network" | "apps"; items: PlatformItem[] }[] = [
  {
    id: "os",
    items: [["Ubuntu", "ubuntu"], ["Debian", "debian"], ["RHEL", "rhel"], ["Rocky", "rocky"], ["Windows Server", "windows"], ["macOS", "macos"], ["FreeBSD", "freebsd"], ["Solaris", "solaris"], ["AIX", "aix"], ["IBM i", "ibm-i"], ["IBM Z", "ibm-z"], ["COS", "cos"]],
  },
  {
    id: "cloud",
    items: [["AWS", "aws"], ["Azure", "azure"], ["AWS", "aws"], ["Microsoft Azure", "azure"], ["Google Cloud", "gcp"], ["Oracle Cloud", "oci"], ["Alibaba Cloud", "alibaba"], ["IBM Cloud", "ibmcloud"], ["Microsoft 365", "m365"], ["Google Workspace", "google-workspace"], ["Power Platform", "power-platform"]],
  },
  {
    id: "databases",
    items: [["PostgreSQL", "postgres"], ["MySQL", "mysql"], ["MariaDB", "mariadb"], ["MongoDB", "mongodb"], ["SQL Server", "mssql"], ["Oracle Database", "oracle-db"], ["Db2", "db2"], ["Redis", "redis"], ["Cassandra", "cassandra"]],
  },
  { id: "web", items: [["NGINX", "nginx"], ["Apache HTTP Server", "apache"], ["Tomcat", "tomcat"], ["IIS", "iis"], ["WebSphere", "websphere"]] },
  { id: "containers", items: [["Docker", "docker"], ["Kubernetes", "kubernetes"], ["EKS", "eks"], ["AKS", "aks"], ["GKE", "gke"], ["OpenShift", "openshift"]] },
  {
    id: "network",
    items: [["Cisco", "cisco-ios"], ["Juniper", "juniper-junos"], ["Palo Alto", "paloalto"], ["Fortinet", "fortigate"], ["F5", "f5-bigip"], ["Check Point", "checkpoint"], ["pfSense", "pfsense"], ["OPNsense", "opnsense"]],
  },
  {
    id: "apps",
    items: [["Chrome", "chrome"], ["Edge", "edge"], ["Firefox", "firefox"], ["Microsoft Office", "office"], ["Zoom", "zoom"], ["iOS", "ios"], ["Android", "android"], ["VS Code", "vscode"], ["GitLab", "gitlab"], ["MCP Server", "mcp-server"]],
  },
];

/** The "works with" band: logos only (the names are in the grid below it), one per brand. */
export const WORKS_WITH: PlatformItem[] = [
  ["Windows", "windows"], ["Linux", "linux"], ["Ubuntu", "ubuntu"], ["Red Hat", "rhel"], ["Debian", "debian"], ["SUSE", "suse"], ["macOS", "macos"], ["FreeBSD", "freebsd"],
  ["Docker", "docker"], ["Kubernetes", "kubernetes"], ["OpenShift", "openshift"], ["AWS", "aws"], ["Microsoft Azure", "azure"], ["Google Cloud", "gcp"], ["Alibaba Cloud", "alibaba"], ["GitHub", "github"], ["GitLab", "gitlab"],
  ["PostgreSQL", "postgres"], ["MySQL", "mysql"], ["MariaDB", "mariadb"], ["MongoDB", "mongodb"], ["Redis", "redis"], ["NGINX", "nginx"], ["Apache", "apache"],
  ["Cisco", "cisco-ios"], ["Juniper", "juniper-junos"], ["Palo Alto Networks", "paloalto"], ["Fortinet", "fortigate"], ["F5", "f5-bigip"], ["VMware", "vmware"], ["Chrome", "chrome"], ["Firefox", "firefox"], ["Android", "android"],
];
