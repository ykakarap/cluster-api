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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
)

// ResponseStatus represents the status of the hook response.
type ResponseStatus string

const (
	ResponseSuccess ResponseStatus = "success"
	ResponseError   ResponseStatus = "error"
)

// BlockingRuntimeHookResponse foo bar.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type BlockingRuntimeHookResponse struct {
	metav1.TypeMeta `json:",inline"`

	Status            ResponseStatus `json:"status"`
	RetryAfterSeconds int            `json:"retryAfterSeconds"`
	Message           string         `json:"message"`
}

func (b *BlockingRuntimeHookResponse) DeepCopyObject() runtime.Object {
	return b
}

// NonBlockingRuntimeHookResponse foo bar.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type NonBlockingRuntimeResponse struct {
	metav1.TypeMeta `json:",inline"`

	Status  ResponseStatus `json:"status"`
	Message string         `json:"message"`
}

func (n *NonBlockingRuntimeResponse) DeepCopyObject() runtime.Object {
	return n
}

// +k8s:openapi-gen=true
type ClusterInput struct {
	Cluster runtime.RawExtension `json:",inline"`
}

// ClusterLifecycleHookRequest foo bar.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ClusterLifecycleHookRequest struct {
	metav1.TypeMeta `json:",inline"`

	Cluster ClusterInput `json:"cluster"`
}

func (c *ClusterLifecycleHookRequest) DeepCopyObject() runtime.Object {
	return c
}

func BeforeClusterCreate(*ClusterLifecycleHookRequest, *BlockingRuntimeHookResponse) {}

func AfterControlPlaneInitialized(*ClusterLifecycleHookRequest, *NonBlockingRuntimeResponse) {}

// BeforeClusterUpgradeHookRequest foo bar.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type BeforeClusterUpgradeHookRequest struct {
	ClusterLifecycleHookRequest `json:",inline"`
	FromVersion                 string `json:"fromVersion"`
	ToVersion                   string `json:"toVersion"`
}

func (b *BeforeClusterUpgradeHookRequest) DeepCopyObject() runtime.Object {
	return b
}

// AfterUpgradeHookRequest foo bar.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type AfterUpgradeHookRequest struct {
	ClusterLifecycleHookRequest `json:",inline"`
	Version                     string `json:"version"`
}

func (a *AfterUpgradeHookRequest) DeepCopyObject() runtime.Object {
	return a
}

func BeforeClusterUpgradeHook(*BeforeClusterUpgradeHookRequest, *BlockingRuntimeHookResponse) {}

func AfterControlPlaneUpgradeHook(*AfterUpgradeHookRequest, *BlockingRuntimeHookResponse) {}

func AfterClusterUpgradeHook(*AfterUpgradeHookRequest, *NonBlockingRuntimeResponse) {}

func BeforeClusterDeleteHook(*ClusterLifecycleHookRequest, *BlockingRuntimeHookResponse) {}

func init() {
	catalogBuilder.RegisterHook(BeforeClusterCreate, &catalog.HookMeta{
		Summary:     "Called before Cluster topology is created",
		Description: "This blocking hook is called after the Cluster is crated by the user and immediately before all the objects which are part of a Cluster topology are going to be created",
	})

	catalogBuilder.RegisterHook(AfterControlPlaneInitialized, &catalog.HookMeta{
		Summary:     "Called after the Control Plane is available for the first time",
		Description: "This non-blocking hook is called after the ControlPlane for the Cluster is marked as available for the first time",
	})

	catalogBuilder.RegisterHook(BeforeClusterUpgradeHook, &catalog.HookMeta{
		Summary:     "Called before the Cluster being upgrade",
		Description: "This hook is called after the Cluster object has been updated with a new spec.topology.version  by the user, and immediately before the new version is going to be propagated to the Control Plane",
	})

	catalogBuilder.RegisterHook(AfterControlPlaneUpgradeHook, &catalog.HookMeta{
		Summary:     "Called after the Control Plane finished upgrade",
		Description: "This blocking hook is called after the Control Plane has been upgraded to the version specified in spec.topology.version, and immediately before the new version is going to be propagated the MachineDeployments existing in the Cluster",
	})

	catalogBuilder.RegisterHook(AfterClusterUpgradeHook, &catalog.HookMeta{
		Summary:     "Called after the Cluster finished upgrade",
		Description: "This non-blocking hook is called after the Cluster, Control Plane and workers have been upgraded to the version specified in spec.topology.version",
	})

	catalogBuilder.RegisterHook(BeforeClusterDeleteHook, &catalog.HookMeta{
		Summary:     "Called before the Cluster is deleted",
		Description: "This blocking hook is called after the Cluster has been deleted by the user, and immediately before objects existing in the Cluster are going to be deleted",
	})
}
