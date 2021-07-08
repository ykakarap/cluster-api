/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha4

import (
	"errors"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var dockerclustertemplatelog = logf.Log.WithName("dockerclustertemplate-resource")

func (r *DockerClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha4-dockerclustertemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=dockerclustertemplates,versions=v1alpha4,name=validation.dockerclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Defaulter = &DockerClusterTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *DockerClusterTemplate) Default() {
	dockerclustertemplatelog.Info("default", "name", r.Name)
	r.Spec.Template.Spec.setDefaults()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-dockerclustertemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=dockerclustertemplates,versions=v1alpha4,name=validation.dockerclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Validator = &DockerClusterTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *DockerClusterTemplate) ValidateCreate() error {
	dockerclustertemplatelog.Info("validate create", "name", r.Name)
	allErrs := r.Spec.Template.Spec.validate()
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("DockerClusterTemplate").GroupKind(), r.Name, allErrs)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *DockerClusterTemplate) ValidateUpdate(oldRaw runtime.Object) error {
	dockerclustertemplatelog.Info("validate update", "name", r.Name)
	old := oldRaw.(*DockerClusterTemplate)
	if !reflect.DeepEqual(r.Spec, old.Spec) {
		return errors.New("DockerClusterTemplateSpec is immutable")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *DockerClusterTemplate) ValidateDelete() error {
	dockerclustertemplatelog.Info("validate delete", "name", r.Name)
	return nil
}
