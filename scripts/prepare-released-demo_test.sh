#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
demo="$tmp/demo"
prepared="$tmp/prepared"
assets="$tmp/assets"
log="$tmp/calls"
mkdir -p "$demo/scripts" "$demo/locks/tofu" "$demo/locks/terraform" "$assets"

cat >"$demo/scripts/demo" <<'EOF'
#!/bin/sh
set -eu
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
printf '%s:%s:%s\n' "$ENGINE" "$1" "$2" >>"$CALL_LOG"
case "$ENGINE" in
  tofu) address=registry.opentofu.org/jackemcpherson/hubspot ;;
  terraform) address=registry.terraform.io/jackemcpherson/hubspot ;;
esac
digest=${LOCK_DIGEST:-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855}
cat >"$root/locks/$ENGINE/.terraform.lock.hcl" <<LOCK
provider "$address" {
  version = "0.4.0"
  hashes = ["zh:$digest"]
}
LOCK
EOF
chmod +x "$demo/scripts/demo"
printf '%s\n' 'exact source' >"$demo/main.tf"
printf '%s\n' 'placeholder' >"$demo/locks/tofu/.terraform.lock.hcl"
printf '%s\n' 'placeholder' >"$demo/locks/terraform/.terraform.lock.hcl"
git -C "$demo" init -q
git -C "$demo" config user.name test
git -C "$demo" config user.email test@example.com
git -C "$demo" add .
git -C "$demo" commit -qm fixture
commit=$(git -C "$demo" rev-parse HEAD)
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
archive="$assets/terraform-provider-hubspot_0.4.0_${os}_${arch}.zip"
: >"$archive"
printf '%s  %s\n' e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 "$(basename "$archive")" >"$assets/terraform-provider-hubspot_0.4.0_SHA256SUMS"

CALL_LOG="$log" "$root/scripts/prepare-released-demo.sh" v0.4.0 "$demo" "$commit" "$prepared" "$assets"
expected=$(printf '%s\n' terraform:registry:init tofu:registry:init)
test "$(cat "$log")" = "$expected"
test ! -e "$prepared/.git"
test "$(cat "$prepared/main.tf")" = 'exact source'
grep -q 'version = "0.4.0"' "$prepared/locks/terraform/.terraform.lock.hcl"
grep -q 'version = "0.4.0"' "$prepared/locks/tofu/.terraform.lock.hcl"

if CALL_LOG="$tmp/wrong.log" "$root/scripts/prepare-released-demo.sh" v0.4.0 "$demo" aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa "$tmp/wrong" "$assets" >/dev/null 2>&1; then
  echo 'expected wrong demo provenance rejection' >&2
  exit 1
fi
test ! -s "$tmp/wrong.log"

if CALL_LOG="$tmp/digest.log" LOCK_DIGEST=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  "$root/scripts/prepare-released-demo.sh" v0.4.0 "$demo" "$commit" "$tmp/bad-digest" "$assets" >/dev/null 2>&1; then
  echo 'expected registry lock digest rejection' >&2
  exit 1
fi

echo 'Released demo preparation contract tests passed'
