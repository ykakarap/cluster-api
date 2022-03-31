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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
)

// BlockingResponse is the response of a blocking lifecycle hook.
// +kubebuilder:object:root=true
type BlockingResponse struct {
	metav1.TypeMeta `json:",inline"`

	// Status of the call. One of "Success" or "Failure".
	Status ResponseStatus `json:"status"`

	// RetryAfterSeconds when set to a non-zero signifies that the hook
	// needs to be retried at a future time.
	RetryAfterSeconds int `json:"retryAfterSeconds"`

	// A human-readable description of the status of the call.
	Message string `json:"message"`
}

// NonBlockingResponse is the response of a non-blocking lifecycle hook.
// +kubebuilder:object:root=true
type NonBlockingResponse struct {
	metav1.TypeMeta `json:",inline"`

	// Status of the call. One of "Success" or "Failure".
	Status ResponseStatus `json:"status"`

	// A human-readable description of the status of the call.
	Message string `json:"message"`
}

// BeforeClusterCreate Hook

// BeforeClusterCreateRequest is the request of the hook.
// +kubebuilder:object:root=true
type BeforeClusterCreateRequest struct {
	metav1.TypeMeta `json:",inline"`

	// The cluster object the lifecycle hook corresponds to.
	Cluster clusterv1.Cluster `json:"cluster"`
}

// BeforeClusterCreateResponse is the response of BeforeClusterCreate hook.
// +kubebuilder:object:root=true
type BeforeClusterCreateResponse struct {
	// Note: This embedding is only done to help with the open api schema generation for the proposal.
	// The final state of the response struct wil need some refactor to be more extensible.
	BlockingResponse `json:",inline"`
}

func BeforeClusterCreate(*BeforeClusterCreateRequest, *BeforeClusterCreateResponse) {}

// AfterControlPlaneInitialized Hook

// AfterControlPlaneInitializedRequest is the request of the hook.
// +kubebuilder:object:root=true
type AfterControlPlaneInitializedRequest struct {
	metav1.TypeMeta `json:",inline"`

	// The cluster object the lifecycle hook corresponds to.
	Cluster clusterv1.Cluster `json:"cluster"`
}

// AfterControlPlaneInitializedResponse is the response of AfterControlPlaneInitialized hook.
// +kubebuilder:object:root=true
type AfterControlPlaneInitializedResponse struct {
	NonBlockingResponse `json:",inline"`
}

func AfterControlPlaneInitialized(*AfterControlPlaneInitializedRequest, *AfterControlPlaneInitializedResponse) {
}

// BeforeClusterUpgrade Hook.

// BeforeClusterUpgradeRequest is the request of the hook.
// +kubebuilder:object:root=true
type BeforeClusterUpgradeRequest struct {
	metav1.TypeMeta `json:",inline"`

	// The cluster object the lifecycle hook corresponds to.
	Cluster clusterv1.Cluster `json:"cluster"`

	// The current version of the cluster.
	FromKubernetesVersion string `json:"fromKubernetesVersion"`
	// The target version of upgrade.
	ToKubernetesVersion string `json:"toKubernetesVersion"`
}

// BeforeClusterUpgradeResponse is the response of BeforeClusterUpgrade hook.
// +kubebuilder:object:root=true
type BeforeClusterUpgradeResponse struct {
	BlockingResponse `json:",inline"`
}

func BeforeClusterUpgrade(*BeforeClusterUpgradeRequest, *BeforeClusterUpgradeResponse) {}

// AfterControlPlaneUpgrade Hook.

// AfterControlPlaneUpgradeRequest is the request of the hook.
// +kubebuilder:object:root=true
type AfterControlPlaneUpgradeRequest struct {
	metav1.TypeMeta `json:",inline"`

	// The cluster object the lifecycle hook corresponds to.
	Cluster clusterv1.Cluster `json:"cluster"`

	// The version after upgrade.
	KubernetesVersion string `json:"kubernetesVersion"`
}

// AfterControlPlaneUpgradeResponse is the response of AfterControlPlaneUpgrade hook.
// +kubebuilder:object:root=true
type AfterControlPlaneUpgradeResponse struct {
	BlockingResponse `json:",inline"`
}

func AfterControlPlaneUpgrade(*AfterControlPlaneUpgradeRequest, *AfterControlPlaneUpgradeResponse) {}

