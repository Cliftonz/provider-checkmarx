#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

CLUSTER="${E2E_CLUSTER:-checkmarx-e2e}"
UPTEST_CLOUD_CREDENTIALS="$(cat test/harness/.out/credentials.json)"
export UPTEST_CLOUD_CREDENTIALS
mkdir -p _output
export KUBECONFIG="$PWD/_output/$CLUSTER.kubeconfig"
kind export kubeconfig --name "$CLUSTER" --kubeconfig "$KUBECONFIG"

make controlplane.up KIND_CLUSTER_NAME="$CLUSTER"
if [ -n "${E2E_PACKAGE:-}" ]; then
  crds=(crd/providers.pkg.crossplane.io crd/deploymentruntimeconfigs.pkg.crossplane.io)
  kubectl -n crossplane-system wait --for=condition=Available deployment --all --timeout=5m
  kubectl wait --for condition=Established --timeout=5m "${crds[@]}"
  {
    cat test/harness/runtimeconfig.yaml
    cat <<EOF
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-checkmarx
spec:
  package: $E2E_PACKAGE
  runtimeConfigRef:
    name: runtimeconfig-provider-checkmarx
EOF
  } | kubectl apply -f -
  kubectl wait provider.pkg provider-checkmarx --for condition=Healthy --timeout=5m
else
  hash="$(find apis internal cmd/provider package cluster/images go.mod go.sum -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1)"
  make build PLATFORMS=linux_amd64
  make e2e.provider KIND_CLUSTER_NAME="$CLUSTER" LOCAL_XPKG_DIGEST="sha256:$hash" DRC_FILE=test/harness/runtimeconfig.yaml
  kubectl wait provider.pkg provider-checkmarx --for=jsonpath='{.status.currentIdentifier}'="xpkg.crossplane.internal/dev/provider-checkmarx@sha256:$hash" --timeout=5m
fi

if [ "${E2E_TARGET:-kind}" = live ]; then
  for example in $(test/harness/examples.sh); do
    make uptest UPTEST_INPUT_MANIFESTS="$example"
  done
else
  KUBECTL=kubectl test/harness/setup.sh
  kubectl -n default wait providerconfig.checkmarx.m.clifton.quest/default --for=jsonpath='{.metadata.finalizers[0]}'=in-use.crossplane.io --timeout=5m
fi
