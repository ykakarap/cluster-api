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

package v1beta1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/cluster-api/feature"
)

func (e *Extension) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(e).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-runtime-cluster-x-k8s-io-v1beta1-extension,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=runtime.cluster.x-k8s.io,resources=extensions,versions=v1beta1,name=validation.extensions.runtime.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-runtime-cluster-x-k8s-io-v1beta1-extension,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=runtime.cluster.x-k8s.io,resources=extensions,versions=v1beta1,name=default.extension.runtime.addons.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &Extension{}
var _ webhook.Defaulter = &Extension{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (e *Extension) Default() {
	// Default NamespaceSelector to an empty LabelSelector, which matches everything, if not set.
	if e.Spec.NamespaceSelector == nil {
		e.Spec.NamespaceSelector = &metav1.LabelSelector{}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (e *Extension) ValidateCreate() error {
	// NOTE: Extensions is behind the RuntimeSDK feature gate flag; the web hook
	// must prevent creating new objects in case the feature flag is disabled.
	if !feature.Gates.Enabled(feature.RuntimeSDK) {
		return field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the RuntimeSDK feature flag is enabled",
		)
	}

	// NOTE: Extensions is also behind the ClusterTopology feature gate flag; the web hook
	// must prevent creating new objects in case the feature flag is disabled.
	if !feature.Gates.Enabled(feature.ClusterTopology) {
		return field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the ClusterTopology feature flag is enabled",
		)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (e *Extension) ValidateUpdate(old runtime.Object) error {
	if !feature.Gates.Enabled(feature.RuntimeSDK) {
		return field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the RuntimeSDK feature flag is enabled",
		)
	}

	// NOTE: Extensions is also behind the ClusterTopology feature gate flag; the web hook
	// must prevent creating new objects in case the feature flag is disabled.
	if !feature.Gates.Enabled(feature.ClusterTopology) {
		return field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the ClusterTopology feature flag is enabled",
		)
	}
	if _, ok := old.(*Extension); !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an Extension but got a %T", old))
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (e *Extension) ValidateDelete() error {
	return nil
}
