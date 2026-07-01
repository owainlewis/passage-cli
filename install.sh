#!/usr/bin/env bash
set -euo pipefail

repo="${PASSAGE_REPO:-owainlewis/passage-cli}"
version="${PASSAGE_VERSION:-latest}"
install_dir="${PASSAGE_INSTALL_DIR:-}"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'passage install: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

need curl
need tar

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin) os="darwin" ;;
  linux) os="linux" ;;
  *) fail "unsupported OS: $(uname -s)" ;;
esac

case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

if [ "$version" = "latest" ]; then
  latest_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest" || true)"
  if [ -z "$latest_url" ] || [ "$latest_url" = "https://github.com/${repo}/releases/latest" ]; then
    fail "no GitHub release found for ${repo}"
  fi
  version="${latest_url##*/}"
fi

case "$version" in
  v*) ;;
  *) fail "release version must look like v0.1.0, got ${version}" ;;
esac

if [ -z "$install_dir" ]; then
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    install_dir="/usr/local/bin"
  else
    install_dir="${HOME}/.local/bin"
  fi
fi

archive="passage_${version}_${os}_${arch}.tar.gz"
base_url="${PASSAGE_DOWNLOAD_BASE_URL:-https://github.com/${repo}/releases/download/${version}}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

log "Downloading passage ${version} for ${os}/${arch}"
curl -fsSLo "${tmpdir}/${archive}" "${base_url}/${archive}"
curl -fsSLo "${tmpdir}/${archive}.sha256" "${base_url}/${archive}.sha256"

expected="$(awk '{print $1}' "${tmpdir}/${archive}.sha256")"
if command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')"
else
  fail "missing shasum or sha256sum for checksum verification"
fi

if [ "$expected" != "$actual" ]; then
  fail "checksum verification failed"
fi

tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"
mkdir -p "$install_dir"
cp "${tmpdir}/passage" "${install_dir}/passage"
chmod +x "${install_dir}/passage"

log "Installed passage to ${install_dir}/passage"

case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *) log "Add ${install_dir} to your PATH to run passage from any shell." ;;
esac

"${install_dir}/passage" version
