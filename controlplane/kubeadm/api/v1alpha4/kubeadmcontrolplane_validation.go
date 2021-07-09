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
	"fmt"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/container"
	"sigs.k8s.io/cluster-api/util/version"
)

func (s *KubeadmControlPlaneSpec) validate(namespace string) (allErrs field.ErrorList) {
	if s.Replicas == nil {
		allErrs = append(
			allErrs,
			field.Required(
				field.NewPath("spec", "replicas"),
				"is required",
			),
		)
	} else if *s.Replicas <= 0 {
		// The use of the scale subresource should provide a guarantee that negative values
		// should not be accepted for this field, but since we have to validate that Replicas != 0
		// it doesn't hurt to also additionally validate for negative numbers here as well.
		allErrs = append(
			allErrs,
			field.Forbidden(
				field.NewPath("spec", "replicas"),
				"cannot be less than or equal to 0",
			),
		)
	}

	externalEtcd := false
	if s.KubeadmConfigSpec.ClusterConfiguration != nil {
		if s.KubeadmConfigSpec.ClusterConfiguration.Etcd.External != nil {
			externalEtcd = true
		}
	}

	if !externalEtcd {
		if s.Replicas != nil && *s.Replicas%2 == 0 {
			allErrs = append(
				allErrs,
				field.Forbidden(
					field.NewPath("spec", "replicas"),
					"cannot be an even number when using managed etcd",
				),
			)
		}
	}

	if s.MachineTemplate.InfrastructureRef.APIVersion == "" {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "machineTemplate", "infrastructure", "apiVersion"),
				s.MachineTemplate.InfrastructureRef.APIVersion,
				"cannot be empty",
			),
		)
	}
	if s.MachineTemplate.InfrastructureRef.Kind == "" {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "machineTemplate", "infrastructure", "kind"),
				s.MachineTemplate.InfrastructureRef.Kind,
				"cannot be empty",
			),
		)
	}
	if s.MachineTemplate.InfrastructureRef.Namespace != namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "machineTemplate", "infrastructure", "namespace"),
				s.MachineTemplate.InfrastructureRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if !version.KubeSemver.MatchString(s.Version) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "version"), s.Version, "must be a valid semantic version"))
	}

	if s.RolloutStrategy != nil {
		if s.RolloutStrategy.Type != RollingUpdateStrategyType {
			allErrs = append(
				allErrs,
				field.Required(
					field.NewPath("spec", "rolloutStrategy", "type"),
					"only RollingUpdateStrategyType is supported",
				),
			)
		}

		ios1 := intstr.FromInt(1)
		ios0 := intstr.FromInt(0)

		if *s.RolloutStrategy.RollingUpdate.MaxSurge == ios0 && *s.Replicas < int32(3) {
			allErrs = append(
				allErrs,
				field.Required(
					field.NewPath("spec", "rolloutStrategy", "rollingUpdate"),
					"when KubeadmControlPlane is configured to scale-in, replica count needs to be at least 3",
				),
			)
		}

		if *s.RolloutStrategy.RollingUpdate.MaxSurge != ios1 && *s.RolloutStrategy.RollingUpdate.MaxSurge != ios0 {
			allErrs = append(
				allErrs,
				field.Required(
					field.NewPath("spec", "rolloutStrategy", "rollingUpdate", "maxSurge"),
					"value must be 1 or 0",
				),
			)
		}
	}

	allErrs = append(allErrs, s.validateCoreDNSImage()...)

	return allErrs
}

func (s *KubeadmControlPlaneSpec) validateCoreDNSImage() (allErrs field.ErrorList) {
	if s.KubeadmConfigSpec.ClusterConfiguration == nil {
		return allErrs
	}
	// TODO: Remove when kubeadm types include OpenAPI validation
	if !container.ImageTagIsValid(s.KubeadmConfigSpec.ClusterConfiguration.DNS.ImageTag) {
		allErrs = append(
			allErrs,
			field.Forbidden(
				field.NewPath("spec", "kubeadmConfigSpec", "clusterConfiguration", "dns", "imageTag"),
				fmt.Sprintf("tag %s is invalid", s.KubeadmConfigSpec.ClusterConfiguration.DNS.ImageTag),
			),
		)
	}
	return allErrs
}

func (s *KubeadmControlPlaneSpec) validateEtcd(prev *KubeadmControlPlaneSpec) (allErrs field.ErrorList) {
	if s.KubeadmConfigSpec.ClusterConfiguration == nil {
		return allErrs
	}

	// TODO: Remove when kubeadm types include OpenAPI validation
	if s.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local != nil && !container.ImageTagIsValid(s.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local.ImageTag) {
		allErrs = append(
			allErrs,
			field.Forbidden(
				field.NewPath("spec", "kubeadmConfigSpec", "clusterConfiguration", "etcd", "local", "imageTag"),
				fmt.Sprintf("tag %s is invalid", s.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local.ImageTag),
			),
		)
	}

	if s.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local != nil && s.KubeadmConfigSpec.ClusterConfiguration.Etcd.External != nil {
		allErrs = append(
			allErrs,
			field.Forbidden(
				field.NewPath("spec", "kubeadmConfigSpec", "clusterConfiguration", "etcd", "local"),
				"cannot have both external and local etcd",
			),
		)
	}

	// update validations
	if prev != nil && prev.KubeadmConfigSpec.ClusterConfiguration != nil {
		if s.KubeadmConfigSpec.ClusterConfiguration.Etcd.External != nil && prev.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local != nil {
			allErrs = append(
				allErrs,
				field.Forbidden(
					field.NewPath("spec", "kubeadmConfigSpec", "clusterConfiguration", "etcd", "external"),
					"cannot change between external and local etcd",
				),
			)
		}

		if s.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local != nil && prev.KubeadmConfigSpec.ClusterConfiguration.Etcd.External != nil {
			allErrs = append(
				allErrs,
				field.Forbidden(
					field.NewPath("spec", "kubeadmConfigSpec", "clusterConfiguration", "etcd", "local"),
					"cannot change between external and local etcd",
				),
			)
		}
	}

	return allErrs
}
