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

// Package test is used to help with testing functions that need a fake RuntimeClient.
package test

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1alpha1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	runtimecatalog "sigs.k8s.io/cluster-api/internal/runtime/catalog"
	runtimeclient "sigs.k8s.io/cluster-api/internal/runtime/client"
)

type FakeRuntimeClientBuilder struct {
	ready            bool
	catalog          *runtimecatalog.Catalog
	callAllResponses map[runtimecatalog.GroupVersionHook]runtimehooksv1.ResponseObject
	callResponses    map[string]runtimehooksv1.ResponseObject
}

// DefaultTestCatalog is a catalog with the runtime hooks already registered.
var DefaultTestCatalog = runtimecatalog.New()

func init() {
	_ = runtimehooksv1.AddToCatalog(DefaultTestCatalog)
}

func NewFakeRuntimeClientBuilder() *FakeRuntimeClientBuilder {
	return &FakeRuntimeClientBuilder{}
}

func (f *FakeRuntimeClientBuilder) WithCatalog(catalog *runtimecatalog.Catalog) *FakeRuntimeClientBuilder {
	f.catalog = catalog
	return nil
}

func (f *FakeRuntimeClientBuilder) WithCallAllExtensionResponses(responses map[runtimecatalog.GroupVersionHook]runtimehooksv1.ResponseObject) *FakeRuntimeClientBuilder {
	f.callAllResponses = responses
	return f
}

func (f *FakeRuntimeClientBuilder) WithCallExtensionResponses(responses map[string]runtimehooksv1.ResponseObject) *FakeRuntimeClientBuilder {
	f.callResponses = responses
	return f
}

func (f *FakeRuntimeClientBuilder) MarkReady(ready bool) *FakeRuntimeClientBuilder {
	f.ready = ready
	return f
}

func (f *FakeRuntimeClientBuilder) Build() runtimeclient.Client {
	if f.catalog == nil {
		f.catalog = DefaultTestCatalog
	}
	return &fakeRuntimeClient{
		isReady:          f.ready,
		callAllResponses: f.callAllResponses,
		callResponses:    f.callResponses,
		catalog:          f.catalog,
	}
}

var _ runtimeclient.Client = &fakeRuntimeClient{}

type fakeRuntimeClient struct {
	isReady          bool
	catalog          *runtimecatalog.Catalog
	callAllResponses map[runtimecatalog.GroupVersionHook]runtimehooksv1.ResponseObject
	callResponses    map[string]runtimehooksv1.ResponseObject
}

// CallAllExtensions implements Client.
func (fc *fakeRuntimeClient) CallAllExtensions(ctx context.Context, hook runtimecatalog.Hook, request runtime.Object, response runtimehooksv1.ResponseObject) error {
	gvh, err := fc.catalog.GroupVersionHook(hook)
	if err != nil {
		return errors.Wrap(err, "failed to compute GVH")
	}
	expectedResponse, ok := fc.callAllResponses[gvh]
	if !ok {
		// This should actually panic because an error here would mean a mistake in the test setup.
		panic(fmt.Sprintf("test response not available hook for %q", gvh))
	}
	if err := fc.catalog.Convert(expectedResponse, response, ctx); err != nil {
		// This should actually panic because an error here would mean a mistake in the test setup.
		panic("cannot update response")
	}
	if response.GetStatus() == runtimehooksv1.ResponseStatusFailure {
		return errors.Errorf("runtime hook %q failed", gvh)
	}
	return nil
}

// CallExtension implements Client.
func (fc *fakeRuntimeClient) CallExtension(ctx context.Context, _ runtimecatalog.Hook, name string, request runtime.Object, response runtimehooksv1.ResponseObject) error {
	expectedResponse, ok := fc.callResponses[name]
	if !ok {
		// This should actually panic because an error here would mean a mistake in the test setup.
		panic(fmt.Sprintf("test response not available for extension %q", name))
	}
	if err := fc.catalog.Convert(expectedResponse, response, ctx); err != nil {
		// This should actually panic because an error here would mean a mistake in the test setup.
		panic("cannot update response")
	}
	return nil
}

// Discover implements Client.
func (fc *fakeRuntimeClient) Discover(context.Context, *runtimev1.ExtensionConfig) (*runtimev1.ExtensionConfig, error) {
	panic("unimplemented")
}

// IsReady implements Client.
func (fc *fakeRuntimeClient) IsReady() bool {
	return fc.isReady
}

// Register implements Client.
func (fc *fakeRuntimeClient) Register(extensionConfig *runtimev1.ExtensionConfig) error {
	panic("unimplemented")
}

// Unregister implements Client.
func (fc *fakeRuntimeClient) Unregister(extensionConfig *runtimev1.ExtensionConfig) error {
	panic("unimplemented")
}

func (fc *fakeRuntimeClient) WarmUp(extensionConfigList *runtimev1.ExtensionConfigList) error {
	panic("unimplemented")
}
