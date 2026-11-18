#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
#
# SPDX-License-Identifier: Apache-2.0

load helpers

@test "make e2e-release requires VERSION" {
  run env -u VERSION -u MAKEFLAGS -u MAKELEVEL make -C "$ROOT" --no-print-directory e2e-release
  [ "$status" -ne 0 ]
  [[ "$output" == *"VERSION is required"* ]]
}

@test "make e2e-release runs the harness against the published Upbound package" {
  run env -u VERSION -u MAKEFLAGS -u MAKELEVEL make -C "$ROOT" --no-print-directory -n e2e-release VERSION=v0.1.0
  [ "$status" -eq 0 ]
  [[ "$output" == *"E2E_PACKAGE=xpkg.upbound.io/cliftonz/provider-checkmarx:v0.1.0 test/harness/e2e.sh"* ]]
}
