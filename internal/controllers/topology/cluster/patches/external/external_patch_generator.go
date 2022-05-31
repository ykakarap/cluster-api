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

// Package external implements the external patch generator.
package external

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/patches/api"
	runtimeclient "sigs.k8s.io/cluster-api/internal/runtime/client"
)

// externalPatchGenerator generates JSON patches for a GeneratePatchesRequest based on a ClusterClassPatch.
type externalPatchGenerator struct {
	runtimeClient runtimeclient.Client
	patch         *clusterv1.ClusterClassPatch
}

// New returns a new external Generator from a given ClusterClassPatch object.
func New(runtimeClient runtimeclient.Client, patch *clusterv1.ClusterClassPatch) api.Generator {
	return &externalPatchGenerator{
		runtimeClient: runtimeClient,
		patch:         patch,
	}
}

func (e externalPatchGenerator) Generate(ctx context.Context, req *runtimehooksv1.GeneratePatchesRequest) (*runtimehooksv1.GeneratePatchesResponse, error) {
	resp := &runtimehooksv1.GeneratePatchesResponse{}
	// FIXME(sbueringer): CallExtension  and CallAllExtensions should check if registry is ready internally.
	err := e.runtimeClient.CallExtension(ctx, runtimehooksv1.GeneratePatches, *e.patch.External.GenerateExtension, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
