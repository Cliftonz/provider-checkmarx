// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	accessassignmentv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/accessassignment/v1alpha1"
	applicationv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/application/v1alpha1"
	groupv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
	projectv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	projectconfigurationv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
	projectgitsettingsv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
	tenantconfigurationv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/tenantconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/apis/v1beta1"
)

var schemeBuilder = runtime.SchemeBuilder{
	v1beta1.SchemeBuilder.AddToScheme,
	projectv1alpha1.SchemeBuilder.AddToScheme,
	applicationv1alpha1.SchemeBuilder.AddToScheme,
	projectconfigurationv1alpha1.SchemeBuilder.AddToScheme,
	tenantconfigurationv1alpha1.SchemeBuilder.AddToScheme,
	accessassignmentv1alpha1.SchemeBuilder.AddToScheme,
	projectgitsettingsv1alpha1.SchemeBuilder.AddToScheme,
	groupv1alpha1.SchemeBuilder.AddToScheme,
}

var AddToScheme = schemeBuilder.AddToScheme
