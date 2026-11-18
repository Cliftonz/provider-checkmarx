#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
#
# SPDX-License-Identifier: Apache-2.0

load helpers

setup() {
  setup_stubs
  cd "$BATS_TEST_TMPDIR"
}

example() {
  mkdir -p "examples/namespaced/$1/v1alpha1"
  printf 'kind: X\nmetadata:\n  annotations:\n%s\n' "${3:-    a: b}" > "examples/namespaced/$1/v1alpha1/$2.yaml"
}

@test "live mode writes the provider credentials from the environment" {
  cp -r "$ROOT/test/harness" harness
  stub kind
  E2E_TARGET=live CHECKMARX_BASE_URL=https://eu.ast.checkmarx.net CHECKMARX_IAM_URL=https://eu.iam.checkmarx.net \
    CHECKMARX_TENANT=acme CHECKMARX_API_KEY=a.b.c run harness/up.sh
  [ "$status" -eq 0 ]
  [ "$(jq -c . harness/.out/credentials.json)" = '{"baseUrl":"https://eu.ast.checkmarx.net","iamUrl":"https://eu.iam.checkmarx.net","tenant":"acme","apiKey":"a.b.c"}' ]
}

@test "kind mode writes credentials for a tenant that does not exist" {
  cp -r "$ROOT/test/harness" harness
  stub kind
  run harness/up.sh
  [ "$status" -eq 0 ]
  [ "$(jq -c . harness/.out/credentials.json)" = '{"baseUrl":"https://ast.checkmarx.invalid","iamUrl":"https://iam.checkmarx.invalid","tenant":"e2e","apiKey":"e2e"}' ]
}

@test "lists every example except manual intervention" {
  example project project
  example application application
  example group group '    upjet.upbound.io/manual-intervention: needs a tenant'
  run "$ROOT/test/harness/examples.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf 'examples/namespaced/application/v1alpha1/application.yaml\nexamples/namespaced/project/v1alpha1/project.yaml')" ]
}

e2e() {
  mkdir -p test/harness/.out
  cp "$ROOT/test/harness/e2e.sh" "$ROOT/test/harness/examples.sh" "$ROOT/test/harness/setup.sh" "$ROOT/test/harness/runtimeconfig.yaml" test/harness/
  echo '{"baseUrl":"b","iamUrl":"i","tenant":"t","apiKey":"k"}' > test/harness/.out/credentials.json
  example project project
  example application application
  stub kind
  stub make
  stub kubectl 'if [ "${*: -2}" = "-f -" ]; then cat >> applied.yaml; fi'
  E2E_PACKAGE=xpkg.upbound.io/cliftonz/provider-checkmarx:v9.9.9 run test/harness/e2e.sh
}

@test "package mode installs the given package instead of building" {
  e2e
  [ "$status" -eq 0 ]
  grep -q 'package: xpkg.upbound.io/cliftonz/provider-checkmarx:v9.9.9' applied.yaml
  grep -q 'kind: DeploymentRuntimeConfig' applied.yaml
  grep '^kubectl .*wait.* provider-checkmarx' "$CALLS" | grep -q Healthy
  ! grep -q '^make build' "$CALLS"
}

@test "kind mode applies the ProviderConfig with the harness credentials and waits for it to reconcile, running no example" {
  e2e
  [ "$status" -eq 0 ]
  grep -qx 'apiVersion: checkmarx.m.clifton.quest/v1beta1' applied.yaml
  grep -qx 'kind: ProviderConfig' applied.yaml
  grep -qx 'kind: ClusterProviderConfig' applied.yaml
  [ "$(grep -A3 'secretRef:' applied.yaml | grep -c '^ *namespace: ')" -eq "$(grep -c 'secretRef:' applied.yaml)" ]
  grep '^kubectl .*create secret' "$CALLS" | grep -qF '{"baseUrl":"b","iamUrl":"i","tenant":"t","apiKey":"k"}'
  grep '^kubectl .*wait.*providerconfig.checkmarx.m.clifton.quest/default' "$CALLS" | grep -q in-use.crossplane.io
  ! grep -q '^make uptest' "$CALLS"
}

@test "live mode runs uptest once per example instead of the ProviderConfig check" {
  E2E_TARGET=live e2e
  [ "$status" -eq 0 ]
  [ "$(grep -c '^make uptest' "$CALLS")" -eq 2 ]
  grep -q '^make uptest.*examples/namespaced/application/v1alpha1/application.yaml' "$CALLS"
  grep -q '^make uptest.*examples/namespaced/project/v1alpha1/project.yaml' "$CALLS"
  ! grep -q 'wait.*providerconfig' "$CALLS"
}
