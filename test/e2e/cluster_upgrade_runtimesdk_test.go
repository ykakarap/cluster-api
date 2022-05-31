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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1alpha1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("When upgrading a workload cluster using ClusterClass with RuntimeSDK [PR-Informing]", func() {
	clusterUpgradeWithRuntimeSDKConformanceSpec(ctx, func() ClusterUpgradeWithRuntimeSDKConformanceSpecInput {
		// "upgrades" is the same as the "topology" flavor but with an additional MachinePool.
		flavor := pointer.String("upgrades-runtimesdk")
		// For KubernetesVersionUpgradeFrom < v1.24 we have to use upgrades-cgroupfs flavor.
		// This is because kind and CAPD only support:
		// * cgroupDriver cgroupfs for Kubernetes < v1.24
		// * cgroupDriver systemd for Kubernetes >= v1.24.
		// Notes:
		// * We always use a ClusterClass-based cluster-template for the upgrade test
		// * The ClusterClass will automatically adjust the cgroupDriver for KCP and MDs.
		// * We have to handle the MachinePool ourselves
		// * The upgrades-cgroupfs flavor uses an MP which is pinned to cgroupfs
		// * During the upgrade UpgradeMachinePoolAndWait automatically drops the cgroupfs pinning
		//   when the target version is >= v1.24.
		// We can remove this as soon as we don't test upgrades from Kubernetes < v1.24 anymore with CAPD
		// or MachinePools are supported in ClusterClass.
		version, err := semver.ParseTolerant(e2eConfig.GetVariable(KubernetesVersionUpgradeFrom))
		Expect(err).ToNot(HaveOccurred(), "Invalid argument, KUBERNETES_VERSION_UPGRADE_FROM is not a valid version")
		if version.LT(semver.MustParse("1.24.0")) {
			// "upgrades-cgroupfs" is the same as the "topology" flavor but with an additional MachinePool
			// with pinned cgroupDriver to cgroupfs.
			flavor = pointer.String("upgrades-runtimesdk-cgroupfs")
		}

		return ClusterUpgradeWithRuntimeSDKConformanceSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			Flavor:                flavor,
		}
	})
})

// ClusterUpgradeWithRuntimeSDKConformanceSpecInput is the input for clusterUpgradeWithRuntimeSDKConformanceSpec.
type ClusterUpgradeWithRuntimeSDKConformanceSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string
	SkipCleanup           bool

	// ControlPlaneMachineCount is used in `config cluster` to configure the count of the control plane machines used in the test.
	// Default is 1.
	ControlPlaneMachineCount *int64
	// WorkerMachineCount is used in `config cluster` to configure the count of the worker machines used in the test.
	// NOTE: If the WORKER_MACHINE_COUNT var is used multiple times in the cluster template, the absolute count of
	// worker machines is a multiple of WorkerMachineCount.
	// Default is 2.
	WorkerMachineCount *int64

	// Flavor to use when creating the cluster for testing, "upgrades" is used if not specified.
	Flavor *string
}

