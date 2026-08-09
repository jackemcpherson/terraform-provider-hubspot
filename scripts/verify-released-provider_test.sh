#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

mkdir -p "$tmp/bin" "$tmp/assets"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) echo "unsupported test architecture" >&2; exit 1 ;;
esac
archive="$tmp/assets/terraform-provider-hubspot_0.1.4_${os}_${arch}.zip"
printf '%s\n' 'provider archive' >"$archive"
digest=$(shasum -a 256 "$archive" | awk '{print $1}')
printf '%s  %s\n' "$digest" "$(basename "$archive")" >"$tmp/assets/terraform-provider-hubspot_0.1.4_SHA256SUMS"

cat >"$tmp/bin/terraform" <<'EOF'
#!/bin/sh
set -eu

chdir=${1#-chdir=}
shift
case "$*" in
  'init -backend=false -input=false')
    cat >"$chdir/.terraform.lock.hcl" <<LOCK
provider "registry.terraform.io/jackemcpherson/hubspot" {
  version     = "$FAKE_VERSION"
  constraints = "0.1.4"
  hashes = ["zh:$FAKE_DIGEST"]
}
LOCK
    ;;
  'providers schema -json')
    if [ "${OMIT_FILES:-}" = 1 ]; then
      printf '%s\n' '{"hubspot_property_group":{},"hubspot_property_definition":{},"hubspot_form_definition":{}}'
    else
      printf '%s\n' '{"hubspot_property_group":{},"hubspot_property_definition":{},"hubspot_form_definition":{},"hubspot_file_folder":{},"hubspot_file":{}}'
    fi
    ;;
  *)
    echo "unexpected terraform invocation: $*" >&2
    exit 1
    ;;
esac
EOF
chmod +x "$tmp/bin/terraform"

PATH="$tmp/bin:$PATH" FAKE_VERSION=0.1.4 FAKE_DIGEST="$digest" \
  "$root/scripts/verify-released-provider.sh" \
  terraform registry.terraform.io/jackemcpherson/hubspot v0.1.4 "$tmp/assets"

ln -s terraform "$tmp/bin/tofu"
PATH="$tmp/bin:$PATH" FAKE_VERSION=0.1.4 FAKE_DIGEST="$digest" \
  "$root/scripts/verify-released-provider.sh" \
  tofu registry.opentofu.org/jackemcpherson/hubspot v0.1.4 "$tmp/assets"

if PATH="$tmp/bin:$PATH" FAKE_VERSION=0.1.3 FAKE_DIGEST="$digest" \
  "$root/scripts/verify-released-provider.sh" \
  terraform registry.terraform.io/jackemcpherson/hubspot v0.1.4 "$tmp/assets" \
  >"$tmp/mismatch-output" 2>&1; then
  echo 'expected verifier to reject a different selected version' >&2
  exit 1
fi
grep -q 'registry lock selected 0.1.3 instead of 0.1.4' "$tmp/mismatch-output"

if PATH="$tmp/bin:$PATH" FAKE_VERSION=0.1.4 FAKE_DIGEST=ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff \
  "$root/scripts/verify-released-provider.sh" \
  terraform registry.terraform.io/jackemcpherson/hubspot v0.1.4 "$tmp/assets" \
  >"$tmp/digest-output" 2>&1; then
  echo 'expected verifier to reject a different registry archive digest' >&2
  exit 1
fi
grep -q 'registry digest does not match the GitHub release' "$tmp/digest-output"

if PATH="$tmp/bin:$PATH" "$root/scripts/verify-released-provider.sh" \
  terraform registry.opentofu.org/jackemcpherson/hubspot v0.1.4 "$tmp/assets" >/dev/null 2>&1; then
  echo 'expected verifier to reject an engine/registry mismatch' >&2
  exit 1
fi

if PATH="$tmp/bin:$PATH" OMIT_FILES=1 FAKE_VERSION=0.1.4 FAKE_DIGEST="$digest" \
  "$root/scripts/verify-released-provider.sh" \
  terraform registry.terraform.io/jackemcpherson/hubspot v0.1.4 "$tmp/assets" >/dev/null 2>&1; then
  echo 'expected verifier to reject a provider without Files resources' >&2
  exit 1
fi

mkdir "$tmp/bad-inventory"
cp "$tmp/assets/"* "$tmp/bad-inventory/"
printf '%064d  %s\n' 0 "$(basename "$archive")" >"$tmp/bad-inventory/terraform-provider-hubspot_0.1.4_SHA256SUMS"
if PATH="$tmp/bin:$PATH" FAKE_VERSION=0.1.4 FAKE_DIGEST="$digest" \
  "$root/scripts/verify-released-provider.sh" \
  terraform registry.terraform.io/jackemcpherson/hubspot v0.1.4 "$tmp/bad-inventory" >/dev/null 2>&1; then
  echo 'expected verifier to reject an archive outside the GitHub checksum inventory' >&2
  exit 1
fi

echo 'Released provider verifier tests passed'
