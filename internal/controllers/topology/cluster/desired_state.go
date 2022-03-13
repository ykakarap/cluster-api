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

package cluster

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/internal/contract"
	"sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/scope"
	tlog "sigs.k8s.io/cluster-api/internal/log"
)

// computeDesiredState computes the desired state of the cluster topology.
// NOTE: We are assuming all the required objects are provided as input; also, in case of any error,
// the entire compute operation will fail. This might be improved in the future if support for reconciling
// subset of a topology will be implemented.
func (r *Reconciler) computeDesiredState(ctx context.Context, s *scope.Scope) (*scope.ClusterState, error) {
	var err error
	desiredState := &scope.ClusterState{
		ControlPlane: &scope.ControlPlaneState{},
	}

	// Compute the desired state of the InfrastructureCluster object.
	if desiredState.InfrastructureCluster, err = computeInfrastructureCluster(ctx, s); err != nil {
		return nil, errors.Wrapf(err, "failed to compute InfrastructureCluster")
	}

	// If the clusterClass mandates the controlPlane has infrastructureMachines, compute the InfrastructureMachineTemplate for the ControlPlane.
	if s.Blueprint.HasControlPlaneInfrastructureMachine() {
		if desiredState.ControlPlane.InfrastructureMachineTemplate, err = computeControlPlaneInfrastructureMachineTemplate(ctx, s); err != nil {
			return nil, errors.Wrapf(err, "failed to compute ControlPlane InfrastructureMachineTemplate")
		}
	}

	// Compute the desired state of the ControlPlane object, eventually adding a reference to the
	// InfrastructureMachineTemplate generated by the previous step.
	if desiredState.ControlPlane.Object, err = computeControlPlane(ctx, s, desiredState.ControlPlane.InfrastructureMachineTemplate); err != nil {
		return nil, errors.Wrapf(err, "failed to compute ControlPlane")
	}

	// Compute the desired state of the ControlPlane MachineHealthCheck if defined.
	// The MachineHealthCheck will have the same name as the ControlPlane Object and a selector for the ControlPlane InfrastructureMachines.
	if s.Blueprint.HasControlPlaneMachineHealthCheck() {
		desiredState.ControlPlane.MachineHealthCheck = computeMachineHealthCheck(
			desiredState.ControlPlane.Object,
			selectorForControlPlaneMHC(),
			s.Current.Cluster.Name,
			s.Blueprint.ControlPlane.MachineHealthCheck)
	}

	// Compute the desired state for the Cluster object adding a reference to the
	// InfrastructureCluster and the ControlPlane objects generated by the previous step.
	desiredState.Cluster = computeCluster(ctx, s, desiredState.InfrastructureCluster, desiredState.ControlPlane.Object)

	// If required, compute the desired state of the MachineDeployments from the list of MachineDeploymentTopologies
	// defined in the cluster.
	if s.Blueprint.HasMachineDeployments() {
		desiredState.MachineDeployments, err = computeMachineDeployments(ctx, s, desiredState.ControlPlane)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compute MachineDeployments")
		}
	}

	// Apply patches the desired state according to the patches from the ClusterClass, variables from the Cluster
	// and builtin variables.
	// NOTE: We have to make sure all spec fields that were explicitly set in desired objects during the computation above
	// are preserved during patching. When desired objects are computed their spec is copied from a template, in some cases
	// further modifications to the spec are made afterwards. In those cases we have to make sure those fields are not overwritten
	// in apply patches. Some examples are .spec.machineTemplate and .spec.version in control planes.
	if err := r.patchEngine.Apply(ctx, s.Blueprint, desiredState); err != nil {
		return nil, errors.Wrap(err, "failed to apply patches")
	}

	return desiredState, nil
}

