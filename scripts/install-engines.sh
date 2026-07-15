#!/bin/sh
set -eu

bin_dir=${1:?tool bin directory is required}
tofu_version=${2:?OpenTofu version is required}
terraform_version=${3:?Terraform version is required}
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) echo "unsupported tool architecture" >&2; exit 1 ;;
esac
case "$os" in darwin|linux) ;; *) echo "unsupported tool operating system" >&2; exit 1 ;; esac

mkdir -p "$bin_dir"

install_zip() {
  binary=$1
  shift
  archive_url=$1
  sums_url=$2
  archive_name=$3
  expected_marker=$4

  if command -v "$binary" >/dev/null 2>&1 && "$binary" version | grep -F "$expected_marker" >/dev/null; then
    return
  fi

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT HUP INT TERM
  curl --fail --location --silent --show-error "$archive_url" --output "$tmp/$archive_name"
  curl --fail --location --silent --show-error "$sums_url" --output "$tmp/SHA256SUMS"
  (cd "$tmp" && awk -v file="$archive_name" '$2 == file' SHA256SUMS | shasum -a 256 -c -)
  unzip -q "$tmp/$archive_name" "$binary" -d "$tmp/unpacked"
  install -m 0755 "$tmp/unpacked/$binary" "$bin_dir/$binary"
  "$bin_dir/$binary" version | grep -F "$expected_marker" >/dev/null
  rm -rf "$tmp"
  trap - EXIT HUP INT TERM
}

tofu_archive="tofu_${tofu_version}_${os}_${arch}.zip"
install_zip tofu \
  "https://github.com/opentofu/opentofu/releases/download/v${tofu_version}/${tofu_archive}" \
  "https://github.com/opentofu/opentofu/releases/download/v${tofu_version}/tofu_${tofu_version}_SHA256SUMS" \
  "$tofu_archive" "OpenTofu v$tofu_version"

terraform_archive="terraform_${terraform_version}_${os}_${arch}.zip"
install_zip terraform \
  "https://releases.hashicorp.com/terraform/${terraform_version}/${terraform_archive}" \
  "https://releases.hashicorp.com/terraform/${terraform_version}/terraform_${terraform_version}_SHA256SUMS" \
  "$terraform_archive" "Terraform v$terraform_version"
