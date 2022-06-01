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

package patches

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/scope"
	"sigs.k8s.io/cluster-api/internal/test/builder"
	. "sigs.k8s.io/cluster-api/internal/test/matchers"
)

func TestApply(t *testing.T) {
	type expectedFields struct {
		infrastructureCluster                          map[string]interface{}
		controlPlane                                   map[string]interface{}
		controlPlaneInfrastructureMachineTemplate      map[string]interface{}
		machineDeploymentBootstrapTemplate             map[string]map[string]interface{}
		machineDeploymentInfrastructureMachineTemplate map[string]map[string]interface{}
	}

	tests := []struct {
		name           string
		patches        []clusterv1.ClusterClassPatch
		expectedFields expectedFields
	}{
		{
			name: "Should preserve desired state, if there are no patches",
			// No changes expected.
			expectedFields: expectedFields{},
		},
		{
			name: "Should apply JSON patches to InfraCluster, ControlPlane and ControlPlaneInfrastructureMachineTemplate",
			patches: []clusterv1.ClusterClassPatch{
				{
					Name: "fake-patch1",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.InfrastructureGroupVersion.String(),
								Kind:       builder.GenericInfrastructureClusterTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									InfrastructureCluster: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/resource",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"infraCluster"`)},
								},
							},
						},
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.ControlPlaneGroupVersion.String(),
								Kind:       builder.GenericControlPlaneTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									ControlPlane: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/resource",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"controlPlane"`)},
								},
							},
						},
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.InfrastructureGroupVersion.String(),
								Kind:       builder.GenericInfrastructureMachineTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									ControlPlane: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/resource",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"controlPlaneInfrastructureMachineTemplate"`)},
								},
							},
						},
					},
				},
			},
			expectedFields: expectedFields{
				infrastructureCluster: map[string]interface{}{
					"spec.resource": "infraCluster",
				},
				controlPlane: map[string]interface{}{
					"spec.resource": "controlPlane",
				},
				controlPlaneInfrastructureMachineTemplate: map[string]interface{}{
					"spec.template.spec.resource": "controlPlaneInfrastructureMachineTemplate",
				},
			},
		},
		{
			name: "Should apply JSON patches to MachineDeployment templates",
			patches: []clusterv1.ClusterClassPatch{
				{
					Name: "fake-patch1",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.InfrastructureGroupVersion.String(),
								Kind:       builder.GenericInfrastructureMachineTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
										Names: []string{"default-worker"},
									},
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/resource",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"default-worker-infra"`)},
								},
							},
						},
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.BootstrapGroupVersion.String(),
								Kind:       builder.GenericBootstrapConfigTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
										Names: []string{"default-worker"},
									},
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/resource",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"default-worker-bootstrap"`)},
								},
							},
						},
					},
				},
			},
			expectedFields: expectedFields{
				machineDeploymentBootstrapTemplate: map[string]map[string]interface{}{
					"default-worker-topo1": {"spec.template.spec.resource": "default-worker-bootstrap"},
					"default-worker-topo2": {"spec.template.spec.resource": "default-worker-bootstrap"},
				},
				machineDeploymentInfrastructureMachineTemplate: map[string]map[string]interface{}{
					"default-worker-topo1": {"spec.template.spec.resource": "default-worker-infra"},
					"default-worker-topo2": {"spec.template.spec.resource": "default-worker-infra"},
				},
			},
		},
		{
			name: "Should apply JSON patches in the correct order",
			patches: []clusterv1.ClusterClassPatch{
				{
					Name: "fake-patch1",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.ControlPlaneGroupVersion.String(),
								Kind:       builder.GenericControlPlaneTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									ControlPlane: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/clusterName",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"cluster1"`)},
								},
								{
									Op:    "add",
									Path:  "/spec/template/spec/files",
									Value: &apiextensionsv1.JSON{Raw: []byte(`[{"key1":"value1"}]`)},
								},
							},
						},
					},
				},
				{
					Name: "fake-patch2",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.ControlPlaneGroupVersion.String(),
								Kind:       builder.GenericControlPlaneTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									ControlPlane: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "replace",
									Path:  "/spec/template/spec/clusterName",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"cluster1-overwritten"`)},
								},
							},
						},
					},
				},
			},
			expectedFields: expectedFields{
				controlPlane: map[string]interface{}{
					"spec.clusterName": "cluster1-overwritten",
					"spec.files": []interface{}{
						map[string]interface{}{
							"key1": "value1",
						},
					},
				},
			},
		},
		{
			name: "Should apply JSON patches and preserve ControlPlane fields",
			patches: []clusterv1.ClusterClassPatch{
				{
					Name: "fake-patch1",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.ControlPlaneGroupVersion.String(),
								Kind:       builder.GenericControlPlaneTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									ControlPlane: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/replicas",
									Value: &apiextensionsv1.JSON{Raw: []byte(`1`)},
								},
								{
									Op:    "add",
									Path:  "/spec/template/spec/version",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"v1.15.0"`)},
								},
								{
									Op:    "add",
									Path:  "/spec/template/spec/machineTemplate/infrastructureRef",
									Value: &apiextensionsv1.JSON{Raw: []byte(`{"apiVersion":"invalid","kind":"invalid","namespace":"invalid","name":"invalid"}`)},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Should apply JSON patches without metadata",
			patches: []clusterv1.ClusterClassPatch{
				{
					Name: "fake-patch1",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.InfrastructureGroupVersion.String(),
								Kind:       builder.GenericInfrastructureClusterTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									InfrastructureCluster: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/clusterName",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"cluster1"`)},
								},
								{
									Op:    "replace",
									Path:  "/metadata/name",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"overwrittenName"`)},
								},
							},
						},
					},
				},
			},
			expectedFields: expectedFields{
				infrastructureCluster: map[string]interface{}{
					"spec.clusterName": "cluster1",
				},
			},
		},
		{
			name: "Should apply JSON merge patches",
			patches: []clusterv1.ClusterClassPatch{
				{
					Name: "fake-patch1",
					Definitions: []clusterv1.PatchDefinition{
						{
							Selector: clusterv1.PatchSelector{
								APIVersion: builder.InfrastructureGroupVersion.String(),
								Kind:       builder.GenericInfrastructureClusterTemplateKind,
								MatchResources: clusterv1.PatchSelectorMatch{
									InfrastructureCluster: true,
								},
							},
							JSONPatches: []clusterv1.JSONPatch{
								{
									Op:    "add",
									Path:  "/spec/template/spec/resource",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"infraCluster"`)},
								},
							},
						},
					},
				},
			},
			expectedFields: expectedFields{
				infrastructureCluster: map[string]interface{}{
					"spec.resource": "infraCluster",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up test objects, which are:
			// * blueprint:
			//   * A ClusterClass with its corresponding templates:
			//     * ControlPlaneTemplate with a corresponding ControlPlane InfrastructureMachineTemplate.
			//     * MachineDeploymentClass "default-worker" with corresponding BootstrapTemplate and InfrastructureMachineTemplate.
			//   * The corresponding Cluster.spec.topology:
			//     * with 3 ControlPlane replicas
			//     * with a "default-worker-topo1" MachineDeploymentTopology without replicas (based on "default-worker")
			//     * with a "default-worker-topo2" MachineDeploymentTopology with 3 replicas (based on "default-worker")
			// * desired: essentially the corresponding desired objects.
			blueprint, desired := setupTestObjects()

			// If there are patches, set up patch generators.
			patchEngine := NewEngine()
			if len(tt.patches) > 0 {
				// Add the patches.
				blueprint.ClusterClass.Spec.Patches = tt.patches
			}

			// Copy the desired objects before applying patches.
			expectedCluster := desired.Cluster.DeepCopy()
			expectedInfrastructureCluster := desired.InfrastructureCluster.DeepCopy()
			expectedControlPlane := desired.ControlPlane.Object.DeepCopy()
			expectedControlPlaneInfrastructureMachineTemplate := desired.ControlPlane.InfrastructureMachineTemplate.DeepCopy()
			expectedBootstrapTemplates := map[string]*unstructured.Unstructured{}
			expectedInfrastructureMachineTemplate := map[string]*unstructured.Unstructured{}
			for mdTopology, md := range desired.MachineDeployments {
				expectedBootstrapTemplates[mdTopology] = md.BootstrapTemplate.DeepCopy()
				expectedInfrastructureMachineTemplate[mdTopology] = md.InfrastructureMachineTemplate.DeepCopy()
			}

			// Set expected fields on the copy of the objects, so they can be used for comparison with the result of Apply.
			if tt.expectedFields.infrastructureCluster != nil {
				setSpecFields(expectedInfrastructureCluster, tt.expectedFields.infrastructureCluster)
			}
			if tt.expectedFields.controlPlane != nil {
				setSpecFields(expectedControlPlane, tt.expectedFields.controlPlane)
			}
			if tt.expectedFields.controlPlaneInfrastructureMachineTemplate != nil {
				setSpecFields(expectedControlPlaneInfrastructureMachineTemplate, tt.expectedFields.controlPlaneInfrastructureMachineTemplate)
			}
			for mdTopology, expectedFields := range tt.expectedFields.machineDeploymentBootstrapTemplate {
				setSpecFields(expectedBootstrapTemplates[mdTopology], expectedFields)
			}
			for mdTopology, expectedFields := range tt.expectedFields.machineDeploymentInfrastructureMachineTemplate {
				setSpecFields(expectedInfrastructureMachineTemplate[mdTopology], expectedFields)
			}

			// Apply patches.
			g.Expect(patchEngine.Apply(context.Background(), blueprint, desired)).To(Succeed())

			// Compare the patched desired objects with the expected desired objects.
			g.Expect(desired.Cluster).To(EqualObject(expectedCluster))
			g.Expect(desired.InfrastructureCluster).To(EqualObject(expectedInfrastructureCluster))
			g.Expect(desired.ControlPlane.Object).To(EqualObject(expectedControlPlane))
			g.Expect(desired.ControlPlane.InfrastructureMachineTemplate).To(EqualObject(expectedControlPlaneInfrastructureMachineTemplate))
			for mdTopology, bootstrapTemplate := range expectedBootstrapTemplates {
				g.Expect(desired.MachineDeployments[mdTopology].BootstrapTemplate).To(EqualObject(bootstrapTemplate))
			}
			for mdTopology, infrastructureMachineTemplate := range expectedInfrastructureMachineTemplate {
				g.Expect(desired.MachineDeployments[mdTopology].InfrastructureMachineTemplate).To(EqualObject(infrastructureMachineTemplate))
			}
		})
	}
}

