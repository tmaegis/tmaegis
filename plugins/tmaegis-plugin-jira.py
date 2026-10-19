#!/usr/bin/env python3
"""TMAegis ITSM plugin: opens one Jira issue per failed critical/high control.

Reads the JSON ScanReport on stdin. Config via environment:
  JIRA_URL, JIRA_USER, JIRA_API_TOKEN, JIRA_PROJECT (required), JIRA_ISSUE_TYPE (default: Task)
Set JIRA_DRY_RUN=1 to print what would be created without calling Jira.
"""
import base64
import json
import os
import sys
import urllib.request

SEVERITIES = {"critical", "high"}


def build_issues(report: dict, project: str, issue_type: str) -> list:
    host = os.uname().nodename if hasattr(os, "uname") else "host"
    issues = []
    for res in report.get("results", []):
        rule = res.get("rule", {})
        if res.get("status") not in ("FAIL", "ERROR"):
            continue
        if str(rule.get("severity", "")).lower() not in SEVERITIES:
            continue
        issues.append({
            "fields": {
                "project": {"key": project},
                "issuetype": {"name": issue_type},
                "summary": f"[TMAegis] {rule.get('id')} failed on {host}: {rule.get('title')}",
                "description": {
                    "type": "doc", "version": 1,
                    "content": [{"type": "paragraph", "content": [
                        {"type": "text", "text": f"Severity: {rule.get('severity')}. Details: {res.get('details') or 'n/a'}"}]}],
                },
            }
        })
    return issues


def main() -> int:
    report = json.load(sys.stdin)
    project = os.environ.get("JIRA_PROJECT", "")
    issues = build_issues(report, project, os.environ.get("JIRA_ISSUE_TYPE", "Task"))
    if os.environ.get("JIRA_DRY_RUN"):
        print(json.dumps(issues, indent=2))
        return 0
    missing = [k for k in ("JIRA_URL", "JIRA_USER", "JIRA_API_TOKEN", "JIRA_PROJECT") if not os.environ.get(k)]
    if missing:
        print("missing environment: " + ", ".join(missing), file=sys.stderr)
        return 2
    auth = base64.b64encode(f"{os.environ['JIRA_USER']}:{os.environ['JIRA_API_TOKEN']}".encode()).decode()
    for issue in issues:
        req = urllib.request.Request(
            os.environ["JIRA_URL"].rstrip("/") + "/rest/api/3/issue",
            data=json.dumps(issue).encode(),
            headers={"Content-Type": "application/json", "Authorization": "Basic " + auth},
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            print("created", json.load(resp).get("key"))
    print(f"{len(issues)} Jira issue(s) created")
    return 0


if __name__ == "__main__":
    sys.exit(main())
