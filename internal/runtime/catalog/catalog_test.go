package catalog

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	group   = "fake.cluster.x-k8s.io"
	version = "v1alpha1"
)

func TestCatalog(t *testing.T) {
	g := NewWithT(t)

	// Create catalog and add the FakeHook.
	c := New()
	g.Expect(AddToCatalog(c)).To(Succeed())

	// Test GroupVersionHook
	hookGVH, err := c.GroupVersionHook(FakeHook)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(hookGVH.Group).To(Equal(group))
	g.Expect(hookGVH.Version).To(Equal(version))
	g.Expect(hookGVH.Hook).To(Equal("FakeHook"))

	// Test Request
	requestGVK, err := c.Request(hookGVH)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(requestGVK.Group).To(Equal(group))
	g.Expect(requestGVK.Version).To(Equal(version))
	g.Expect(requestGVK.Kind).To(Equal("FakeRequest"))

	// Test Response
	responseGVK, err := c.Response(hookGVH)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(responseGVK.Group).To(Equal(group))
	g.Expect(responseGVK.Version).To(Equal(version))
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

	// FIXME: different version
}

var (
	FakeGroupVersion = schema.GroupVersion{Group: group, Version: version}

	catalogBuilder = Builder{GroupVersion: FakeGroupVersion}

	AddToCatalog = catalogBuilder.AddToCatalog
)

func FakeHook(*FakeRequest, *FakeResponse) {}

type FakeRequest struct {
	metav1.TypeMeta `json:",inline"`

	Second string
	First  int
}

func (in *FakeRequest) DeepCopyObject() runtime.Object {
	panic("implement me!")
}

type FakeResponse struct {
	metav1.TypeMeta `json:",inline"`

	Second string
	First  int
}

func (in *FakeResponse) DeepCopyObject() runtime.Object {
	panic("implement me!")
}

func init() {
	catalogBuilder.RegisterHook(FakeHook, &HookMeta{
		Tags:        []string{"fake-tag"},
		Summary:     "Fake summary",
		Description: "Fake description",
		Deprecated:  true,
	})
}
