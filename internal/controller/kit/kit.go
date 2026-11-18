// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

type NewExternal[T resource.ModernManaged] func(c *clients.Connection) managed.TypedExternalClient[T]

func Setup[T resource.ModernManaged](mgr ctrl.Manager, o controller.Options, gvk schema.GroupVersionKind, mg T, list resource.ManagedList, newExternal NewExternal[T]) error {
	name := managed.ControllerName(gvk.GroupKind().String())

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[T](connector(mgr.GetClient(), newExternal)),
		managed.WithInitializers(),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))), //nolint:staticcheck // crossplane-runtime has not adopted the new events API
		managed.WithMetricRecorder(o.MetricOptions.MRMetrics),
	}
	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}
	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if err := mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, list, o.MetricOptions.PollStateMetricInterval)); err != nil {
		return err
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(gvk), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(mg).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

func connector[T resource.ModernManaged](kube client.Client, newExternal NewExternal[T]) managed.TypedExternalConnectorFn[T] {
	return func(ctx context.Context, mg T) (managed.TypedExternalClient[T], error) {
		conn, err := clients.NewClient(ctx, kube, mg)
		if err != nil {
			return nil, err
		}
		return newExternal(conn), nil
	}
}

func SetupGated[T resource.ModernManaged](mgr ctrl.Manager, o controller.Options, gvk schema.GroupVersionKind, mg T, list resource.ManagedList, newExternal NewExternal[T]) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o, gvk, mg, list, newExternal); err != nil {
			mgr.GetLogger().Error(err, "unable to setup reconciler", "gvk", gvk.String())
		}
	}, gvk)
	return nil
}

type response interface {
	StatusCode() int
	Status() string
	GetBody() []byte
}

func CheckStatus(action string, resp response) error {
	if resp.StatusCode() >= http.StatusOK && resp.StatusCode() < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("cannot %s: %s: %s", action, resp.Status(), resp.GetBody())
}

func NotFound(resp interface{ StatusCode() int }) bool {
	return resp.StatusCode() == http.StatusNotFound
}

func Value[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

func Time(t *time.Time) *metav1.Time {
	if t == nil {
		return nil
	}
	return &metav1.Time{Time: *t}
}

func ExternalUUID(mg resource.Managed) (uuid.UUID, error) {
	name := meta.GetExternalName(mg)
	id, err := uuid.Parse(name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("external name %q is not a UUID: %w", name, err)
	}
	return id, nil
}