func setupTestObjects() (*scope.ClusterBlueprint, *scope.ClusterState) {
	infrastructureClusterTemplate := builder.InfrastructureClusterTemplate(metav1.NamespaceDefault, "infraClusterTemplate1").
		Build()

	controlPlaneInfrastructureMachineTemplate := builder.InfrastructureMachineTemplate(metav1.NamespaceDefault, "controlplaneinframachinetemplate1").
		Build()
	controlPlaneTemplate := builder.ControlPlaneTemplate(metav1.NamespaceDefault, "controlPlaneTemplate1").
		WithInfrastructureMachineTemplate(controlPlaneInfrastructureMachineTemplate).
		Build()

	workerInfrastructureMachineTemplate := builder.InfrastructureMachineTemplate(metav1.NamespaceDefault, "linux-worker-inframachinetemplate").
		Build()
	workerBootstrapTemplate := builder.BootstrapTemplate(metav1.NamespaceDefault, "linux-worker-bootstraptemplate").
		Build()
	mdClass1 := builder.MachineDeploymentClass("default-worker").
		WithInfrastructureTemplate(workerInfrastructureMachineTemplate).
		WithBootstrapTemplate(workerBootstrapTemplate).
		Build()

	clusterClass := builder.ClusterClass(metav1.NamespaceDefault, "clusterClass1").
		WithInfrastructureClusterTemplate(infrastructureClusterTemplate).
		WithControlPlaneTemplate(controlPlaneTemplate).
		WithControlPlaneInfrastructureMachineTemplate(controlPlaneInfrastructureMachineTemplate).
		WithWorkerMachineDeploymentClasses(*mdClass1).
		Build()

	// Note: we depend on TypeMeta being set to calculate HolderReferences correctly.
	// We also set TypeMeta explicitly in the topology/cluster/cluster_controller.go.
	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: false,
			ClusterNetwork: &clusterv1.ClusterNetwork{
				APIServerPort: pointer.Int32(8),
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"10.10.10.1/24"},
				},
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"11.10.10.1/24"},
				},
				ServiceDomain: "lark",
			},
			ControlPlaneRef:   nil,
			InfrastructureRef: nil,
			Topology: &clusterv1.Topology{
				Version: "v1.21.2",
				Class:   clusterClass.Name,
				ControlPlane: clusterv1.ControlPlaneTopology{
					Replicas: pointer.Int32(3),
				},
				Workers: &clusterv1.WorkersTopology{
					MachineDeployments: []clusterv1.MachineDeploymentTopology{
						{
							Metadata: clusterv1.ObjectMeta{},
							Class:    "default-worker",
							Name:     "default-worker-topo1",
						},
						{
							Metadata: clusterv1.ObjectMeta{},
							Class:    "default-worker",
							Name:     "default-worker-topo2",
							Replicas: pointer.Int32(5),
						},
					},
				},
			},
		},
	}

	// Aggregating Cluster, Templates and ClusterClass into a blueprint.
	blueprint := &scope.ClusterBlueprint{
		Topology:                      cluster.Spec.Topology,
		ClusterClass:                  clusterClass,
		InfrastructureClusterTemplate: infrastructureClusterTemplate,
		ControlPlane: &scope.ControlPlaneBlueprint{
			Template:                      controlPlaneTemplate,
			InfrastructureMachineTemplate: controlPlaneInfrastructureMachineTemplate,
		},
		MachineDeployments: map[string]*scope.MachineDeploymentBlueprint{
			"default-worker": {
				InfrastructureMachineTemplate: workerInfrastructureMachineTemplate,
				BootstrapTemplate:             workerBootstrapTemplate,
			},
		},
	}

	// Create a Cluster using the ClusterClass from above with multiple MachineDeployments
	// using the same MachineDeployment class.
	desiredCluster := cluster.DeepCopy()

	infrastructureCluster := builder.InfrastructureCluster(metav1.NamespaceDefault, "infraClusterTemplate1").
		WithSpecFields(map[string]interface{}{
			// Add an empty spec field, to make sure the InfrastructureCluster matches
			// the one calculated by computeInfrastructureCluster.
			"spec": map[string]interface{}{},
		}).
		Build()

	controlPlane := builder.ControlPlane(metav1.NamespaceDefault, "controlPlane1").
		WithVersion("v1.21.2").
		WithReplicas(3).
		// Make sure we're using an independent instance of the template.
		WithInfrastructureMachineTemplate(controlPlaneInfrastructureMachineTemplate.DeepCopy()).
		Build()

	desired := &scope.ClusterState{
		Cluster:               desiredCluster,
		InfrastructureCluster: infrastructureCluster,
		ControlPlane: &scope.ControlPlaneState{
			Object: controlPlane,
			// Make sure we're using an independent instance of the template.
			InfrastructureMachineTemplate: controlPlaneInfrastructureMachineTemplate.DeepCopy(),
		},
		MachineDeployments: map[string]*scope.MachineDeploymentState{
			"default-worker-topo1": {
				Object: builder.MachineDeployment(metav1.NamespaceDefault, "md1").
					WithVersion("v1.21.2").
					Build(),
				// Make sure we're using an independent instance of the template.
				InfrastructureMachineTemplate: workerInfrastructureMachineTemplate.DeepCopy(),
				BootstrapTemplate:             workerBootstrapTemplate.DeepCopy(),
			},
			"default-worker-topo2": {
				Object: builder.MachineDeployment(metav1.NamespaceDefault, "md2").
					WithVersion("v1.20.6").
					WithReplicas(5).
					Build(),
				// Make sure we're using an independent instance of the template.
				InfrastructureMachineTemplate: workerInfrastructureMachineTemplate.DeepCopy(),
				BootstrapTemplate:             workerBootstrapTemplate.DeepCopy(),
			},
		},
	}
	return blueprint, desired
}

// setSpecFields sets fields on an unstructured object from a map.
func setSpecFields(obj *unstructured.Unstructured, fields map[string]interface{}) {
	for k, v := range fields {
		fieldParts := strings.Split(k, ".")
		if len(fieldParts) == 0 {
			panic(fmt.Errorf("fieldParts invalid"))
		}
		if fieldParts[0] != "spec" {
			panic(fmt.Errorf("can not set fields outside spec"))
		}
		if err := unstructured.SetNestedField(obj.UnstructuredContent(), v, strings.Split(k, ".")...); err != nil {
			panic(err)
		}
	}
}