// AfterClusterUpgrade Hook.

// AfterClusterUpgradeRequest is the request of the hook.
// +kubebuilder:object:root=true
type AfterClusterUpgradeRequest struct {
	metav1.TypeMeta `json:",inline"`

	// The cluster object the lifecycle hook corresponds to.
	Cluster clusterv1.Cluster `json:"cluster"`

	// The version after upgrade.
	KubernetesVersion string `json:"kubernetesVersion"`
}

// AfterClusterUpgradeResponse is the response of AfterClusterUpgrade hook.
// +kubebuilder:object:root=true
type AfterClusterUpgradeResponse struct {
	NonBlockingResponse `json:",inline"`
}

func AfterClusterUpgrade(*AfterClusterUpgradeRequest, *AfterClusterUpgradeResponse) {}

// BeforeClusterDeleteRequest is the request of the hook.
// +kubebuilder:object:root=true
type BeforeClusterDeleteRequest struct {
	metav1.TypeMeta `json:",inline"`

	// The cluster object the lifecycle hook corresponds to.
	Cluster clusterv1.Cluster `json:"cluster"`
}

// BeforeClusterDeleteResponse is the response of BeforeClusterDelete hook.
// +kubebuilder:object:root=true
type BeforeClusterDeleteResponse struct {
	BlockingResponse `json:",inline"`
}

func BeforeClusterDelete(*BeforeClusterDeleteRequest, *BeforeClusterDeleteResponse) {}

func init() {
	catalogBuilder.RegisterHook(BeforeClusterCreate, &catalog.HookMeta{
		Tags:        []string{"Lifecycle Hooks"},
		Summary:     "Called before Cluster topology is created",
		Description: "This hook is called after the Cluster object has been created by the user, immediately before all the objects which are part of a Cluster topology are going to be created. Runtime Extension implementers can use this hook to determine/prepare add-ons for the Cluster and block the creation of those objects.",
	})

	catalogBuilder.RegisterHook(AfterControlPlaneInitialized, &catalog.HookMeta{
		Tags:        []string{"Lifecycle Hooks"},
		Summary:     "Called after the Control Plane is available for the first time",
		Description: "This hook is called after the ControlPlane for the Cluster is marked as available for the first time. Runtime Extension implementers can use this hook to execute tasks that are only possible once the Control Plane is available. This hook does not block any further changes to the Cluster.",
	})

	catalogBuilder.RegisterHook(BeforeClusterUpgrade, &catalog.HookMeta{
		Tags:        []string{"Lifecycle Hooks"},
		Summary:     "Called before the Cluster begins upgrade",
		Description: "This hook is called after the Cluster object has been updated with a new spec.topology.version by the user, and immediately before the new version is going to be propagated to the control plane. Runtime Extension implementers can use this hook to execute pre-upgrade tasks and block upgrades of the ControlPlane and Workers.",
	})

	catalogBuilder.RegisterHook(AfterControlPlaneUpgrade, &catalog.HookMeta{
		Tags:        []string{"Lifecycle Hooks"},
		Summary:     "Called after the Control Plane finished upgrade",
		Description: "This hook is called after the control plane has been upgraded to the version specified in spec.topology.version, and immediately before the new version is going to be propagated to the MachineDeployments existing in the Cluster. Runtime Extension implementers can use this hook to execute post-upgrade tasks and eventually block upgrades to workers.",
	})

	catalogBuilder.RegisterHook(AfterClusterUpgrade, &catalog.HookMeta{
		Tags:        []string{"Lifecycle Hooks"},
		Summary:     "Called after the Cluster finished upgrade",
		Description: "TThis hook is called after the Cluster, control plane and workers have been upgraded to the version specified in spec.topology.version. Runtime Extensions implementers can use this hook to execute post-upgrade tasks. This hook does not block any further changes or upgrades to the Cluster.",
	})

	catalogBuilder.RegisterHook(BeforeClusterDelete, &catalog.HookMeta{
		Tags:        []string{"Lifecycle Hooks"},
		Summary:     "Called before the Cluster is deleted",
		Description: "This hook is called after the Cluster has been deleted by the user, and immediately before objects existing in the Cluster are going to be deleted. Runtime Extension implementers can use this hook to execute cleanup tasks and eventually block deletion of the Cluster and descendant objects until everything is ready.",
	})
}
