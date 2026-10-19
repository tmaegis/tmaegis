#!/usr/bin/env bash
# Fills the Homebrew formula and Scoop manifest templates from a published release.
#   packaging/render.sh 0.1.0 <dir-with-checksums.txt> <out-dir>
# Download checksums.txt from the release first:
#   gh release download v0.1.0 -R tmaegis/tmaegis -p checksums.txt -D /tmp/rel
set -euo pipefail
VERSION="${1:?version, e.g. 0.1.0}"; SUMS="${2:?dir containing checksums.txt}/checksums.txt"; OUT="${3:?output dir}"
here="$(cd "$(dirname "$0")" && pwd)"
sha() { local v; v=$(awk -v f="tmaegis_${VERSION}_$1" '$2==f {print $1}' "$SUMS"); [ -n "$v" ] || { echo "no checksum for tmaegis_${VERSION}_$1 in $SUMS" >&2; exit 1; }; echo "$v"; }
mkdir -p "$OUT"
render() {
  sed -e "s/@VERSION@/${VERSION}/g" \
      -e "s/@SHA_DARWIN_ARM64@/$(sha darwin_arm64.tar.gz)/" -e "s/@SHA_DARWIN_AMD64@/$(sha darwin_amd64.tar.gz)/" \
      -e "s/@SHA_LINUX_ARM64@/$(sha linux_arm64.tar.gz)/"   -e "s/@SHA_LINUX_AMD64@/$(sha linux_amd64.tar.gz)/" \
      -e "s/@SHA_WINDOWS_AMD64@/$(sha windows_amd64.zip)/"  -e "s/@SHA_WINDOWS_ARM64@/$(sha windows_arm64.zip)/" "$1" > "$2"
}
render "$here/homebrew/tmaegis.rb.tmpl" "$OUT/tmaegis.rb"
render "$here/scoop/tmaegis.json.tmpl" "$OUT/tmaegis.json"
echo "wrote $OUT/tmaegis.rb and $OUT/tmaegis.json"
