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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
)

// DiscoveryHookRequest foo bar baz.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type DiscoveryHookRequest struct {
	metav1.TypeMeta `json:",inline"`

	Second string `json:"second"`
	First  int    `json:"first"`
}

// DiscoveryHookResponse foo bar baz.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type DiscoveryHookResponse struct {
	metav1.TypeMeta `json:",inline"`

	Message string `json:"message"`
}

func DiscoveryHook(*DiscoveryHookRequest, *DiscoveryHookResponse) {}

func init() {
	catalogBuilder.RegisterHook(DiscoveryHook, &catalog.HookMeta{
		Tags:        []string{"discovery"},
		Summary:     "Discovery endpoint",
		Description: "Discovery endpoint discovers...",
		Deprecated:  true,
	})
}
