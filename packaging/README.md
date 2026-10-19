# Packaging templates

Templates for package managers that need a manifest in a separate repository. They are not published
automatically: they need repositories and credentials that only the project owners can create.

| Manager | Where it goes | Status |
|---|---|---|
| Homebrew (macOS, Linux) | repo `tmaegis/homebrew-tap`, file `Formula/tmaegis.rb` | template ready |
| Scoop (Windows) | repo `tmaegis/scoop-bucket`, file `bucket/tmaegis.json` | template ready |
| winget | pull request to `microsoft/winget-pkgs` | needs an installer (.msi or signed .exe); the zip layout keeps the rules next to the exe, which winget's portable mode does not support |

After a release is published:

```bash
mkdir /tmp/rel && gh release download v0.1.0 -R tmaegis/tmaegis -p checksums.txt -D /tmp/rel
packaging/render.sh 0.1.0 /tmp/rel /tmp/out      # writes tmaegis.rb and tmaegis.json
```

Copy the files into the tap and the bucket and push. Users then run
`brew install tmaegis/tap/tmaegis` or `scoop bucket add tmaegis https://github.com/tmaegis/scoop-bucket && scoop install tmaegis`.
