/*
Copyright 2020 The Kubernetes Authors.

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

package controllers

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/test/builder"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/secret"
)

func TestReconcileKubeconfigEmptyAPIEndpoints(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmControlPlane",
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version: "v1.16.6",
		},
	}
	clusterName := client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "foo"}

	fakeClient := newFakeClient(kcp.DeepCopy())
	r := &KubeadmControlPlaneReconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}

	result, err := r.reconcileKubeconfig(ctx, cluster, kcp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeZero())

	kubeconfigSecret := &corev1.Secret{}
	secretName := client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      secret.Name(clusterName.Name, secret.Kubeconfig),
	}
	g.Expect(r.Client.Get(ctx, secretName, kubeconfigSecret)).To(MatchError(ContainSubstring("not found")))
}

func TestReconcileKubeconfigMissingCACertificate(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "test.local", Port: 8443},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmControlPlane",
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version: "v1.16.6",
		},
	}

	fakeClient := newFakeClient(kcp.DeepCopy())
	r := &KubeadmControlPlaneReconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}

	result, err := r.reconcileKubeconfig(ctx, cluster, kcp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(ctrl.Result{RequeueAfter: dependentCertRequeueAfter}))

	kubeconfigSecret := &corev1.Secret{}
	secretName := client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      secret.Name(cluster.Name, secret.Kubeconfig),
	}
	g.Expect(r.Client.Get(ctx, secretName, kubeconfigSecret)).To(MatchError(ContainSubstring("not found")))
}

func TestReconcileKubeconfigSecretAdoptsV1alpha2Secrets(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "test.local", Port: 8443},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmControlPlane",
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version: "v1.16.6",
		},
	}

	existingKubeconfigSecret := kubeconfig.GenerateSecretWithOwner(
		client.ObjectKey{Name: "foo", Namespace: metav1.NamespaceDefault},
		[]byte{},
		metav1.OwnerReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       cluster.Name,
			UID:        cluster.UID,
		}, // the Cluster ownership defines v1alpha2 controlled secrets
	)

	fakeClient := newFakeClient(kcp.DeepCopy(), existingKubeconfigSecret.DeepCopy())
	r := &KubeadmControlPlaneReconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}

	result, err := r.reconcileKubeconfig(ctx, cluster, kcp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(ctrl.Result{}))

	kubeconfigSecret := &corev1.Secret{}
	secretName := client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      secret.Name(cluster.Name, secret.Kubeconfig),
	}
	g.Expect(r.Client.Get(ctx, secretName, kubeconfigSecret)).To(Succeed())
	g.Expect(kubeconfigSecret.Labels).To(Equal(existingKubeconfigSecret.Labels))
	g.Expect(kubeconfigSecret.Data).To(Equal(existingKubeconfigSecret.Data))
	g.Expect(kubeconfigSecret.OwnerReferences).ToNot(ContainElement(metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}))
	g.Expect(kubeconfigSecret.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"))))
}

func TestReconcileKubeconfigSecretDoesNotAdoptsUserSecrets(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "test.local", Port: 8443},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmControlPlane",
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version: "v1.16.6",
		},
	}

	existingKubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name("foo", secret.Kubeconfig),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "foo",
			},
			OwnerReferences: []metav1.OwnerReference{},
		},
		Data: map[string][]byte{
			secret.KubeconfigDataName: {},
		},
		// KCP identifies CAPI-created Secrets using the clusterv1.ClusterSecretType. Setting any other type allows
		// the controllers to treat it as a user-provided Secret.
		Type: corev1.SecretTypeOpaque,
	}

	fakeClient := newFakeClient(kcp.DeepCopy(), existingKubeconfigSecret.DeepCopy())
	r := &KubeadmControlPlaneReconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}

	result, err := r.reconcileKubeconfig(ctx, cluster, kcp)
	g.Expect(err).To(Succeed())
	g.Expect(result).To(BeZero())

	kubeconfigSecret := &corev1.Secret{}
	secretName := client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      secret.Name(cluster.Name, secret.Kubeconfig),
	}
	g.Expect(r.Client.Get(ctx, secretName, kubeconfigSecret)).To(Succeed())
	g.Expect(kubeconfigSecret.Labels).To(Equal(existingKubeconfigSecret.Labels))
	g.Expect(kubeconfigSecret.Data).To(Equal(existingKubeconfigSecret.Data))
	g.Expect(kubeconfigSecret.OwnerReferences).ToNot(ContainElement(*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"))))
}

func TestKubeadmControlPlaneReconciler_reconcileKubeconfig(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "test.local", Port: 8443},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmControlPlane",
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version: "v1.16.6",
		},
	}

	clusterCerts := secret.NewCertificatesForInitialControlPlane(&bootstrapv1.ClusterConfiguration{})
	g.Expect(clusterCerts.Generate()).To(Succeed())
	caCert := clusterCerts.GetByPurpose(secret.ClusterCA)
	existingCACertSecret := caCert.AsSecret(
		client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "foo"},
		*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane")),
	)

	fakeClient := newFakeClient(kcp.DeepCopy(), existingCACertSecret.DeepCopy())
	r := &KubeadmControlPlaneReconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}
	result, err := r.reconcileKubeconfig(ctx, cluster, kcp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(ctrl.Result{}))

	kubeconfigSecret := &corev1.Secret{}
	secretName := client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      secret.Name(cluster.Name, secret.Kubeconfig),
	}
	g.Expect(r.Client.Get(ctx, secretName, kubeconfigSecret)).To(Succeed())
	g.Expect(kubeconfigSecret.OwnerReferences).NotTo(BeEmpty())
	g.Expect(kubeconfigSecret.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"))))
	g.Expect(kubeconfigSecret.Labels).To(HaveKeyWithValue(clusterv1.ClusterNameLabel, cluster.Name))
}

func TestCloneConfigsAndGenerateMachine(t *testing.T) {
	setup := func(t *testing.T, g *WithT) *corev1.Namespace {
		t.Helper()

		t.Log("Creating the namespace")
		ns, err := env.CreateNamespace(ctx, "test-applykubeadmconfig")
		g.Expect(err).To(BeNil())

		return ns
	}

	teardown := func(t *testing.T, g *WithT, ns *corev1.Namespace) {
		t.Helper()

		t.Log("Deleting the namespace")
		g.Expect(env.Delete(ctx, ns)).To(Succeed())
	}

	g := NewWithT(t)
	namespace := setup(t, g)
	defer teardown(t, g, namespace)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: namespace.Name,
		},
	}

	genericInfrastructureMachineTemplate := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "GenericInfrastructureMachineTemplate",
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"metadata": map[string]interface{}{
				"name":      "infra-foo",
				"namespace": cluster.Namespace,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"hello": "world",
					},
				},
			},
		},
	}
	g.Expect(env.Create(ctx, genericInfrastructureMachineTemplate)).To(Succeed())

	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kcp-foo",
			Namespace: cluster.Namespace,
			UID:       "abc-123-kcp-control-plane",
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					Kind:       genericInfrastructureMachineTemplate.GetKind(),
					APIVersion: genericInfrastructureMachineTemplate.GetAPIVersion(),
					Name:       genericInfrastructureMachineTemplate.GetName(),
					Namespace:  cluster.Namespace,
				},
			},
			Version: "v1.16.6",
		},
	}

	r := &KubeadmControlPlaneReconciler{
		Client:   env,
		recorder: record.NewFakeRecorder(32),
	}

	bootstrapSpec := &bootstrapv1.KubeadmConfigSpec{
		JoinConfiguration: &bootstrapv1.JoinConfiguration{},
	}
	g.Expect(r.cloneConfigsAndGenerateMachine(ctx, cluster, kcp, bootstrapSpec, nil)).To(Succeed())

	machineList := &clusterv1.MachineList{}
	g.Expect(env.List(ctx, machineList, client.InNamespace(cluster.Namespace))).To(Succeed())
	g.Expect(machineList.Items).To(HaveLen(1))

	for _, m := range machineList.Items {
		g.Expect(m.Namespace).To(Equal(cluster.Namespace))
		g.Expect(m.Name).NotTo(BeEmpty())
		g.Expect(m.Name).To(HavePrefix(kcp.Name))

		infraObj, err := external.Get(ctx, r.Client, &m.Spec.InfrastructureRef, m.Spec.InfrastructureRef.Namespace)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(infraObj.GetAnnotations()).To(HaveKeyWithValue(clusterv1.TemplateClonedFromNameAnnotation, genericInfrastructureMachineTemplate.GetName()))
		g.Expect(infraObj.GetAnnotations()).To(HaveKeyWithValue(clusterv1.TemplateClonedFromGroupKindAnnotation, genericInfrastructureMachineTemplate.GroupVersionKind().GroupKind().String()))

		g.Expect(m.Spec.InfrastructureRef.Namespace).To(Equal(cluster.Namespace))
		g.Expect(m.Spec.InfrastructureRef.Name).To(HavePrefix(genericInfrastructureMachineTemplate.GetName()))
		g.Expect(m.Spec.InfrastructureRef.APIVersion).To(Equal(genericInfrastructureMachineTemplate.GetAPIVersion()))
		g.Expect(m.Spec.InfrastructureRef.Kind).To(Equal("GenericInfrastructureMachine"))

		g.Expect(m.Spec.Bootstrap.ConfigRef.Namespace).To(Equal(cluster.Namespace))
		g.Expect(m.Spec.Bootstrap.ConfigRef.Name).To(HavePrefix(kcp.Name))
		g.Expect(m.Spec.Bootstrap.ConfigRef.APIVersion).To(Equal(bootstrapv1.GroupVersion.String()))
		g.Expect(m.Spec.Bootstrap.ConfigRef.Kind).To(Equal("KubeadmConfig"))
	}
}

func TestCloneConfigsAndGenerateMachineFail(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
	}

	genericMachineTemplate := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "GenericMachineTemplate",
			"apiVersion": "generic.io/v1",
			"metadata": map[string]interface{}{
				"name":      "infra-foo",
				"namespace": cluster.Namespace,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"hello": "world",
					},
				},
			},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kcp-foo",
			Namespace: cluster.Namespace,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					Kind:       genericMachineTemplate.GetKind(),
					APIVersion: genericMachineTemplate.GetAPIVersion(),
					Name:       genericMachineTemplate.GetName(),
					Namespace:  cluster.Namespace,
				},
			},
			Version: "v1.16.6",
		},
	}

	fakeClient := newFakeClient(cluster.DeepCopy(), kcp.DeepCopy(), genericMachineTemplate.DeepCopy())

	r := &KubeadmControlPlaneReconciler{
		Client:   fakeClient,
		recorder: record.NewFakeRecorder(32),
	}

	bootstrapSpec := &bootstrapv1.KubeadmConfigSpec{
		JoinConfiguration: &bootstrapv1.JoinConfiguration{},
	}

	// Try to break Infra Cloning
	kcp.Spec.MachineTemplate.InfrastructureRef.Name = "something_invalid"
	g.Expect(r.cloneConfigsAndGenerateMachine(ctx, cluster, kcp, bootstrapSpec, nil)).To(HaveOccurred())
	g.Expect(&kcp.GetConditions()[0]).Should(conditions.HaveSameStateOf(&clusterv1.Condition{
		Type:     controlplanev1.MachinesCreatedCondition,
		Status:   corev1.ConditionFalse,
		Severity: clusterv1.ConditionSeverityError,
		Reason:   controlplanev1.InfrastructureTemplateCloningFailedReason,
		Message:  "failed to create Infrastructure Machine object from Template default/something_invalid: failed to get template default/something_invalid: failed to retrieve GenericMachineTemplate external object \"default\"/\"something_invalid\": genericmachinetemplates.generic.io \"something_invalid\" not found",
	}))
}

func TestKubeadmControlPlaneReconciler_computeDesiredMachine(t *testing.T) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: metav1.NamespaceDefault,
		},
	}

	duration5s := &metav1.Duration{Duration: 5 * time.Second}
	duration10s := &metav1.Duration{Duration: 10 * time.Second}

	kcpMachineTemplateObjectMeta := clusterv1.ObjectMeta{
		Labels: map[string]string{
			"machineTemplateLabel": "machineTemplateLabelValue",
		},
		Annotations: map[string]string{
			"machineTemplateAnnotation": "machineTemplateAnnotationValue",
		},
	}
	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testControlPlane",
			Namespace: cluster.Namespace,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version: "v1.16.6",
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				ObjectMeta:              kcpMachineTemplateObjectMeta,
				NodeDrainTimeout:        duration5s,
				NodeDeletionTimeout:     duration5s,
				NodeVolumeDetachTimeout: duration5s,
			},
		},
	}

	infraRef := &corev1.ObjectReference{
		Kind:       "InfraKind",
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		Name:       "infra",
		Namespace:  cluster.Namespace,
	}
	bootstrapRef := &corev1.ObjectReference{
		Kind:       "BootstrapKind",
		APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
		Name:       "bootstrap",
		Namespace:  cluster.Namespace,
	}

	t.Run("should return the correct Machine object when creating a new Machine", func(t *testing.T) {
		g := NewWithT(t)

		failureDomain := pointer.String("fd1")
		createdMachine, err := (&KubeadmControlPlaneReconciler{}).computeDesiredMachine(
			kcp, cluster,
			infraRef, bootstrapRef,
			failureDomain, nil,
		)
		g.Expect(err).To(BeNil())

		expectedMachineSpec := clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			Version:     pointer.String(kcp.Spec.Version),
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: bootstrapRef.DeepCopy(),
			},
			InfrastructureRef:       *infraRef.DeepCopy(),
			FailureDomain:           failureDomain,
			NodeDrainTimeout:        kcp.Spec.MachineTemplate.NodeDrainTimeout.DeepCopy(),
			NodeDeletionTimeout:     kcp.Spec.MachineTemplate.NodeDeletionTimeout.DeepCopy(),
			NodeVolumeDetachTimeout: kcp.Spec.MachineTemplate.NodeVolumeDetachTimeout.DeepCopy(),
		}
		g.Expect(createdMachine.Name).To(HavePrefix(kcp.Name))
		g.Expect(createdMachine.Namespace).To(Equal(kcp.Namespace))
		g.Expect(createdMachine.OwnerReferences).To(HaveLen(1))
		g.Expect(createdMachine.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"))))
		g.Expect(createdMachine.Spec).To(Equal(expectedMachineSpec))

		// Verify that the machineTemplate.ObjectMeta has been propagated to the Machine.
		for k, v := range kcpMachineTemplateObjectMeta.Labels {
			g.Expect(createdMachine.Labels[k]).To(Equal(v))
		}
		g.Expect(createdMachine.Labels[clusterv1.ClusterNameLabel]).To(Equal(cluster.Name))
		g.Expect(createdMachine.Labels[clusterv1.MachineControlPlaneLabel]).To(Equal(""))
		g.Expect(createdMachine.Labels[clusterv1.MachineControlPlaneNameLabel]).To(Equal(kcp.Name))

		for k, v := range kcpMachineTemplateObjectMeta.Annotations {
			g.Expect(createdMachine.Annotations[k]).To(Equal(v))
		}

		// Verify the new Machine has the ClusterConfiguration annotation.
		g.Expect(createdMachine.Annotations).To(HaveKey(controlplanev1.KubeadmClusterConfigurationAnnotation))

		// Verify that machineTemplate.ObjectMeta in KCP has not been modified.
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).NotTo(HaveKey(clusterv1.ClusterNameLabel))
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).NotTo(HaveKey(clusterv1.MachineControlPlaneLabel))
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).NotTo(HaveKey(clusterv1.MachineControlPlaneNameLabel))
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Annotations).NotTo(HaveKey(controlplanev1.KubeadmClusterConfigurationAnnotation))
	})

	t.Run("should return the correct Machine object when updating an existing Machine", func(t *testing.T) {
		g := NewWithT(t)

		machineName := "existing-machine"
		machineUID := types.UID("abc-123-existing-machine")
		clusterConfigurationString := "cluster-configuration-information"
		failureDomain := pointer.String("fd-1")
		machineVersion := pointer.String("v1.25.3")
		exitingMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name: machineName,
				UID:  machineUID,
				Annotations: map[string]string{
					controlplanev1.KubeadmClusterConfigurationAnnotation: clusterConfigurationString,
				},
			},
			Spec: clusterv1.MachineSpec{
				Version:                 machineVersion,
				FailureDomain:           failureDomain,
				NodeDrainTimeout:        duration10s,
				NodeDeletionTimeout:     duration10s,
				NodeVolumeDetachTimeout: duration10s,
			},
		}

		updatedMachine, err := (&KubeadmControlPlaneReconciler{}).computeDesiredMachine(
			kcp, cluster,
			infraRef, bootstrapRef,
			exitingMachine.Spec.FailureDomain, exitingMachine,
		)
		g.Expect(err).To(BeNil())

		expectedMachineSpec := clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			Version:     machineVersion, // Should use the Machine version and not the version from KCP.
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: bootstrapRef.DeepCopy(),
			},
			InfrastructureRef:       *infraRef.DeepCopy(),
			FailureDomain:           failureDomain,
			NodeDrainTimeout:        kcp.Spec.MachineTemplate.NodeDrainTimeout.DeepCopy(),
			NodeDeletionTimeout:     kcp.Spec.MachineTemplate.NodeDeletionTimeout.DeepCopy(),
			NodeVolumeDetachTimeout: kcp.Spec.MachineTemplate.NodeVolumeDetachTimeout.DeepCopy(),
		}
		g.Expect(updatedMachine.Namespace).To(Equal(kcp.Namespace))
		g.Expect(updatedMachine.OwnerReferences).To(HaveLen(1))
		g.Expect(updatedMachine.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"))))
		g.Expect(updatedMachine.Spec).To(Equal(expectedMachineSpec))

		// Verify the Name and UID of the Machine remain unchanged
		g.Expect(updatedMachine.Name).To(Equal(machineName))
		g.Expect(updatedMachine.UID).To(Equal(machineUID))

		// Verify that the machineTemplate.ObjectMeta has been propagated to the Machine.
		for k, v := range kcpMachineTemplateObjectMeta.Labels {
			g.Expect(updatedMachine.Labels[k]).To(Equal(v))
		}
		g.Expect(updatedMachine.Labels[clusterv1.ClusterNameLabel]).To(Equal(cluster.Name))
		g.Expect(updatedMachine.Labels[clusterv1.MachineControlPlaneLabel]).To(Equal(""))
		g.Expect(updatedMachine.Labels[clusterv1.MachineControlPlaneNameLabel]).To(Equal(kcp.Name))

		for k, v := range kcpMachineTemplateObjectMeta.Annotations {
			g.Expect(updatedMachine.Annotations[k]).To(Equal(v))
		}

		// Verify the ClusterConfiguration annotation is unchanged
		g.Expect(updatedMachine.Annotations).To(HaveKeyWithValue(controlplanev1.KubeadmClusterConfigurationAnnotation, clusterConfigurationString))

		// Verify that machineTemplate.ObjectMeta in KCP has not been modified.
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).NotTo(HaveKey(clusterv1.ClusterNameLabel))
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).NotTo(HaveKey(clusterv1.MachineControlPlaneLabel))
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).NotTo(HaveKey(clusterv1.MachineControlPlaneNameLabel))
		g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Annotations).NotTo(HaveKey(controlplanev1.KubeadmClusterConfigurationAnnotation))
	})
}

func TestKubeadmControlPlaneReconciler_createKubeadmConfig(t *testing.T) {
	setup := func(t *testing.T, g *WithT) *corev1.Namespace {
		t.Helper()

		t.Log("Creating the namespace")
		ns, err := env.CreateNamespace(ctx, "test-applykubeadmconfig")
		g.Expect(err).To(BeNil())

		return ns
	}

	teardown := func(t *testing.T, g *WithT, ns *corev1.Namespace) {
		t.Helper()

		t.Log("Deleting the namespace")
		g.Expect(env.Delete(ctx, ns)).To(Succeed())
	}

	g := NewWithT(t)
	namespace := setup(t, g)
	defer teardown(t, g, namespace)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: namespace.Name,
		},
	}

	kubeadmConfigSpec := &bootstrapv1.KubeadmConfigSpec{
		Format: "cloud-config",
		Users: []bootstrapv1.User{
			{Name: "test-user"},
		},
	}
	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: namespace.Name,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version:           "v1.25.3",
			KubeadmConfigSpec: *kubeadmConfigSpec,
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: map[string]string{
						"label-1": "value-1",
					},
					Annotations: map[string]string{
						"annotation-1": "value-1",
					},
				},
				InfrastructureRef: corev1.ObjectReference{
					Kind:       "GenericInfrastructureMachineTemplate",
					Namespace:  namespace.Name,
					Name:       "control-plane-infra-machine-template",
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				},
			},
		},
	}

	// Creat the KCP object
	g.Expect(env.Create(ctx, kcp)).To(Succeed())

	expectedReferenceKind := "KubeadmConfig"
	expectedReferenceAPIVersion := bootstrapv1.GroupVersion.String()
	expectedOwner := metav1.OwnerReference{
		Kind:       "KubeadmControlPlane",
		APIVersion: controlplanev1.GroupVersion.String(),
		Name:       kcp.Name,
		UID:        kcp.UID,
	}
	r := &KubeadmControlPlaneReconciler{Client: env}
	// Create the KubeadmConfig
	got, err := r.createKubeadmConfig(ctx, kcp, cluster, kubeadmConfigSpec)
	g.Expect(err).To(BeNil())
	g.Expect(got).NotTo(BeNil())
	g.Expect(got.Name).To(HavePrefix(kcp.Name))
	g.Expect(got.Namespace).To(Equal(kcp.Namespace))
	g.Expect(got.Kind).To(Equal(expectedReferenceKind))
	g.Expect(got.APIVersion).To(Equal(expectedReferenceAPIVersion))

	bootstrapConfig := &bootstrapv1.KubeadmConfig{}
	key := client.ObjectKey{Name: got.Name, Namespace: got.Namespace}
	g.Expect(env.Get(ctx, key, bootstrapConfig)).To(Succeed())
	g.Expect(bootstrapConfig.OwnerReferences).To(HaveLen(1))
	g.Expect(bootstrapConfig.OwnerReferences).To(ContainElement(expectedOwner))
	g.Expect(bootstrapConfig.Spec).To(BeComparableTo(*kubeadmConfigSpec))

	// Verify the labels are propagated from the MachineTemplate
	// Verify that the machineTemplate.ObjectMeta has been propagated to the Machine.
	for k, v := range kcp.Spec.MachineTemplate.ObjectMeta.Labels {
		g.Expect(bootstrapConfig.Labels[k]).To(Equal(v))
	}
	g.Expect(bootstrapConfig.Labels[clusterv1.ClusterNameLabel]).To(Equal(cluster.Name))
	g.Expect(bootstrapConfig.Labels[clusterv1.MachineControlPlaneLabel]).To(Equal(""))
	g.Expect(bootstrapConfig.Labels[clusterv1.MachineControlPlaneNameLabel]).To(Equal(kcp.Name))

	// Verify the annotations are propagated from the MachineTemplate
	g.Expect(bootstrapConfig.Annotations).To(Equal(kcp.Spec.MachineTemplate.ObjectMeta.Annotations))
}

func TestKubeadmControlPlaneReconciler_updateKubeadmConfig(t *testing.T) {
	setup := func(t *testing.T, g *WithT) *corev1.Namespace {
		t.Helper()

		t.Log("Creating the namespace")
		ns, err := env.CreateNamespace(ctx, "test-applykubeadmconfig")
		g.Expect(err).To(BeNil())

		return ns
	}

	teardown := func(t *testing.T, g *WithT, ns *corev1.Namespace) {
		t.Helper()

		t.Log("Deleting the namespace")
		g.Expect(env.Delete(ctx, ns)).To(Succeed())
	}

	g := NewWithT(t)
	namespace := setup(t, g)
	defer teardown(t, g, namespace)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: namespace.Name,
		},
	}

	kubeadmConfigSpec := &bootstrapv1.KubeadmConfigSpec{
		Format: "cloud-config",
		Users: []bootstrapv1.User{
			{Name: "test-user"},
		},
	}
	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: namespace.Name,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Version:           "v1.25.3",
			KubeadmConfigSpec: *kubeadmConfigSpec,
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: map[string]string{
						"label-1": "value-1",
					},
					Annotations: map[string]string{
						"annotation-1": "value-1",
					},
				},
				InfrastructureRef: corev1.ObjectReference{
					Kind:       "GenericInfrastructureMachineTemplate",
					Namespace:  namespace.Name,
					Name:       "control-plane-infra-machine-template",
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				},
			},
		},
	}

	// Creat the KCP object
	g.Expect(env.Create(ctx, kcp)).To(Succeed())

	expectedReferenceKind := "KubeadmConfig"
	expectedReferenceAPIVersion := bootstrapv1.GroupVersion.String()
	expectedOwner := metav1.OwnerReference{
		Kind:       "KubeadmControlPlane",
		APIVersion: controlplanev1.GroupVersion.String(),
		Name:       kcp.Name,
		UID:        kcp.UID,
	}
	r := &KubeadmControlPlaneReconciler{Client: env}
	oldKCPMachineTemplateLabels := kcp.Spec.MachineTemplate.ObjectMeta.Labels
	// Set new labels on KCP MachineTemplate
	kcp.Spec.MachineTemplate.ObjectMeta.Labels = map[string]string{"label-2": "value-2"}

	// Create a KubeadmConfig that will be updated
	bootstrapConfig := &bootstrapv1.KubeadmConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmConfig",
			APIVersion: bootstrapv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeadmconfig",
			Namespace: namespace.Name,
			Labels:    oldKCPMachineTemplateLabels,
		},
		Spec: *kubeadmConfigSpec.DeepCopy(),
	}
	g.Expect(env.Patch(ctx, bootstrapConfig, client.Apply, client.ForceOwnership, client.FieldOwner(kcpManagerName))).To(Succeed())

	// Update the KubeadmConfig
	got, err := r.updateKubeadmConfig(ctx, kcp, cluster, bootstrapConfig)
	g.Expect(err).To(BeNil())
	g.Expect(got).NotTo(BeNil())
	g.Expect(got.UID).To(Equal(bootstrapConfig.UID))   // Verify that UID is the same
	g.Expect(got.Name).To(Equal(bootstrapConfig.Name)) // Verify that Name is the same
	g.Expect(got.Namespace).To(Equal(kcp.Namespace))
	g.Expect(got.Kind).To(Equal(expectedReferenceKind))
	g.Expect(got.APIVersion).To(Equal(expectedReferenceAPIVersion))

	updatedBootstrapConfig := &bootstrapv1.KubeadmConfig{}
	key := client.ObjectKey{Name: got.Name, Namespace: got.Namespace}
	g.Expect(env.Get(ctx, key, updatedBootstrapConfig)).To(Succeed())
	g.Expect(updatedBootstrapConfig.OwnerReferences).To(HaveLen(1))
	g.Expect(updatedBootstrapConfig.OwnerReferences).To(ContainElement(expectedOwner))
	g.Expect(updatedBootstrapConfig.Spec).To(BeComparableTo(*kubeadmConfigSpec))

	// Verify the labels are propagated from the MachineTemplate
	// Verify that the machineTemplate.ObjectMeta has been propagated to the Machine.
	for k, v := range kcp.Spec.MachineTemplate.ObjectMeta.Labels {
		g.Expect(updatedBootstrapConfig.Labels[k]).To(Equal(v))
	}
	g.Expect(updatedBootstrapConfig.Labels[clusterv1.ClusterNameLabel]).To(Equal(cluster.Name))
	g.Expect(updatedBootstrapConfig.Labels[clusterv1.MachineControlPlaneLabel]).To(Equal(""))
	g.Expect(updatedBootstrapConfig.Labels[clusterv1.MachineControlPlaneNameLabel]).To(Equal(kcp.Name))

	// Verify the old labels are not present on the Bootstrap config
	for k := range oldKCPMachineTemplateLabels {
		g.Expect(updatedBootstrapConfig.Labels).NotTo(HaveKey(k))
	}
}

func TestKubeadmControlPlaneReconciler_adoptKubeconfigSecret(t *testing.T) {
	g := NewWithT(t)
	otherOwner := metav1.OwnerReference{
		Name:               "testcontroller",
		UID:                "5",
		Kind:               "OtherController",
		APIVersion:         clusterv1.GroupVersion.String(),
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}
	clusterName := "test1"
	cluster := builder.Cluster(metav1.NamespaceDefault, clusterName).Build()

	// A KubeadmConfig secret created by CAPI controllers with no owner references.
	capiKubeadmConfigSecretNoOwner := kubeconfig.GenerateSecretWithOwner(
		client.ObjectKey{Name: clusterName, Namespace: metav1.NamespaceDefault},
		[]byte{},
		metav1.OwnerReference{})
	capiKubeadmConfigSecretNoOwner.OwnerReferences = []metav1.OwnerReference{}

	// A KubeadmConfig secret created by CAPI controllers with a non-KCP owner reference.
	capiKubeadmConfigSecretOtherOwner := capiKubeadmConfigSecretNoOwner.DeepCopy()
	capiKubeadmConfigSecretOtherOwner.OwnerReferences = []metav1.OwnerReference{otherOwner}

	// A user provided KubeadmConfig secret with no owner reference.
	userProvidedKubeadmConfigSecretNoOwner := kubeconfig.GenerateSecretWithOwner(
		client.ObjectKey{Name: clusterName, Namespace: metav1.NamespaceDefault},
		[]byte{},
		metav1.OwnerReference{})
	userProvidedKubeadmConfigSecretNoOwner.Type = corev1.SecretTypeOpaque

	// A user provided KubeadmConfig with a non-KCP owner reference.
	userProvidedKubeadmConfigSecretOtherOwner := userProvidedKubeadmConfigSecretNoOwner.DeepCopy()
	userProvidedKubeadmConfigSecretOtherOwner.OwnerReferences = []metav1.OwnerReference{otherOwner}

	kcp := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeadmControlPlane",
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testControlPlane",
			Namespace: cluster.Namespace,
		},
	}
	tests := []struct {
		name             string
		configSecret     *corev1.Secret
		expectedOwnerRef metav1.OwnerReference
	}{
		{
			name:         "add KCP owner reference on kubeconfig secret generated by CAPI",
			configSecret: capiKubeadmConfigSecretNoOwner,
			expectedOwnerRef: metav1.OwnerReference{
				Name:               kcp.Name,
				UID:                kcp.UID,
				Kind:               kcp.Kind,
				APIVersion:         kcp.APIVersion,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			},
		},
		{
			name:         "replace owner reference with KCP on kubeconfig secret generated by CAPI with other owner",
			configSecret: capiKubeadmConfigSecretOtherOwner,
			expectedOwnerRef: metav1.OwnerReference{
				Name:               kcp.Name,
				UID:                kcp.UID,
				Kind:               kcp.Kind,
				APIVersion:         kcp.APIVersion,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			},
		},
		{
			name:             "don't add ownerReference on kubeconfig secret provided by user",
			configSecret:     userProvidedKubeadmConfigSecretNoOwner,
			expectedOwnerRef: metav1.OwnerReference{},
		},
		{
			name:             "don't replace ownerReference on kubeconfig secret provided by user",
			configSecret:     userProvidedKubeadmConfigSecretOtherOwner,
			expectedOwnerRef: otherOwner,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := newFakeClient(cluster, kcp, tt.configSecret)
			r := &KubeadmControlPlaneReconciler{
				APIReader: fakeClient,
				Client:    fakeClient,
			}
			g.Expect(r.adoptKubeconfigSecret(ctx, cluster, tt.configSecret, kcp)).To(Succeed())
			actualSecret := &corev1.Secret{}
			g.Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: tt.configSecret.Namespace, Name: tt.configSecret.Namespace}, actualSecret))
			g.Expect(tt.configSecret.GetOwnerReferences()).To(ConsistOf(tt.expectedOwnerRef))
		})
	}
}
