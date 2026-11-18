// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projectconfiguration

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.ProjectConfigurationGroupVersionKind, &v1alpha1.ProjectConfiguration{}, &v1alpha1.ProjectConfigurationList{}, newExternal)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.ProjectConfigurationGroupVersionKind, &v1alpha1.ProjectConfiguration{}, &v1alpha1.ProjectConfigurationList{}, newExternal)
}