// ClusterUpgradeConformanceSpec implements a spec that upgrades a cluster and runs the Kubernetes conformance suite.
// Upgrading a cluster refers to upgrading the control-plane and worker nodes (managed by MD and machine pools).
// NOTE: This test only works with a KubeadmControlPlane.
// NOTE: This test works with Clusters with and without ClusterClass.
// When using ClusterClass the ClusterClass must have the variables "etcdImageTag" and "coreDNSImageTag" of type string.
// Those variables should have corresponding patches which set the etcd and CoreDNS tags in KCP.
func clusterUpgradeWithRuntimeSDKConformanceSpec(ctx context.Context, inputGetter func() ClusterUpgradeWithRuntimeSDKConformanceSpecInput) {
	const (
		textExtensionPathVariable = "TEST_EXTENSION"
		specName                  = "k8s-upgrade-with-runtimesdk"
	)
	var (
		input         ClusterUpgradeWithRuntimeSDKConformanceSpecInput
		namespace     *corev1.Namespace
		cancelWatches context.CancelFunc

		controlPlaneMachineCount int64
		workerMachineCount       int64

		clusterResources  *clusterctl.ApplyClusterTemplateAndWaitResult
		testExtensionPath string
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.E2EConfig).ToNot(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.ClusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. input.ClusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		Expect(input.E2EConfig.Variables).To(HaveKey(KubernetesVersionUpgradeFrom))
		Expect(input.E2EConfig.Variables).To(HaveKey(KubernetesVersionUpgradeTo))
		Expect(input.E2EConfig.Variables).To(HaveKey(EtcdVersionUpgradeTo))
		Expect(input.E2EConfig.Variables).To(HaveKey(CoreDNSVersionUpgradeTo))

		if input.ControlPlaneMachineCount == nil {
			controlPlaneMachineCount = 1
		} else {
			controlPlaneMachineCount = *input.ControlPlaneMachineCount
		}

		if input.WorkerMachineCount == nil {
			workerMachineCount = 2
		} else {
			workerMachineCount = *input.WorkerMachineCount
		}

		testExtensionPath = input.E2EConfig.GetVariable(textExtensionPathVariable)
		Expect(testExtensionPath).To(BeAnExistingFile(), "The %s variable should resolve to an existing file", textExtensionPathVariable)

		// Setup a Namespace where to host objects for this spec and create a watcher for the Namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	})

	It("Should create and upgrade a workload cluster", func() {
		// FIXME(sbueringer): should we create an additional cluster and then deploy the extension there? (like self-hosted, ...)
		By("Deploy Test Extension")
		testExtensionDeployment, err := os.ReadFile(testExtensionPath) //nolint:gosec
		Expect(err).ToNot(HaveOccurred(), "Failed to read the extension config deployment manifest file")
		Expect(testExtensionDeployment).ToNot(BeEmpty(), "Test Extension deployment manifest file should not be empty")
		Expect(input.BootstrapClusterProxy.Apply(ctx, testExtensionDeployment)).To(Succeed())

		By("Deploy Test Extension ExtensionConfig")

		extensionConfig := &runtimev1.ExtensionConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-extension-config",
				Annotations: map[string]string{
					"cert-manager.io/inject-ca-from-secret": "test-extension-system/webhook-service-cert",
				},
			},
			Spec: runtimev1.ExtensionConfigSpec{
				ClientConfig: runtimev1.ClientConfig{
					// FIXME(sbueringer): add a hack to "reconcile" secret into caBundle
					CABundle: nil,
					Service: &runtimev1.ServiceReference{
						Name:      "webhook-service",
						Namespace: "test-extension-system",
					},
					//URL: pointer.String("https://webhook-service.test-extension-system.svc.cluster.local"),
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name:": namespace.Name,
					},
				},
			},
		}

		err = input.BootstrapClusterProxy.GetClient().Create(ctx, extensionConfig)
		Expect(err).ToNot(HaveOccurred(), "Failed to create the extension config")

		By("Creating a workload cluster")

		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: input.BootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     input.ClusterctlConfigPath,
				KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   pointer.StringDeref(input.Flavor, "upgrades"),
				Namespace:                namespace.Name,
				ClusterName:              fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
				KubernetesVersion:        input.E2EConfig.GetVariable(KubernetesVersionUpgradeFrom),
				ControlPlaneMachineCount: pointer.Int64Ptr(controlPlaneMachineCount),
				WorkerMachineCount:       pointer.Int64Ptr(workerMachineCount),
			},
			WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
			WaitForMachinePools:          input.E2EConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		}, clusterResources)

		// Cluster is using ClusterClass, upgrade via topology.
		By("Upgrading the Cluster topology")
		framework.UpgradeClusterTopologyAndWaitForUpgrade(ctx, framework.UpgradeClusterTopologyAndWaitForUpgradeInput{
			ClusterProxy:                input.BootstrapClusterProxy,
			Cluster:                     clusterResources.Cluster,
			ControlPlane:                clusterResources.ControlPlane,
			EtcdImageTag:                input.E2EConfig.GetVariable(EtcdVersionUpgradeTo),
			DNSImageTag:                 input.E2EConfig.GetVariable(CoreDNSVersionUpgradeTo),
			MachineDeployments:          clusterResources.MachineDeployments,
			KubernetesUpgradeVersion:    input.E2EConfig.GetVariable(KubernetesVersionUpgradeTo),
			WaitForMachinesToBeUpgraded: input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade"),
			WaitForKubeProxyUpgrade:     input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade"),
			WaitForDNSUpgrade:           input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade"),
			WaitForEtcdUpgrade:          input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade"),
		})

		// Only attempt to upgrade MachinePools if they were provided in the template.
		if len(clusterResources.MachinePools) > 0 && workerMachineCount > 0 {
			By("Upgrading the machinepool instances")
			framework.UpgradeMachinePoolAndWait(ctx, framework.UpgradeMachinePoolAndWaitInput{
				ClusterProxy:                   input.BootstrapClusterProxy,
				Cluster:                        clusterResources.Cluster,
				UpgradeVersion:                 input.E2EConfig.GetVariable(KubernetesVersionUpgradeTo),
				WaitForMachinePoolToBeUpgraded: input.E2EConfig.GetIntervals(specName, "wait-machine-pool-upgrade"),
				MachinePools:                   clusterResources.MachinePools,
			})
		}

		By("Waiting until nodes are ready")
		workloadProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterResources.Cluster.Name)
		workloadClient := workloadProxy.GetClient()
		framework.WaitForNodesReady(ctx, framework.WaitForNodesReadyInput{
			Lister:            workloadClient,
			KubernetesVersion: input.E2EConfig.GetVariable(KubernetesVersionUpgradeTo),
			Count:             int(clusterResources.ExpectedTotalNodes()),
			WaitForNodesReady: input.E2EConfig.GetIntervals(specName, "wait-nodes-ready"),
		})

		By("PASSED!")
	})

	AfterEach(func() {
		// Dumps all the resources in the spec Namespace, then cleanups the cluster object and the spec Namespace itself.
		dumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder, namespace, cancelWatches, clusterResources.Cluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}
