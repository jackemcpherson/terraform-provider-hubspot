#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
demo="$tmp/demo"
log="$tmp/release-calls"
mkdir -p "$demo/locks/tofu" "$demo/locks/terraform"

cat >"$demo/versions.tf" <<'EOF'
terraform {
  required_providers {
    hubspot = {
      source  = "jackemcpherson/hubspot"
      version = ">= 0.4.0, < 0.5.0"
    }
  }
}
EOF
for engine in tofu terraform; do
	case "$engine" in
		tofu) address=registry.opentofu.org/jackemcpherson/hubspot ;;
		terraform) address=registry.terraform.io/jackemcpherson/hubspot ;;
	esac
	cat >"$demo/locks/$engine/.terraform.lock.hcl" <<EOF
provider "$address" {
  version     = "0.4.0"
  constraints = ">= 0.4.0, < 0.5.0"
  hashes      = ["h1:0WnQNSWX/OwYe1pbEN8Kla8ej2VV90oP7vLcBj6SZ7A="]
}
EOF
done
cat >"$tmp/release-preflight" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"$CALL_LOG"
EOF
chmod +x "$tmp/release-preflight"

CALL_LOG="$log" REGISTRY_RELEASE_PREFLIGHT_SCRIPT="$tmp/release-preflight" \
	"$root/scripts/candidate-preflight.sh" v0.4.0 "$demo"
test "$(cat "$log")" = v0.4.0

: >"$log"
sed 's/< 0.5.0/< 0.4.0/' "$demo/versions.tf" >"$demo/stale.tf"
rm "$demo/versions.tf"
if CALL_LOG="$log" REGISTRY_RELEASE_PREFLIGHT_SCRIPT="$tmp/release-preflight" \
	"$root/scripts/candidate-preflight.sh" v0.4.0 "$demo"; then
	echo 'candidate preflight built a bundle after compatibility failed' >&2
	exit 1
fi
test ! -s "$log"
