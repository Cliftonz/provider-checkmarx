#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

cd "$(dirname "$0")"
kind delete cluster --name "${E2E_CLUSTER:-checkmarx-e2e}"
rm -f .out/credentials.json
