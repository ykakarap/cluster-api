//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AfterClusterUpgradeRequest) DeepCopyInto(out *AfterClusterUpgradeRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AfterClusterUpgradeRequest.
func (in *AfterClusterUpgradeRequest) DeepCopy() *AfterClusterUpgradeRequest {
	if in == nil {
		return nil
	}
	out := new(AfterClusterUpgradeRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AfterClusterUpgradeRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AfterClusterUpgradeResponse) DeepCopyInto(out *AfterClusterUpgradeResponse) {
	*out = *in
	out.NonBlockingResponse = in.NonBlockingResponse
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AfterClusterUpgradeResponse.
func (in *AfterClusterUpgradeResponse) DeepCopy() *AfterClusterUpgradeResponse {
	if in == nil {
		return nil
	}
	out := new(AfterClusterUpgradeResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AfterClusterUpgradeResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AfterControlPlaneInitializedRequest) DeepCopyInto(out *AfterControlPlaneInitializedRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AfterControlPlaneInitializedRequest.
func (in *AfterControlPlaneInitializedRequest) DeepCopy() *AfterControlPlaneInitializedRequest {
	if in == nil {
		return nil
	}
	out := new(AfterControlPlaneInitializedRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AfterControlPlaneInitializedRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AfterControlPlaneInitializedResponse) DeepCopyInto(out *AfterControlPlaneInitializedResponse) {
	*out = *in
	out.NonBlockingResponse = in.NonBlockingResponse
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AfterControlPlaneInitializedResponse.
func (in *AfterControlPlaneInitializedResponse) DeepCopy() *AfterControlPlaneInitializedResponse {
	if in == nil {
		return nil
	}
	out := new(AfterControlPlaneInitializedResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AfterControlPlaneInitializedResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AfterControlPlaneUpgradeRequest) DeepCopyInto(out *AfterControlPlaneUpgradeRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AfterControlPlaneUpgradeRequest.
func (in *AfterControlPlaneUpgradeRequest) DeepCopy() *AfterControlPlaneUpgradeRequest {
	if in == nil {
		return nil
	}
	out := new(AfterControlPlaneUpgradeRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AfterControlPlaneUpgradeRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AfterControlPlaneUpgradeResponse) DeepCopyInto(out *AfterControlPlaneUpgradeResponse) {
	*out = *in
	out.BlockingResponse = in.BlockingResponse
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AfterControlPlaneUpgradeResponse.
func (in *AfterControlPlaneUpgradeResponse) DeepCopy() *AfterControlPlaneUpgradeResponse {
	if in == nil {
		return nil
	}
	out := new(AfterControlPlaneUpgradeResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AfterControlPlaneUpgradeResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BeforeClusterCreateRequest) DeepCopyInto(out *BeforeClusterCreateRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BeforeClusterCreateRequest.
func (in *BeforeClusterCreateRequest) DeepCopy() *BeforeClusterCreateRequest {
	if in == nil {
		return nil
	}
	out := new(BeforeClusterCreateRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BeforeClusterCreateRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BeforeClusterCreateResponse) DeepCopyInto(out *BeforeClusterCreateResponse) {
	*out = *in
	out.BlockingResponse = in.BlockingResponse
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BeforeClusterCreateResponse.
func (in *BeforeClusterCreateResponse) DeepCopy() *BeforeClusterCreateResponse {
	if in == nil {
		return nil
	}
	out := new(BeforeClusterCreateResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BeforeClusterCreateResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BeforeClusterDeleteRequest) DeepCopyInto(out *BeforeClusterDeleteRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BeforeClusterDeleteRequest.
func (in *BeforeClusterDeleteRequest) DeepCopy() *BeforeClusterDeleteRequest {
	if in == nil {
		return nil
	}
	out := new(BeforeClusterDeleteRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BeforeClusterDeleteRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BeforeClusterDeleteResponse) DeepCopyInto(out *BeforeClusterDeleteResponse) {
	*out = *in
	out.BlockingResponse = in.BlockingResponse
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BeforeClusterDeleteResponse.
func (in *BeforeClusterDeleteResponse) DeepCopy() *BeforeClusterDeleteResponse {
	if in == nil {
		return nil
	}
	out := new(BeforeClusterDeleteResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BeforeClusterDeleteResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BeforeClusterUpgradeRequest) DeepCopyInto(out *BeforeClusterUpgradeRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BeforeClusterUpgradeRequest.
func (in *BeforeClusterUpgradeRequest) DeepCopy() *BeforeClusterUpgradeRequest {
	if in == nil {
		return nil
	}
	out := new(BeforeClusterUpgradeRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BeforeClusterUpgradeRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BeforeClusterUpgradeResponse) DeepCopyInto(out *BeforeClusterUpgradeResponse) {
	*out = *in
	out.BlockingResponse = in.BlockingResponse
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BeforeClusterUpgradeResponse.
func (in *BeforeClusterUpgradeResponse) DeepCopy() *BeforeClusterUpgradeResponse {
	if in == nil {
		return nil
	}
	out := new(BeforeClusterUpgradeResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BeforeClusterUpgradeResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BlockingResponse) DeepCopyInto(out *BlockingResponse) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BlockingResponse.
func (in *BlockingResponse) DeepCopy() *BlockingResponse {
	if in == nil {
		return nil
	}
	out := new(BlockingResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BlockingResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiscoveryHookRequest) DeepCopyInto(out *DiscoveryHookRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiscoveryHookRequest.
func (in *DiscoveryHookRequest) DeepCopy() *DiscoveryHookRequest {
	if in == nil {
		return nil
	}
	out := new(DiscoveryHookRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DiscoveryHookRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiscoveryHookResponse) DeepCopyInto(out *DiscoveryHookResponse) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiscoveryHookResponse.
func (in *DiscoveryHookResponse) DeepCopy() *DiscoveryHookResponse {
	if in == nil {
		return nil
	}
	out := new(DiscoveryHookResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DiscoveryHookResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GeneratePatchesRequest) DeepCopyInto(out *GeneratePatchesRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]Variable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GeneratePatchesRequestItem, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GeneratePatchesRequest.
func (in *GeneratePatchesRequest) DeepCopy() *GeneratePatchesRequest {
	if in == nil {
		return nil
	}
	out := new(GeneratePatchesRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GeneratePatchesRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GeneratePatchesRequestItem) DeepCopyInto(out *GeneratePatchesRequestItem) {
	*out = *in
	out.HolderReference = in.HolderReference
	in.Object.DeepCopyInto(&out.Object)
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]Variable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GeneratePatchesRequestItem.
func (in *GeneratePatchesRequestItem) DeepCopy() *GeneratePatchesRequestItem {
	if in == nil {
		return nil
	}
	out := new(GeneratePatchesRequestItem)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GeneratePatchesResponse) DeepCopyInto(out *GeneratePatchesResponse) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GeneratePatchesResponseItem, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GeneratePatchesResponse.
func (in *GeneratePatchesResponse) DeepCopy() *GeneratePatchesResponse {
	if in == nil {
		return nil
	}
	out := new(GeneratePatchesResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GeneratePatchesResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GeneratePatchesResponseItem) DeepCopyInto(out *GeneratePatchesResponseItem) {
	*out = *in
	if in.Patch != nil {
		in, out := &in.Patch, &out.Patch
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GeneratePatchesResponseItem.
func (in *GeneratePatchesResponseItem) DeepCopy() *GeneratePatchesResponseItem {
	if in == nil {
		return nil
	}
	out := new(GeneratePatchesResponseItem)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HolderReference) DeepCopyInto(out *HolderReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HolderReference.
func (in *HolderReference) DeepCopy() *HolderReference {
	if in == nil {
		return nil
	}
	out := new(HolderReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NonBlockingResponse) DeepCopyInto(out *NonBlockingResponse) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NonBlockingResponse.
func (in *NonBlockingResponse) DeepCopy() *NonBlockingResponse {
	if in == nil {
		return nil
	}
	out := new(NonBlockingResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NonBlockingResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidateTopologyRequest) DeepCopyInto(out *ValidateTopologyRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]Variable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]*ValidateTopologyRequestItem, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ValidateTopologyRequestItem)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidateTopologyRequest.
func (in *ValidateTopologyRequest) DeepCopy() *ValidateTopologyRequest {
	if in == nil {
		return nil
	}
	out := new(ValidateTopologyRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ValidateTopologyRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidateTopologyRequestItem) DeepCopyInto(out *ValidateTopologyRequestItem) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.HolderReference = in.HolderReference
	in.Object.DeepCopyInto(&out.Object)
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]Variable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidateTopologyRequestItem.
func (in *ValidateTopologyRequestItem) DeepCopy() *ValidateTopologyRequestItem {
	if in == nil {
		return nil
	}
	out := new(ValidateTopologyRequestItem)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidateTopologyResponse) DeepCopyInto(out *ValidateTopologyResponse) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidateTopologyResponse.
func (in *ValidateTopologyResponse) DeepCopy() *ValidateTopologyResponse {
	if in == nil {
		return nil
	}
	out := new(ValidateTopologyResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ValidateTopologyResponse) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Variable) DeepCopyInto(out *Variable) {
	*out = *in
	in.Value.DeepCopyInto(&out.Value)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Variable.
func (in *Variable) DeepCopy() *Variable {
	if in == nil {
		return nil
	}
	out := new(Variable)
	in.DeepCopyInto(out)
	return out
}
