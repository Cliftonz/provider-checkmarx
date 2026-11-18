// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/internal/controller/accessassignment"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/application"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/config"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/group"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/project"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/projectconfiguration"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/projectgitsettings"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/tenantconfiguration"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return run(mgr, o,
		config.Setup,
		project.Setup,
		application.Setup,
		projectconfiguration.Setup,
		tenantconfiguration.Setup,
		accessassignment.Setup,
		projectgitsettings.Setup,
		group.Setup,
	)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return run(mgr, o,
		config.SetupGated,
		project.SetupGated,
		application.SetupGated,
		projectconfiguration.SetupGated,
		tenantconfiguration.SetupGated,
		accessassignment.SetupGated,
		projectgitsettings.SetupGated,
		group.SetupGated,
	)
}

func run(mgr ctrl.Manager, o controller.Options, setups ...func(ctrl.Manager, controller.Options) error) error {
	for _, setup := range setups {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
