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

// Package test contains catalog tests
// Note: They have to be outside the catalog package to be realistic. Otherwise using
// test types with different versions would result in a cyclic dependency and thus
// wouldn't be possible.
package test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha1"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog/test/v1alpha2"
)

var c = catalog.New()

func init() {
	_ = v1alpha1.AddToCatalog(c)
	_ = v1alpha2.AddToCatalog(c)
}

func TestCatalog(t *testing.T) {
	g := NewWithT(t)

	verify := func(hook catalog.Hook, expectedGV schema.GroupVersion) {
		// Test GroupVersionHook
		hookGVH, err := c.GroupVersionHook(hook)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hookGVH.GroupVersion()).To(Equal(expectedGV))
		g.Expect(hookGVH.Hook).To(Equal("FakeHook"))

		// Test Request
		requestGVK, err := c.Request(hookGVH)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(requestGVK.GroupVersion()).To(Equal(expectedGV))
		g.Expect(requestGVK.Kind).To(Equal("FakeRequest"))

		// Test Response
		responseGVK, err := c.Response(hookGVH)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(responseGVK.GroupVersion()).To(Equal(expectedGV))
		g.Expect(responseGVK.Kind).To(Equal("FakeResponse"))

		// Test NewRequest
		request, err := c.NewRequest(hookGVH)
		g.Expect(err).ToNot(HaveOccurred())

		// Test NewResponse
		response, err := c.NewResponse(hookGVH)
		g.Expect(err).ToNot(HaveOccurred())

		// Test ValidateRequest/ValidateResponse
		g.Expect(c.ValidateRequest(hookGVH, request)).To(Succeed())
		g.Expect(c.ValidateResponse(hookGVH, response)).To(Succeed())
	}

	verify(v1alpha1.FakeHook, v1alpha1.GroupVersion)
	verify(v1alpha2.FakeHook, v1alpha2.GroupVersion)
}