// computeInfrastructureCluster computes the desired state for the InfrastructureCluster object starting from the
// corresponding template defined in the blueprint.
func computeInfrastructureCluster(_ context.Context, s *scope.Scope) (*unstructured.Unstructured, error) {
	template := s.Blueprint.InfrastructureClusterTemplate
	templateClonedFromRef := s.Blueprint.ClusterClass.Spec.Infrastructure.Ref
	cluster := s.Current.Cluster
	currentRef := cluster.Spec.InfrastructureRef

	infrastructureCluster, err := templateToObject(templateToInput{
		template:              template,
		templateClonedFromRef: templateClonedFromRef,
		cluster:               cluster,
		namePrefix:            fmt.Sprintf("%s-", cluster.Name),
		currentObjectRef:      currentRef,
		// Note: It is not possible to add an ownerRef to Cluster at this stage, otherwise the provisioning
		// of the infrastructure cluster starts no matter of the object being actually referenced by the Cluster itself.
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate the InfrastructureCluster object from the %s", template.GetKind())
	}
	return infrastructureCluster, nil
}

// computeControlPlaneInfrastructureMachineTemplate computes the desired state for InfrastructureMachineTemplate
// that should be referenced by the ControlPlane object.
func computeControlPlaneInfrastructureMachineTemplate(_ context.Context, s *scope.Scope) (*unstructured.Unstructured, error) {
	template := s.Blueprint.ControlPlane.InfrastructureMachineTemplate
	templateClonedFromRef := s.Blueprint.ClusterClass.Spec.ControlPlane.MachineInfrastructure.Ref
	cluster := s.Current.Cluster

	// Check if the current control plane object has a machineTemplate.infrastructureRef already defined.
	// TODO: Move the next few lines into a method on scope.ControlPlaneState
	var currentRef *corev1.ObjectReference
	if s.Current.ControlPlane != nil && s.Current.ControlPlane.Object != nil {
		var err error
		if currentRef, err = contract.ControlPlane().MachineTemplate().InfrastructureRef().Get(s.Current.ControlPlane.Object); err != nil {
			return nil, errors.Wrap(err, "failed to get spec.machineTemplate.infrastructureRef for the current ControlPlane object")
		}
	}

	controlPlaneInfrastructureMachineTemplate := templateToTemplate(templateToInput{
		template:              template,
		templateClonedFromRef: templateClonedFromRef,
		cluster:               cluster,
		namePrefix:            controlPlaneInfrastructureMachineTemplateNamePrefix(cluster.Name),
		currentObjectRef:      currentRef,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and updating the Cluster object
		// with the reference to the ControlPlane object using this template.
		ownerRef: ownerReferenceTo(s.Current.Cluster),
	})
	return controlPlaneInfrastructureMachineTemplate, nil
}

// computeControlPlane computes the desired state for the ControlPlane object starting from the
// corresponding template defined in the blueprint.
func computeControlPlane(_ context.Context, s *scope.Scope, infrastructureMachineTemplate *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	template := s.Blueprint.ControlPlane.Template
	templateClonedFromRef := s.Blueprint.ClusterClass.Spec.ControlPlane.Ref
	cluster := s.Current.Cluster
	currentRef := cluster.Spec.ControlPlaneRef

	controlPlane, err := templateToObject(templateToInput{
		template:              template,
		templateClonedFromRef: templateClonedFromRef,
		cluster:               cluster,
		namePrefix:            fmt.Sprintf("%s-", cluster.Name),
		currentObjectRef:      currentRef,
		// Note: It is not possible to add an ownerRef to Cluster at this stage, otherwise the provisioning
		// of the ControlPlane starts no matter of the object being actually referenced by the Cluster itself.
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate the ControlPlane object from the %s", template.GetKind())
	}

	// If the ClusterClass mandates the controlPlane has infrastructureMachines, add a reference to InfrastructureMachine
	// template and metadata to be used for the control plane machines.
	if s.Blueprint.HasControlPlaneInfrastructureMachine() {
		if err := contract.ControlPlane().MachineTemplate().InfrastructureRef().Set(controlPlane, infrastructureMachineTemplate); err != nil {
			return nil, errors.Wrap(err, "failed to spec.machineTemplate.infrastructureRef in the ControlPlane object")
		}

		// Compute the labels and annotations to be applied to ControlPlane machines.
		// We merge the labels and annotations from topology and ClusterClass.
		// We also add the cluster-name and the topology owned labels, so they are propagated down to Machines.
		topologyMetadata := s.Blueprint.Topology.ControlPlane.Metadata
		clusterClassMetadata := s.Blueprint.ClusterClass.Spec.ControlPlane.Metadata

		machineLabels := mergeMap(topologyMetadata.Labels, clusterClassMetadata.Labels)
		if machineLabels == nil {
			machineLabels = map[string]string{}
		}
		machineLabels[clusterv1.ClusterLabelName] = cluster.Name
		machineLabels[clusterv1.ClusterTopologyOwnedLabel] = ""
		if err := contract.ControlPlane().MachineTemplate().Metadata().Set(controlPlane,
			&clusterv1.ObjectMeta{
				Labels:      machineLabels,
				Annotations: mergeMap(topologyMetadata.Annotations, clusterClassMetadata.Annotations),
			}); err != nil {
			return nil, errors.Wrap(err, "failed to set spec.machineTemplate.metadata in the ControlPlane object")
		}
	}

	// If it is required to manage the number of replicas for the control plane, set the corresponding field.
	// NOTE: If the Topology.ControlPlane.replicas value is nil, it is assumed that the control plane controller
	// does not implement support for this field and the ControlPlane object is generated without the number of Replicas.
	if s.Blueprint.Topology.ControlPlane.Replicas != nil {
		if err := contract.ControlPlane().Replicas().Set(controlPlane, int64(*s.Blueprint.Topology.ControlPlane.Replicas)); err != nil {
			return nil, errors.Wrap(err, "failed to set spec.replicas in the ControlPlane object")
		}
	}

	// Sets the desired Kubernetes version for the control plane.
	version, err := computeControlPlaneVersion(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute version of control plane")
	}
	if err := contract.ControlPlane().Version().Set(controlPlane, version); err != nil {
		return nil, errors.Wrap(err, "failed to set spec.version in the ControlPlane object")
	}

	return controlPlane, nil
}

// computeControlPlaneVersion calculates the version of the desired control plane.
// The version is calculated using the state of the current machine deployments, the current control plane
// and the version defined in the topology.
func computeControlPlaneVersion(s *scope.Scope) (string, error) {
	desiredVersion := s.Blueprint.Topology.Version
	// If we are creating the control plane object (current control plane is nil), use version from topology.
	if s.Current.ControlPlane == nil || s.Current.ControlPlane.Object == nil {
		return desiredVersion, nil
	}

	// Get the current currentVersion of the control plane.
	currentVersion, err := contract.ControlPlane().Version().Get(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the version from control plane spec")
	}

	s.UpgradeTracker.ControlPlane.PendingUpgrade = true
	if *currentVersion == desiredVersion {
		// Mark that the control plane spec is already at the desired version.
		// This information is used to show the appropriate message for the TopologyReconciled
		// condition.
		s.UpgradeTracker.ControlPlane.PendingUpgrade = false
	}

	// Check if the control plane is being created for the first time.
	cpProvisioning, err := contract.ControlPlane().IsProvisioning(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to check if the control plane is being provisioned")
	}
	// If the control plane is being provisioned (being craeted for the first time), then do not
	// pick up the desiredVersion yet.
	// Return the current version of the control plane. We will pick up the new version after the
	// control plane is provisioned.
	if cpProvisioning {
		s.UpgradeTracker.ControlPlane.IsProvisioning = true
		return *currentVersion, nil
	}

	// Check if the current control plane is upgrading
	cpUpgrading, err := contract.ControlPlane().IsUpgrading(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to check if control plane is upgrading")
	}
	// If the current control plane is upgrading  (still completing a previous upgrade),
	// then do not pick up the desiredVersion yet.
	// Return the current version of the control plane. We will pick up the new version
	// after the control plane is stable.
	if cpUpgrading {
		s.UpgradeTracker.ControlPlane.IsUpgrading = true
		return *currentVersion, nil
	}

	// Return here if the control plane is already at the desired version
	if !s.UpgradeTracker.ControlPlane.PendingUpgrade {
		// At this stage the control plane is not upgrading and is already at the desired version.
		// We can return.
		// Nb. We do not return early in the function if the control plane is already at the desired version so as
		// to know if the control plane is being upgraded. This information
		// is required when updating the TopologyReconciled condition on the cluster.
		return *currentVersion, nil
	}

	// If the control plane supports replicas, check if the control plane is in the middle of a scale operation.
	// If yes, then do not pick up the desiredVersion yet. We will pick up the new version after the control plane is stable.
	if s.Blueprint.Topology.ControlPlane.Replicas != nil {
		cpScaling, err := contract.ControlPlane().IsScaling(s.Current.ControlPlane.Object)
		if err != nil {
			return "", errors.Wrap(err, "failed to check if the control plane is scaling")
		}
		if cpScaling {
			s.UpgradeTracker.ControlPlane.IsScaling = true
			return *currentVersion, nil
		}
	}

	// If the control plane is not upgrading or scaling, we can assume the control plane is stable.
	// However, we should also check for the MachineDeployments to be stable.
	// If the MachineDeployments are rolling out (still completing a previous upgrade), then do not pick
	// up the desiredVersion yet. We will pick up the new version after the MachineDeployments are stable.
	if s.Current.MachineDeployments.IsAnyRollingOut() {
		return *currentVersion, nil
	}

	// Control plane and machine deployments are stable.
	// Ready to pick up the topology version.

	// Call the BeforeClusterUpgradeHook extensions.
	res, err := registry.Call(BeforeClusterUpgradeHook{})
	if err != nil {
		return *currentVersion, errors.Wrap(err, "BeforeClusterUpgradeHook extensions failed")
	}
	if res.RetryAfterSeconds != 0 {
		// don't pickup the new version yet.
		// TODO: how to recheck some time without blocking the entire reconcile operation?
		return *currentVersion, nil
	}

	return desiredVersion, nil
}

// computeCluster computes the desired state for the Cluster object.
// NOTE: Some fields of the Cluster’s fields contribute to defining the Cluster blueprint (e.g. Cluster.Spec.Topology),
// while some other fields should be managed as part of the actual Cluster (e.g. Cluster.Spec.ControlPlaneRef); in this func
// we are concerned only about the latest group of fields.
func computeCluster(_ context.Context, s *scope.Scope, infrastructureCluster, controlPlane *unstructured.Unstructured) *clusterv1.Cluster {
	cluster := s.Current.Cluster.DeepCopy()

	// Enforce the topology labels.
	// NOTE: The cluster label is added at creation time so this object could be read by the ClusterTopology
	// controller immediately after creation, even before other controllers are going to add the label (if missing).
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	cluster.Labels[clusterv1.ClusterLabelName] = cluster.Name
	cluster.Labels[clusterv1.ClusterTopologyOwnedLabel] = ""

	// Set the references to the infrastructureCluster and controlPlane objects.
	// NOTE: Once set for the first time, the references are not expected to change.
	cluster.Spec.InfrastructureRef = contract.ObjToRef(infrastructureCluster)
	cluster.Spec.ControlPlaneRef = contract.ObjToRef(controlPlane)

	return cluster
}

// computeMachineDeployments computes the desired state of the list of MachineDeployments.
func computeMachineDeployments(ctx context.Context, s *scope.Scope, desiredControlPlaneState *scope.ControlPlaneState) (scope.MachineDeploymentsStateMap, error) {
	// Mark all the machine deployments that are currently rolling out.
	// This captured information will be used for
	//   - Building the TopologyReconciled condition.
	//   - Making upgrade decisions on machine deployments.
	s.UpgradeTracker.MachineDeployments.MarkRollingOut(s.Current.MachineDeployments.RollingOut()...)
	machineDeploymentsStateMap := make(scope.MachineDeploymentsStateMap)
	for _, mdTopology := range s.Blueprint.Topology.Workers.MachineDeployments {
		desiredMachineDeployment, err := computeMachineDeployment(ctx, s, desiredControlPlaneState, mdTopology)
		if err != nil {
			return nil, err
		}
		machineDeploymentsStateMap[mdTopology.Name] = desiredMachineDeployment
	}
	return machineDeploymentsStateMap, nil
}

// computeMachineDeployment computes the desired state for a MachineDeploymentTopology.
// The generated machineDeployment object is calculated using the values from the machineDeploymentTopology and
// the machineDeployment class.
func computeMachineDeployment(_ context.Context, s *scope.Scope, desiredControlPlaneState *scope.ControlPlaneState, machineDeploymentTopology clusterv1.MachineDeploymentTopology) (*scope.MachineDeploymentState, error) {
	desiredMachineDeployment := &scope.MachineDeploymentState{}

	// Gets the blueprint for the MachineDeployment class.
	className := machineDeploymentTopology.Class
	machineDeploymentBlueprint, ok := s.Blueprint.MachineDeployments[className]
	if !ok {
		return nil, errors.Errorf("MachineDeployment class %s not found in %s", className, tlog.KObj{Obj: s.Blueprint.ClusterClass})
	}

	// Compute the boostrap template.
	currentMachineDeployment := s.Current.MachineDeployments[machineDeploymentTopology.Name]
	var currentBootstrapTemplateRef *corev1.ObjectReference
	if currentMachineDeployment != nil && currentMachineDeployment.BootstrapTemplate != nil {
		currentBootstrapTemplateRef = currentMachineDeployment.Object.Spec.Template.Spec.Bootstrap.ConfigRef
	}
	desiredMachineDeployment.BootstrapTemplate = templateToTemplate(templateToInput{
		template:              machineDeploymentBlueprint.BootstrapTemplate,
		templateClonedFromRef: contract.ObjToRef(machineDeploymentBlueprint.BootstrapTemplate),
		cluster:               s.Current.Cluster,
		namePrefix:            bootstrapTemplateNamePrefix(s.Current.Cluster.Name, machineDeploymentTopology.Name),
		currentObjectRef:      currentBootstrapTemplateRef,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and creating/updating the MachineDeployment object
		// with the reference to the ControlPlane object using this template.
		ownerRef: ownerReferenceTo(s.Current.Cluster),
	})

	bootstrapTemplateLabels := desiredMachineDeployment.BootstrapTemplate.GetLabels()
	if bootstrapTemplateLabels == nil {
		bootstrapTemplateLabels = map[string]string{}
	}
	// Add ClusterTopologyMachineDeploymentLabel to the generated Bootstrap template
	bootstrapTemplateLabels[clusterv1.ClusterTopologyMachineDeploymentLabelName] = machineDeploymentTopology.Name
	desiredMachineDeployment.BootstrapTemplate.SetLabels(bootstrapTemplateLabels)

	// Compute the Infrastructure template.
	var currentInfraMachineTemplateRef *corev1.ObjectReference
	if currentMachineDeployment != nil && currentMachineDeployment.InfrastructureMachineTemplate != nil {
		currentInfraMachineTemplateRef = &currentMachineDeployment.Object.Spec.Template.Spec.InfrastructureRef
	}
	desiredMachineDeployment.InfrastructureMachineTemplate = templateToTemplate(templateToInput{
		template:              machineDeploymentBlueprint.InfrastructureMachineTemplate,
		templateClonedFromRef: contract.ObjToRef(machineDeploymentBlueprint.InfrastructureMachineTemplate),
		cluster:               s.Current.Cluster,
		namePrefix:            infrastructureMachineTemplateNamePrefix(s.Current.Cluster.Name, machineDeploymentTopology.Name),
		currentObjectRef:      currentInfraMachineTemplateRef,
		// Note: we are adding an ownerRef to Cluster so the template will be automatically garbage collected
		// in case of errors in between creating this template and creating/updating the MachineDeployment object
		// with the reference to the ControlPlane object using this template.
		ownerRef: ownerReferenceTo(s.Current.Cluster),
	})

	infraMachineTemplateLabels := desiredMachineDeployment.InfrastructureMachineTemplate.GetLabels()
	if infraMachineTemplateLabels == nil {
		infraMachineTemplateLabels = map[string]string{}
	}
	// Add ClusterTopologyMachineDeploymentLabel to the generated InfrastructureMachine template
	infraMachineTemplateLabels[clusterv1.ClusterTopologyMachineDeploymentLabelName] = machineDeploymentTopology.Name
	desiredMachineDeployment.InfrastructureMachineTemplate.SetLabels(infraMachineTemplateLabels)
	version, err := computeMachineDeploymentVersion(s, desiredControlPlaneState, currentMachineDeployment)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compute version for %s", machineDeploymentTopology.Name)
	}

	// Compute the MachineDeployment object.
	gv := clusterv1.GroupVersion
	desiredMachineDeploymentObj := &clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       gv.WithKind("MachineDeployment").Kind,
			APIVersion: gv.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-%s-", s.Current.Cluster.Name, machineDeploymentTopology.Name)),
			Namespace: s.Current.Cluster.Namespace,
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: s.Current.Cluster.Name,
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels:      mergeMap(machineDeploymentTopology.Metadata.Labels, machineDeploymentBlueprint.Metadata.Labels),
					Annotations: mergeMap(machineDeploymentTopology.Metadata.Annotations, machineDeploymentBlueprint.Metadata.Annotations),
				},
				Spec: clusterv1.MachineSpec{
					ClusterName:       s.Current.Cluster.Name,
					Version:           pointer.String(version),
					Bootstrap:         clusterv1.Bootstrap{ConfigRef: contract.ObjToRef(desiredMachineDeployment.BootstrapTemplate)},
					InfrastructureRef: *contract.ObjToRef(desiredMachineDeployment.InfrastructureMachineTemplate),
					FailureDomain:     machineDeploymentTopology.FailureDomain,
				},
			},
		},
	}

	// If it's a new MachineDeployment, set the finalizer.
	// Note: we only add it on creation to avoid race conditions later on when
	// the MachineDeployment topology controller removes the finalizer.
	if currentMachineDeployment == nil {
		controllerutil.AddFinalizer(desiredMachineDeploymentObj, clusterv1.MachineDeploymentTopologyFinalizer)
	}

	// If an existing MachineDeployment is present, override the MachineDeployment generate name
	// re-using the existing name (this will help in reconcile).
	if currentMachineDeployment != nil && currentMachineDeployment.Object != nil {
		desiredMachineDeploymentObj.SetName(currentMachineDeployment.Object.Name)
	}

	// Apply Labels
	// NOTE: On top of all the labels applied to managed objects we are applying the ClusterTopologyMachineDeploymentLabel
	// keeping track of the MachineDeployment name from the Topology; this will be used to identify the object in next reconcile loops.
	labels := map[string]string{}
	labels[clusterv1.ClusterLabelName] = s.Current.Cluster.Name
	labels[clusterv1.ClusterTopologyOwnedLabel] = ""
	labels[clusterv1.ClusterTopologyMachineDeploymentLabelName] = machineDeploymentTopology.Name
	desiredMachineDeploymentObj.SetLabels(labels)

	// Set the selector with the subset of labels identifying controlled machines.
	// NOTE: this prevents the web hook to add cluster.x-k8s.io/deployment-name label, that is
	// redundant for managed MachineDeployments given that we already have topology.cluster.x-k8s.io/deployment-name.
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels = map[string]string{}
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels[clusterv1.ClusterLabelName] = s.Current.Cluster.Name
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels[clusterv1.ClusterTopologyOwnedLabel] = ""
	desiredMachineDeploymentObj.Spec.Selector.MatchLabels[clusterv1.ClusterTopologyMachineDeploymentLabelName] = machineDeploymentTopology.Name

	// Also set the labels in .spec.template.labels so that they are propagated to
	// MachineSet.labels and MachineSet.spec.template.labels and thus to Machine.labels.
	// Note: the labels in MachineSet are used to properly cleanup templates when the MachineSet is deleted.
	if desiredMachineDeploymentObj.Spec.Template.Labels == nil {
		desiredMachineDeploymentObj.Spec.Template.Labels = map[string]string{}
	}
	desiredMachineDeploymentObj.Spec.Template.Labels[clusterv1.ClusterLabelName] = s.Current.Cluster.Name
	desiredMachineDeploymentObj.Spec.Template.Labels[clusterv1.ClusterTopologyOwnedLabel] = ""
	desiredMachineDeploymentObj.Spec.Template.Labels[clusterv1.ClusterTopologyMachineDeploymentLabelName] = machineDeploymentTopology.Name

	// Set the desired replicas.
	desiredMachineDeploymentObj.Spec.Replicas = machineDeploymentTopology.Replicas

	desiredMachineDeployment.Object = desiredMachineDeploymentObj

	// If the ClusterClass defines a MachineHealthCheck for the MachineDeployment add it to the desired state.
	if machineDeploymentBlueprint.MachineHealthCheck != nil {
		// Note: The MHC is going to use a selector that provides a minimal set of labels which are common to all MachineSets belonging to the MachineDeployment.
		desiredMachineDeployment.MachineHealthCheck = computeMachineHealthCheck(
			desiredMachineDeploymentObj,
			selectorForMachineDeploymentMHC(desiredMachineDeploymentObj),
			s.Current.Cluster.Name,
			machineDeploymentBlueprint.MachineHealthCheck)
	}
	return desiredMachineDeployment, nil
}

