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

// externalValidator validates templates.
type externalValidator struct {
	runtimeClient runtimeclient.Client
	patch         *clusterv1.ClusterClassPatch
}

// NewValidator returns a new external Validator from a given ClusterClassPatch object.
func NewValidator(runtimeClient runtimeclient.Client, patch *clusterv1.ClusterClassPatch) api.Validator {
	return &externalValidator{
		runtimeClient: runtimeClient,
		patch:         patch,
	}
}

func (e externalValidator) Validate(ctx context.Context, req *runtimehooksv1.ValidateTopologyRequest) (*runtimehooksv1.ValidateTopologyResponse, error) {
	resp := &runtimehooksv1.ValidateTopologyResponse{}
	err := e.runtimeClient.CallExtension(ctx, runtimehooksv1.ValidateTopology, *e.patch.External.ValidateExtension, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}