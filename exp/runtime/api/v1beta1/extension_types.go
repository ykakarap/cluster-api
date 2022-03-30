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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ANCHOR: ExtensionSpec

// ExtensionSpec defines the desired state of Extension.
type ExtensionSpec struct {
	// ClientConfig defines how to communicate with the hook.
	ClientConfig ExtensionClientConfig `json:"clientConfig"`

	// Default to the empty LabelSelector, which matches everything.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// ExtensionClientConfig contains the information to make a TLS
// connection with the extension.
type ExtensionClientConfig struct {
	// URL gives the location of the extension, in standard URL form
	// (`scheme://host:port/path`). Exactly one of `url` or `service`
	// must be specified.
	//
	// The `host` should not refer to a service running in the cluster; use
	// the `service` field instead.
	//
	// Please note that using `localhost` or `127.0.0.1` as a `host` is
	// risky unless you take great care to run this extension on all hosts
	// which run a controller which might need to make calls to this
	// extension. Such installs are likely to be non-portable, i.e., not easy
	// to turn up in a new cluster.
	//
	// The scheme should be "https"; the URL should begin with "https://".
	// "http" is supported for insecure development purposes only.
	//
	// A path is optional, and if present may be any string permissible in
	// a URL. If a path is set it will be used as prefix and the hook-specific
	// path will be appended.
	//
	// Attempting to use a user or basic auth e.g. "user:password@" is not
	// allowed. Fragments ("#...") and query parameters ("?...") are not
	// allowed, either.
	//
	// +optional
	URL *string `json:"url,omitempty"`

	// Service is a reference to the Kubernetes service for this extension.
	// Either `service` or `url` must be specified.
	//
	// If the extension is running within a cluster, then you should use `service`.
	//
	// +optional
	Service *ServiceReference `json:"service,omitempty"`

	// CABundle is a PEM encoded CA bundle which will be used to validate the extension's server certificate.
	// If unspecified, // FIXME: ?
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`
}

// ServiceReference holds a reference to a Kubernetes Service.
type ServiceReference struct {
	// Namespace is the namespace of the service.
	Namespace string `json:"namespace"`

	// Name is the name of the service.
	Name string `json:"name"`

	// Path is an optional URL path which will be sent in any request to
	// this service.  If a path is set it will be used as prefix and the hook-specific
	// path will be appended.
	// +optional
	Path *string `json:"path,omitempty"`

	// Port is the port on the service that hosting extension.
	// Default to 443 for backward compatibility.
	// `port` should be a valid port number (1-65535, inclusive).
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// ANCHOR_END: ExtensionSpec

// ANCHOR: ExtensionStatus

// ExtensionStatus defines the observed state of Extension.
type ExtensionStatus struct {
	// RuntimeExtensions defines the current RuntimeExtensions supported by the Extension.
	RuntimeExtensions []RuntimeExtension `json:"runtimeExtensions,omitempty"`

	// Conditions define the current service state of the Extension.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// RuntimeExtension specifies the details of a particular runtime extension registered by an Extension.
type RuntimeExtension struct {
	// Name is the name of the RuntimeExtension
	Name string `json:"name"`

	// Hook defines the specific runtime event for which this RuntimeExtension calls.
	Hook Hook `json:"hook"`

	// TimeoutSeconds defines the timeout duration for client calls to the Hook
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// FailurePolicy defines how failures in calls to the Hook should be handled by a client.
	FailurePolicy *FailurePolicyType `json:"failurePolicy,omitempty"`
}

// Hook defines the runtime event when the RuntimeExtension is called.
type Hook struct {
	// APIVersion is the Version of the Hook
	APIVersion string `json:"apiVersion"`

	// Name is the name of the hook
	Name string `json:"name"`
}

// FailurePolicyType specifies a failure policy that defines how unrecognized errors from the admission endpoint are handled.
type FailurePolicyType string

const (
	// Ignore means that an error calling the extension is ignored.
	Ignore FailurePolicyType = "Ignore"

	// Fail means that an error calling the extension causes the admission to fail.
	Fail FailurePolicyType = "Fail"
)

// ANCHOR_END: ExtensionStatus

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=extensions,shortName=ext,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Extension"
// +k8s:conversion-gen=false

// Extension is the Schema for the Extensions API.
type Extension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the Extension
	Spec ExtensionSpec `json:"spec,omitempty"`

	// Status is the current state of the Extension
	Status ExtensionStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (e *Extension) GetConditions() clusterv1.Conditions {
	return e.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (e *Extension) SetConditions(conditions clusterv1.Conditions) {
	e.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ExtensionList contains a list of Extension.
type ExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Extension `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Extension{}, &ExtensionList{})
}

const (
	// RuntimeExtensionDiscovered is a condition set on an Extension object once it has been discovered by the Runtime SDK client.
	RuntimeExtensionDiscovered clusterv1.ConditionType = "Discovered"
)