// computeMachineDeploymentVersion calculates the version of the desired machine deployment.
// The version is calculated using the state of the current machine deployments,
// the current control plane and the version defined in the topology.
// Nb: No MachineDeployment upgrades will be triggered while any MachineDeployment is in the middle
// of an upgrade. Even if the number of MachineDeployments that are being upgraded is less
// than the number of allowed concurrent upgrades.
func computeMachineDeploymentVersion(s *scope.Scope, desiredControlPlaneState *scope.ControlPlaneState, currentMDState *scope.MachineDeploymentState) (string, error) {
	desiredVersion := s.Blueprint.Topology.Version
	// If creating a new machine deployment, we can pick up the desired version
	// Note: We are not blocking the creation of new machine deployments when
	// the control plane or any of the machine deployments are upgrading/scaling.
	if currentMDState == nil || currentMDState.Object == nil {
		return desiredVersion, nil
	}

	// Get the current version of the machine deployment.
	currentVersion := *currentMDState.Object.Spec.Template.Spec.Version

	// Return early if the currentVersion is already equal to the desiredVersion
	// no further checks required.
	if currentVersion == desiredVersion {
		return currentVersion, nil
	}

	// Return early if we are not allowed to upgrade the machine deployment.
	if !s.UpgradeTracker.MachineDeployments.AllowUpgrade() {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion, nil
	}

	// If the control plane is being created (current control plane is nil), do not perform
	// any machine deployment upgrade in this case.
	// Return the current version of the machine deployment.
	// NOTE: this case should never happen (upgrading a MachineDeployment) before creating a CP,
	// but we are implementing this check for extra safety.
	if s.Current.ControlPlane == nil || s.Current.ControlPlane.Object == nil {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion, nil
	}

	// If the current control plane is upgrading, then do not pick up the desiredVersion yet.
	// Return the current version of the machine deployment. We will pick up the new version after the control
	// plane is stable.
	cpUpgrading, err := contract.ControlPlane().IsUpgrading(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to check if control plane is upgrading")
	}
	if cpUpgrading {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion, nil
	}

	// If control plane supports replicas, check if the control plane is in the middle of a scale operation.
	// If the current control plane is scaling, then do not pick up the desiredVersion yet.
	// Return the current version of the machine deployment. We will pick up the new version after the control
	// plane is stable.
	if s.Blueprint.Topology.ControlPlane.Replicas != nil {
		cpScaling, err := contract.ControlPlane().IsScaling(s.Current.ControlPlane.Object)
		if err != nil {
			return "", errors.Wrap(err, "failed to check if the control plane is scaling")
		}
		if cpScaling {
			s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
			return currentVersion, nil
		}
	}

	// Check if we are about to upgrade the control plane. In that case, do not upgrade the machine deployment yet.
	// Wait for the new upgrade operation on the control plane to finish before picking up the new version for the
	// machine deployment.
	currentCPVersion, err := contract.ControlPlane().Version().Get(s.Current.ControlPlane.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to get version of current control plane")
	}
	desiredCPVersion, err := contract.ControlPlane().Version().Get(desiredControlPlaneState.Object)
	if err != nil {
		return "", errors.Wrap(err, "failed to get version of desired control plane")
	}
	if *currentCPVersion != *desiredCPVersion {
		// The versions of the current and desired control planes do no match,
		// implies we are about to upgrade the control plane.
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion, nil
	}

	// At this point the control plane is stable (not scaling, not upgrading, not being upgraded).
	// Checking to see if the machine deployments are also stable.
	// If any of the MachineDeployments is rolling out, do not upgrade the machine deployment yet.
	if s.Current.MachineDeployments.IsAnyRollingOut() {
		s.UpgradeTracker.MachineDeployments.MarkPendingUpgrade(currentMDState.Object.Name)
		return currentVersion, nil
	}

	// Control plane and machine deployments are stable.
	// Ready to pick up the topology version.
	s.UpgradeTracker.MachineDeployments.MarkRollingOut(currentMDState.Object.Name)
	return desiredVersion, nil
}

