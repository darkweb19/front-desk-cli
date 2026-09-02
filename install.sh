#!/usr/bin/env sh

set -eu

repository="darkweb19/front-desk-cli"
version="${1:-latest}"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "Unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

asset="tm_${os}_${arch}"
if [ "$version" = "latest" ]; then
  release_url="https://github.com/${repository}/releases/latest/download"
else
  case "$version" in
    v*) ;;
    *)
      echo "Version must be a tag beginning with v, for example v1.2.0." >&2
      exit 1
      ;;
  esac
  release_url="https://github.com/${repository}/releases/download/${version}"
fi

temp_dir="$(mktemp -d)"
staged_binary=""

cleanup() {
  if [ -n "$staged_binary" ] && [ -e "$staged_binary" ]; then
    rm -f "$staged_binary"
  fi
  rm -rf "$temp_dir"
}

trap cleanup EXIT
trap 'exit 1' HUP INT TERM

download() {
  url="$1"
  destination="$2"
  if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error "$url" --output "$destination"
  elif command -v wget >/dev/null 2>&1; then
    wget --quiet "$url" --output-document="$destination"
  else
    echo "Install curl or wget, then run this installer again." >&2
    exit 1
  fi
}

download "${release_url}/${asset}" "${temp_dir}/${asset}"
download "${release_url}/SHA256SUMS" "${temp_dir}/SHA256SUMS"

expected="$(awk -v asset="$asset" '$2 == asset || $2 == ("*" asset) { print $1 }' "${temp_dir}/SHA256SUMS")"
if [ -z "$expected" ]; then
  echo "The release checksum file does not contain ${asset}." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${temp_dir}/${asset}" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${temp_dir}/${asset}" | awk '{ print $1 }')"
else
  echo "No SHA-256 checksum tool was found." >&2
  exit 1
fi

if [ "$actual" != "$expected" ]; then
  echo "Checksum verification failed for ${asset}." >&2
  exit 1
fi

install_dir="${HOME}/.local/bin"
mkdir -p "$install_dir"
staged_binary="$(mktemp "${install_dir}/.tm.install.XXXXXX")"
install -m 0755 "${temp_dir}/${asset}" "$staged_binary"
mv -f "$staged_binary" "${install_dir}/tm"
staged_binary=""

echo "Installed tm to ${install_dir}/tm"
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *) echo "Add ${install_dir} to PATH before running tm." ;;
esac
