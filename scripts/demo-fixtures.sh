#!/usr/bin/env bash
# Create sample state files under /tmp/tfdemo for recording docs/media/tui-demo.tape.
set -euo pipefail

root=/tmp/tfdemo
rm -rf "$root"
mkdir -p "$root/dev/network" "$root/dev/app" "$root/prod"

provider='provider[\"registry.terraform.io/hashicorp/aws\"]'
resources="[{\"mode\":\"managed\",\"type\":\"aws_vpc\",\"name\":\"main\",\"provider\":\"$provider\",\"instances\":[{\"schema_version\":1,\"attributes\":{\"id\":\"vpc-0abc\"}}]},{\"mode\":\"managed\",\"type\":\"aws_subnet\",\"name\":\"private\",\"provider\":\"$provider\",\"instances\":[{\"schema_version\":1,\"attributes\":{\"id\":\"subnet-01\"}}]}]"

write_state() {
  printf '{"version":4,"terraform_version":"1.11.3","serial":12,"lineage":"demo","outputs":{},"resources":%s}' "$2" > "$1/terraform.tfstate"
}

write_state "$root/dev/network" "$resources"
write_state "$root/prod" "$resources"
write_state "$root/dev/app" "[]"