type templateToInput struct {
	template              *unstructured.Unstructured
	templateClonedFromRef *corev1.ObjectReference
	cluster               *clusterv1.Cluster
	namePrefix            string
	currentObjectRef      *corev1.ObjectReference
	// OwnerRef is an optional OwnerReference to attach to the cloned object.
	ownerRef *metav1.OwnerReference
}

// templateToObject generates an object from a template, taking care
// of adding required labels (cluster, topology), annotations (clonedFrom)
// and assigning a meaningful name (or reusing current reference name).
func templateToObject(in templateToInput) (*unstructured.Unstructured, error) {
	// NOTE: The cluster label is added at creation time so this object could be read by the ClusterTopology
	// controller immediately after creation, even before other controllers are going to add the label (if missing).
	labels := map[string]string{}
	labels[clusterv1.ClusterLabelName] = in.cluster.Name
	labels[clusterv1.ClusterTopologyOwnedLabel] = ""

	// Generate the object from the template.
	// NOTE: OwnerRef can't be set at this stage; other controllers are going to add OwnerReferences when
	// the object is actually created.
	object, err := external.GenerateTemplate(&external.GenerateTemplateInput{
		Template:    in.template,
		TemplateRef: in.templateClonedFromRef,
		Namespace:   in.cluster.Namespace,
		Labels:      labels,
		ClusterName: in.cluster.Name,
		OwnerRef:    in.ownerRef,
	})
	if err != nil {
		return nil, err
	}

	// Ensure the generated objects have a meaningful name.
	// NOTE: In case there is already a ref to this object in the Cluster, re-use the same name
	// in order to simplify compare at later stages of the reconcile process.
	object.SetName(names.SimpleNameGenerator.GenerateName(in.namePrefix))
	if in.currentObjectRef != nil && len(in.currentObjectRef.Name) > 0 {
		object.SetName(in.currentObjectRef.Name)
	}

	return object, nil
}

