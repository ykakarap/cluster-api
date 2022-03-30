/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1beta1"
	runtimeclient "sigs.k8s.io/cluster-api/internal/runtime/client"
	"sigs.k8s.io/cluster-api/internal/runtime/registry"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

// FIXME: Replace all direct calls to registry with Client equivalents when available.

// +kubebuilder:rbac:groups=runtime.cluster.x-k8s.io,resources=extension,verbs=get;list;watch;create;update;patch;delete

// Reconciler reconciles an Extension object.
type Reconciler struct {
	Client        client.Client
	APIReader     client.Reader
	RuntimeClient runtimeclient.Client
	Registry      registry.ExtensionRegistry
	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1.Extension{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	if !r.Registry.IsReady() {
		err := r.syncRegistry(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "extensions controller not initialized")
		}
		// Note: there was an intermittent bug when allowing this to continue (likely due to patching the same object multiple times?)
		return ctrl.Result{}, nil
	}

	extension := &runtimev1.Extension{}
	err := r.Client.Get(ctx, req.NamespacedName, extension)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Extension not found. Remove from registry.
			err = r.RuntimeClient.Extension(extension).Unregister()
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(extension, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// FIXME: could dedupe these patch calls. Ignoring for simplicity at this stage.
	defer func() {
		// Always attempt to patch the object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
				runtimev1.RuntimeExtensionDiscovered,
			}})
		}
		if err := patchExtension(ctx, patchHelper, extension, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deletion reconciliation loop.
	if !extension.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, extension)
	}

	// Handle normal reconciliation loop.
	return r.reconcile(ctx, extension)
}

func patchExtension(ctx context.Context, patchHelper *patch.Helper, ext *runtimev1.Extension, options ...patch.Option) error {
	// Can add errors and waits here to the Extension object.
	return patchHelper.Patch(ctx, ext, options...)
}

func (r *Reconciler) reconcileDelete(_ context.Context, extension *runtimev1.Extension) (ctrl.Result, error) {
	// FIXME: This isn't implemented on the client/registry.
	if err := r.RuntimeClient.Extension(extension).Unregister(); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcile(ctx context.Context, extension *runtimev1.Extension) (ctrl.Result, error) {
	if err := r.discoverRuntimeExtensions(ctx, extension); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) syncRegistry(ctx context.Context) (reterr error) {
	extensionList := runtimev1.ExtensionList{}
	if err := r.APIReader.List(ctx, &extensionList); err != nil {
		return err
	}
	if err := r.Registry.WarmUp(&extensionList); err != nil {
		return err
	}
	var errs []error
	for _, ext := range extensionList.Items {
		extension := ext.DeepCopy()
		// Initialize the patch helper for this extension.
		patchHelper, err := patch.NewHelper(extension, r.Client)
		if err != nil {
			return err
		}
		if err := r.discoverRuntimeExtensions(ctx, extension); err != nil {
			errs = append(errs, err)
		} else {
			conditions.MarkTrue(extension, runtimev1.RuntimeExtensionDiscovered)
		}

		defer func(extension runtimev1.Extension) { //nolint:gocritic
			// Always attempt to patch the object and status after each reconciliation.
			// Patch ObservedGeneration only if the reconciliation completed successfully
			patchOpts := []patch.Option{}
			if reterr == nil {
				patchOpts = append(patchOpts, patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
					runtimev1.RuntimeExtensionDiscovered,
				}})
			}
			if err = patchExtension(ctx, patchHelper, &extension, patchOpts...); err != nil {
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}(*extension)
	}

	if len(errs) != 0 {
		return kerrors.NewAggregate(errs)
	}
	// If all extensions are listed and discovered successfully the reconciler is now ready.
	return nil
}

func (r Reconciler) discoverRuntimeExtensions(_ context.Context, extension *runtimev1.Extension) error {
	extensions, err := r.RuntimeClient.Extension(extension).Discover()
	if err != nil {
		// If discovery fails we mark the condition as untrue and return. Previous registrations are not removed.
		// what should we do about intermittent errors? should the registry know when each individual extension is not discovered?
		// This handling should be in the client possibly?
		conditions.MarkFalse(extension, runtimev1.RuntimeExtensionDiscovered, "DiscoveryFailed", clusterv1.ConditionSeverityError, "error in discovery: %v", err)
		return err
	}
	extension.Status.RuntimeExtensions = extensions

	return nil
}
