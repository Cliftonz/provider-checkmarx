#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

cd "$(dirname "$0")"
set -a
# shellcheck source=images.env
. images.env
set +a

CLUSTER="${E2E_CLUSTER:-checkmarx-e2e}"

mkdir -p .out
if ! kind get clusters | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --image "$KIND_NODE_IMAGE" --config kind.yaml
fi

if [ "${E2E_TARGET:-kind}" = live ]; then
  set -- "$CHECKMARX_BASE_URL" "$CHECKMARX_IAM_URL" "$CHECKMARX_TENANT" "$CHECKMARX_API_KEY"
else
  set -- https://ast.checkmarx.invalid https://iam.checkmarx.invalid e2e e2e
fi
jq -n --arg baseUrl "$1" --arg iamUrl "$2" --arg tenant "$3" --arg apiKey "$4" '{$baseUrl, $iamUrl, $tenant, $apiKey}' > .out/credentials.json
echo "wrote $(pwd)/.out/credentials.json"
