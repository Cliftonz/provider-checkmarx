<!--
SPDX-FileCopyrightText: 2026 The provider-checkmarx authors

SPDX-License-Identifier: Apache-2.0
-->

# Checkmarx One test harness

A kind cluster named `checkmarx-e2e` that end to end tests run against. Crossplane and the provider are installed into it by `e2e.sh`. By default no Checkmarx One tenant is involved: the run proves the package installs and the `ProviderConfig` reconciles. Live mode runs the examples against a real tenant.

## Run

Requires Docker, kind, kubectl and jq; run under bash (WSL Ubuntu or Linux).

```sh
test/harness/up.sh     # create cluster if missing, write credentials
test/harness/e2e.sh    # install Crossplane and the locally built provider, check the ProviderConfig
E2E_PACKAGE=xpkg.upbound.io/cliftonz/provider-checkmarx:v0.1.0 test/harness/e2e.sh   # same, with a published package
test/harness/down.sh   # delete the cluster
```

`up.sh` writes `test/harness/.out/credentials.json`, the ProviderConfig secret, with the keys `baseUrl`, `iamUrl`, `tenant` and `apiKey`. In kind mode they name a tenant that does not exist.

## Live mode

```sh
E2E_TARGET=live CHECKMARX_BASE_URL=https://ast.checkmarx.net CHECKMARX_IAM_URL=https://iam.checkmarx.net \
  CHECKMARX_TENANT=my-tenant CHECKMARX_API_KEY=... test/harness/up.sh
E2E_TARGET=live test/harness/e2e.sh
```

`e2e.sh` then runs uptest on every example under `examples/namespaced` that is not marked `upjet.upbound.io/manual-intervention`, instead of the `ProviderConfig` check. The `Live End to End Testing` workflow does the same when the repository secrets `CHECKMARX_BASE_URL`, `CHECKMARX_IAM_URL`, `CHECKMARX_TENANT` and `CHECKMARX_API_KEY` are all set, and skips with a notice otherwise.
