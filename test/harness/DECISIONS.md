<!--
SPDX-FileCopyrightText: 2026 The provider-checkmarx authors

SPDX-License-Identifier: Apache-2.0
-->

# Decisions

- No Checkmarx One tenant is available and Checkmarx One cannot be self-hosted, so the kind harness deploys nothing besides Crossplane and the provider. `up.sh` only creates the cluster and writes credentials for a tenant that does not exist, under the reserved `.invalid` domain.
- In kind mode `e2e.sh` applies the `ProviderConfig` and `ClusterProviderConfig` through `setup.sh` and waits for the `ProviderConfig` to carry the `in-use.crossplane.io` finalizer, which its reconciler adds without contacting Checkmarx. No example runs, since every managed resource needs a tenant.
- Live mode (`E2E_TARGET=live`) takes an API key credential from `CHECKMARX_*` variables and runs uptest once per example file. The live workflow runs only when all four repository secrets exist.
- `E2E_PACKAGE` makes `e2e.sh` install a published package with `runtimeconfig.yaml` instead of building and deploying a local xpkg.
- The release check installs the published package and verifies the `ProviderConfig` reconciles.
- The kind node image is pinned by digest in `images.env`.