// templateToTemplate generates a template from an existing template, taking care
// of adding required labels (cluster, topology), annotations (clonedFrom)
// and assigning a meaningful name (or reusing current reference name).
// NOTE: We are creating a copy of the ClusterClass template for each cluster so
// it is possible to add cluster specific information without affecting the original object.
func templateToTemplate(in templateToInput) *unstructured.Unstructured {
	template := &unstructured.Unstructured{}
	in.template.DeepCopyInto(template)

	// Remove all the info automatically assigned by the API server and not relevant from
	// the copy of the template.
	template.SetResourceVersion("")
	template.SetFinalizers(nil)
	template.SetUID("")
	template.SetSelfLink("")

	// Enforce the topology labels into the provided label set.
	// NOTE: The cluster label is added at creation time so this object could be read by the ClusterTopology
	// controller immediately after creation, even before other controllers are going to add the label (if missing).
	labels := template.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[clusterv1.ClusterLabelName] = in.cluster.Name
	labels[clusterv1.ClusterTopologyOwnedLabel] = ""
	template.SetLabels(labels)

	// Enforce cloned from annotations and removes the kubectl last-applied-configuration annotation
	// because we don't want to propagate it to the cloned template objects.
	annotations := template.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[clusterv1.TemplateClonedFromNameAnnotation] = in.templateClonedFromRef.Name
	annotations[clusterv1.TemplateClonedFromGroupKindAnnotation] = in.templateClonedFromRef.GroupVersionKind().GroupKind().String()
	delete(annotations, corev1.LastAppliedConfigAnnotation)
	template.SetAnnotations(annotations)

	// Set the owner reference.
	if in.ownerRef != nil {
		template.SetOwnerReferences([]metav1.OwnerReference{*in.ownerRef})
	}

	// Ensure the generated template gets a meaningful name.
	// NOTE: In case there is already an object ref to this template, it is required to re-use the same name
	// in order to simplify compare at later stages of the reconcile process.
	template.SetName(names.SimpleNameGenerator.GenerateName(in.namePrefix))
	if in.currentObjectRef != nil && len(in.currentObjectRef.Name) > 0 {
		template.SetName(in.currentObjectRef.Name)
	}

	return template
}

