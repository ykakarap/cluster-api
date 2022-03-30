package controllers

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/component-base/featuregate/testing"
	ctrl "sigs.k8s.io/controller-runtime"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1beta1"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	runtimeclient "sigs.k8s.io/cluster-api/internal/runtime/client"
	"sigs.k8s.io/cluster-api/internal/runtime/registry"
)

func TestExtensionReconciler_Reconcile(t *testing.T) {
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.ClusterTopology, true)()
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.RuntimeSDK, true)()

	g := NewWithT(t)
	ns, err := env.CreateNamespace(ctx, "test-runtime-extension")
	g.Expect(err).ToNot(HaveOccurred())
	workingExtension := &runtimev1.Extension{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Extension",
			APIVersion: "runtime.cluster.x-k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "working",
			Namespace: ns.Name,
		},
		Spec: runtimev1.ExtensionSpec{
			ClientConfig:      runtimev1.ExtensionClientConfig{},
			NamespaceSelector: nil,
		},
	}

	brokenExtension := &runtimev1.Extension{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Extension",
			APIVersion: "runtime.cluster.x-k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "broken-extension",
			Namespace: ns.Name,
		},
		Spec: runtimev1.ExtensionSpec{
			ClientConfig:      runtimev1.ExtensionClientConfig{},
			NamespaceSelector: nil,
		},
	}
	hookStore := map[string][]runtimev1.RuntimeExtension{
		namespacedName(workingExtension).String(): {
			runtimev1.RuntimeExtension{
				Name: "first",
				Hook: runtimev1.Hook{
					Name:       "first",
					APIVersion: "v1alpha1",
				},
			},
		},
	}

	t.Run("create and discover extension", func(t *testing.T) {
		var result runtimev1.Extension

		r := &Reconciler{
			Client:    env.GetClient(),
			APIReader: env.GetAPIReader(),
			Registry:  registry.Extensions(),
			RuntimeClient: &fakeClient{
				extensionStore: hookStore,
			},
		}

		// Create and attempt to discover a working, discoverable extension. Expect no error.
		g.Expect(env.CreateAndWait(ctx, workingExtension.DeepCopy())).To(Succeed())
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: namespacedName(workingExtension)})
		g.Expect(err).To(BeNil())
		g.Expect(env.Get(ctx, namespacedName(workingExtension), &result)).To(Succeed())

		g.Expect(result.Status.Conditions[0].Status).To(Equal(corev1.ConditionTrue))

		// Create and attempt to discover a broken, undiscoverable extension. Expect an error.
		g.Expect(env.CreateAndWait(ctx, brokenExtension.DeepCopy())).To(Succeed())
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: namespacedName(brokenExtension)})
		g.Expect(err).NotTo(BeNil())

		// Set reconciler ready to false and recreate all objects to simulate a restart. Expect discovery of working extension with an overall error.
		g.Expect(env.CleanupAndWait(ctx, workingExtension, brokenExtension)).To(Succeed())
	})

	t.Run("discover pre-created extensions", func(t *testing.T) {
		var results runtimev1.ExtensionList
		r := &Reconciler{
			Client:    env.GetClient(),
			APIReader: env.GetAPIReader(),
			RuntimeClient: &fakeClient{
				extensionStore: hookStore,
			},
			Registry: registry.Extensions(),
		}

		g.Expect(env.CreateAndWait(ctx, workingExtension.DeepCopy())).To(Succeed())
		g.Expect(env.CreateAndWait(ctx, brokenExtension.DeepCopy())).To(Succeed())
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: namespacedName(workingExtension)})
		g.Expect(err).NotTo(BeNil())
		g.Expect(r.Registry.IsReady()).To(BeFalse())
		time.Sleep(time.Millisecond * 500)
		g.Expect(env.List(ctx, &results)).To(Succeed())
		for _, extension := range results.Items {
			if extension.Name == workingExtension.Name {
				for _, condition := range extension.GetConditions() {
					if condition.Type == runtimev1.RuntimeExtensionDiscovered {
						g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
					}
					g.Expect(len(extension.Status.RuntimeExtensions)).To(Equal(1))
				}
			}

			if extension.Name == brokenExtension.Name {
				for _, condition := range extension.GetConditions() {
					if condition.Type == runtimev1.RuntimeExtensionDiscovered {
						g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
					}
				}
				g.Expect(len(extension.Status.RuntimeExtensions)).To(Equal(0))
			}
		}
		g.Expect(env.CleanupAndWait(ctx, workingExtension, brokenExtension)).To(Succeed())
	},
	)
}

type fakeClient struct {
	extensionStore map[string][]runtimev1.RuntimeExtension
	ready          bool
}

func (c *fakeClient) Hook(service catalog.Hook) runtimeclient.HookClient {
	panic("not implemented")
}

func (c *fakeClient) IsRegistryReady() bool {
	return c.ready
}

func (c *fakeClient) SetRegistryReady(ready bool) {
	c.ready = ready
}

type fakeExtensionClient struct {
	client *fakeClient
	ext    *runtimev1.Extension
}

func (c *fakeClient) Extension(ext *runtimev1.Extension) runtimeclient.ExtensionClient {
	return fakeExtensionClient{
		client: c,
		ext:    ext,
	}
}

func (e fakeExtensionClient) Discover() ([]runtimev1.RuntimeExtension, error) {
	v, ok := e.client.extensionStore[namespacedName(e.ext).String()]
	if !ok {
		return nil, errors.New("runtimeExtensions could not be found for Extension")
	}
	return v, nil
}

func (e fakeExtensionClient) Unregister() error {
	return errors.New("implement me")
}

func (c *fakeClient) ServiceOld(service catalog.Hook, opts ...runtimeclient.ServiceOption) runtimeclient.ServiceClient {
	panic("not implemented")
}

// namespacedName returns the NamespacedName for the extension.
func namespacedName(e *runtimev1.Extension) types.NamespacedName {
	return types.NamespacedName{
		Namespace: e.Namespace,
		Name:      e.Name,
	}
}