// mergeMap merges two maps into another one.
// NOTE: In case a key exists in both maps, the value in the first map is preserved.
func mergeMap(a, b map[string]string) map[string]string {
	m := make(map[string]string)
	for k, v := range b {
		m[k] = v
	}
	for k, v := range a {
		m[k] = v
	}

	// Nil the result if the map is empty, thus avoiding triggering infinite reconcile
	// given that at json level label: {} or annotation: {} is different from no field, which is the
	// corresponding value stored in etcd given that those fields are defined as omitempty.
	if len(m) == 0 {
		return nil
	}
	return m
}

func ownerReferenceTo(obj client.Object) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
	}
}

func computeMachineHealthCheck(healthCheckTarget client.Object, selector *metav1.LabelSelector, clusterName string, check *clusterv1.MachineHealthCheckClass) *clusterv1.MachineHealthCheck {
	// Create a MachineHealthCheck with the spec given in the ClusterClass.
	mhc := &clusterv1.MachineHealthCheck{
		TypeMeta: metav1.TypeMeta{
			Kind:       clusterv1.GroupVersion.WithKind("MachineHealthCheck").Kind,
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      healthCheckTarget.GetName(),
			Namespace: healthCheckTarget.GetNamespace(),
		},
		Spec: clusterv1.MachineHealthCheckSpec{
			ClusterName:         clusterName,
			Selector:            *selector,
			UnhealthyConditions: check.UnhealthyConditions,
			MaxUnhealthy:        check.MaxUnhealthy,
			UnhealthyRange:      check.UnhealthyRange,
			NodeStartupTimeout:  check.NodeStartupTimeout,
			RemediationTemplate: check.RemediationTemplate,
		},
	}
	// Default all fields in the MachineHealthCheck using the same function called in the webhook. This ensures the desired
	// state of the object won't be different from the current state due to webhook Defaulting.
	mhc.Default()
	return mhc
}

func selectorForControlPlaneMHC() *metav1.LabelSelector {
	// The selector returned here is the minimal common selector for all Machines belonging to the ControlPlane.
	// It does not include any labels set in ClusterClass, Cluster Topology or elsewhere.
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			clusterv1.ClusterTopologyOwnedLabel:    "",
			clusterv1.MachineControlPlaneLabelName: "",
		},
	}
}

func selectorForMachineDeploymentMHC(md *clusterv1.MachineDeployment) *metav1.LabelSelector {
	// The selector returned here is the minimal common selector for all MachineSets belonging to a MachineDeployment.
	// It does not include any labels set in ClusterClass, Cluster Topology or elsewhere.
	return &metav1.LabelSelector{MatchLabels: map[string]string{
		clusterv1.ClusterTopologyOwnedLabel:                 "",
		clusterv1.ClusterTopologyMachineDeploymentLabelName: md.Spec.Selector.MatchLabels[clusterv1.ClusterTopologyMachineDeploymentLabelName],
	},
	}
}
