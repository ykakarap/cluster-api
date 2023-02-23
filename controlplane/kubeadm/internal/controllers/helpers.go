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
	"context"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/internal"
	"sigs.k8s.io/cluster-api/internal/labels"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/conditions"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
)

const kubeadmConfigKind = "KubeadmConfig"

func (r *KubeadmControlPlaneReconciler) reconcileKubeconfig(ctx context.Context, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	endpoint := cluster.Spec.ControlPlaneEndpoint
	if endpoint.IsZero() {
		return ctrl.Result{}, nil
	}

	controllerOwnerRef := *metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"))
	clusterName := util.ObjectKey(cluster)
	configSecret, err := secret.GetFromNamespacedName(ctx, r.Client, clusterName, secret.Kubeconfig)
	switch {
	case apierrors.IsNotFound(err):
		createErr := kubeconfig.CreateSecretWithOwner(
			ctx,
			r.Client,
			clusterName,
			endpoint.String(),
			controllerOwnerRef,
		)
		if errors.Is(createErr, kubeconfig.ErrDependentCertificateNotFound) {
			return ctrl.Result{RequeueAfter: dependentCertRequeueAfter}, nil
		}
		// always return if we have just created in order to skip rotation checks
		return ctrl.Result{}, createErr
	case err != nil:
		return ctrl.Result{}, errors.Wrap(err, "failed to retrieve kubeconfig Secret")
	}

	if err := r.adoptKubeconfigSecret(ctx, cluster, configSecret, kcp); err != nil {
		return ctrl.Result{}, err
	}

	// only do rotation on owned secrets
	if !util.IsControlledBy(configSecret, kcp) {
		return ctrl.Result{}, nil
	}

	needsRotation, err := kubeconfig.NeedsClientCertRotation(configSecret, certs.ClientCertificateRenewalDuration)
	if err != nil {
		return ctrl.Result{}, err
	}

	if needsRotation {
		log.Info("rotating kubeconfig secret")
		if err := kubeconfig.RegenerateSecret(ctx, r.Client, configSecret); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to regenerate kubeconfig")
		}
	}

	return ctrl.Result{}, nil
}

// Ensure the KubeadmConfigSecret has an owner reference to the control plane if it is not a user-provided secret.
func (r *KubeadmControlPlaneReconciler) adoptKubeconfigSecret(ctx context.Context, cluster *clusterv1.Cluster, configSecret *corev1.Secret, kcp *controlplanev1.KubeadmControlPlane) error {
	log := ctrl.LoggerFrom(ctx)
	controller := metav1.GetControllerOf(configSecret)

	// If the Type doesn't match the CAPI-created secret type this is a no-op.
	if configSecret.Type != clusterv1.ClusterSecretType {
		return nil
	}
	// If the secret is already controlled by KCP this is a no-op.
	if controller != nil && controller.Kind == "KubeadmControlPlane" {
		return nil
	}
	log.Info("Adopting KubeConfig secret", "Secret", klog.KObj(configSecret))
	patch, err := patch.NewHelper(configSecret, r.Client)
	if err != nil {
		return errors.Wrap(err, "failed to create patch helper for the kubeconfig secret")
	}

	// If the kubeconfig secret was created by v1alpha2 controllers, and thus it has the Cluster as the owner instead of KCP.
	// In this case remove the ownerReference to the Cluster.
	if util.IsOwnedByObject(configSecret, cluster) {
		configSecret.SetOwnerReferences(util.RemoveOwnerRef(configSecret.OwnerReferences, metav1.OwnerReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       cluster.Name,
			UID:        cluster.UID,
		}))
	}

	// Remove the current controller if one exists.
	if controller != nil {
		configSecret.SetOwnerReferences(util.RemoveOwnerRef(configSecret.OwnerReferences, *controller))
	}

	// Add the KubeadmControlPlane as the controller for this secret.
	configSecret.OwnerReferences = util.EnsureOwnerRef(configSecret.OwnerReferences,
		*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane")))

	if err := patch.Patch(ctx, configSecret); err != nil {
		return errors.Wrap(err, "failed to patch the kubeconfig secret")
	}
	return nil
}

func (r *KubeadmControlPlaneReconciler) reconcileExternalReference(ctx context.Context, cluster *clusterv1.Cluster, ref *corev1.ObjectReference) error {
	if !strings.HasSuffix(ref.Kind, clusterv1.TemplateSuffix) {
		return nil
	}

	if err := utilconversion.UpdateReferenceAPIContract(ctx, r.Client, ref); err != nil {
		return err
	}

	obj, err := external.Get(ctx, r.Client, ref, cluster.Namespace)
	if err != nil {
		return err
	}

	// Note: We intentionally do not handle checking for the paused label on an external template reference

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return err
	}

	obj.SetOwnerReferences(util.EnsureOwnerRef(obj.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}))

	return patchHelper.Patch(ctx, obj)
}

func (r *KubeadmControlPlaneReconciler) cloneConfigsAndGenerateMachine(ctx context.Context, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, bootstrapSpec *bootstrapv1.KubeadmConfigSpec, failureDomain *string) error {
	var errs []error

	infraRef, err := r.createInfraMachine(ctx, kcp, cluster)
	if err != nil {
		// Safe to return early here since no resources have been created yet.
		conditions.MarkFalse(kcp, controlplanev1.MachinesCreatedCondition, controlplanev1.InfrastructureTemplateCloningFailedReason,
			clusterv1.ConditionSeverityError, err.Error())
		return errors.Wrap(err, "failed to clone infrastructure template")
	}

	// Clone the bootstrap configuration
	bootstrapRef, err := r.createKubeadmConfig(ctx, kcp, cluster, bootstrapSpec)
	if err != nil {
		conditions.MarkFalse(kcp, controlplanev1.MachinesCreatedCondition, controlplanev1.BootstrapTemplateCloningFailedReason,
			clusterv1.ConditionSeverityError, err.Error())
		errs = append(errs, errors.Wrap(err, "failed to generate bootstrap config"))
	}

	// Only proceed to generating the Machine if we haven't encountered an error
	if len(errs) == 0 {
		if err := r.createMachine(ctx, kcp, cluster, infraRef, bootstrapRef, failureDomain); err != nil {
			conditions.MarkFalse(kcp, controlplanev1.MachinesCreatedCondition, controlplanev1.MachineGenerationFailedReason,
				clusterv1.ConditionSeverityError, err.Error())
			errs = append(errs, errors.Wrap(err, "failed to create Machine"))
		}
	}

	// If we encountered any errors, attempt to clean up any dangling resources
	if len(errs) > 0 {
		if err := r.cleanupFromGeneration(ctx, infraRef, bootstrapRef); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to cleanup generated resources"))
		}

		return kerrors.NewAggregate(errs)
	}

	return nil
}

func (r *KubeadmControlPlaneReconciler) cleanupFromGeneration(ctx context.Context, remoteRefs ...*corev1.ObjectReference) error {
	var errs []error

	for _, ref := range remoteRefs {
		if ref == nil {
			continue
		}
		config := &unstructured.Unstructured{}
		config.SetKind(ref.Kind)
		config.SetAPIVersion(ref.APIVersion)
		config.SetNamespace(ref.Namespace)
		config.SetName(ref.Name)

		if err := r.Client.Delete(ctx, config); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, errors.Wrap(err, "failed to cleanup generated resources after error"))
		}
	}

	return kerrors.NewAggregate(errs)
}

func (r *KubeadmControlPlaneReconciler) createKubeadmConfig(ctx context.Context, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster, spec *bootstrapv1.KubeadmConfigSpec) (*corev1.ObjectReference, error) {
	// FIXMe(ykakarap) Explain why we call this with spec.
	kubeadmConfig := r.computeDesiredKubeadmConfig(kcp, cluster, spec, nil)
	patchOptions := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(kcpManagerName),
	}
	if err := r.Client.Patch(ctx, kubeadmConfig, client.Apply, patchOptions...); err != nil {
		return nil, errors.Wrap(err, "failed to create KubeadmConfig: Apply failed")
	}
	return getReference(kubeadmConfig), nil
}

func (r *KubeadmControlPlaneReconciler) updateKubeadmConfig(ctx context.Context, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster, kubeadmConfig *bootstrapv1.KubeadmConfig) (*corev1.ObjectReference, error) {
	// FIXMe(ykakarap) Explain why we call this with spec in kubeadmconfig and not the once from KCP.
	updatedKubeadmConfig := r.computeDesiredKubeadmConfig(kcp, cluster, &kubeadmConfig.Spec, kubeadmConfig)
	patchOptions := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(kcpManagerName),
	}
	if err := r.Client.Patch(ctx, updatedKubeadmConfig, client.Apply, patchOptions...); err != nil {
		return nil, errors.Wrap(err, "failed to create KubeadmConfig: Apply failed")
	}
	return getReference(updatedKubeadmConfig), nil
}

func getReference(obj client.Object) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		UID:        obj.GetUID(),
	}
}

// computeDesiredKubeadmConfig computes the desired KubeadmConfig.
// This KubeadmConfig will be used during reconciliation to:
// * create a KubeadmConfig
// * update an existing KubeadmConfig
// Because we are using Server-Side-Apply we always have to calculate the full object.
// There are small differences in how we calculate the KubeadmConfig depending on if it
// is a create or update. Example: for a new KubeadmConfig we have to calculate a new name,
// while for an existing Machine we have to use the name of the existing KubeadmConfig.
func (r *KubeadmControlPlaneReconciler) computeDesiredKubeadmConfig(kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster, spec *bootstrapv1.KubeadmConfigSpec, existingKubeadmConfig *bootstrapv1.KubeadmConfig) *bootstrapv1.KubeadmConfig {
	// Create an owner reference without a controller reference because the owning controller is the machine controller
	owner := metav1.OwnerReference{
		APIVersion: controlplanev1.GroupVersion.String(),
		Kind:       "KubeadmControlPlane",
		Name:       kcp.Name,
		UID:        kcp.UID,
	}

	bootstrapConfig := &bootstrapv1.KubeadmConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       kubeadmConfigKind,
			APIVersion: bootstrapv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            names.SimpleNameGenerator.GenerateName(kcp.Name + "-"),
			Namespace:       kcp.Namespace,
			Labels:          internal.ControlPlaneMachineLabelsForCluster(kcp, cluster.Name),
			Annotations:     kcp.Spec.MachineTemplate.ObjectMeta.Annotations,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: *spec.DeepCopy(),
	}
	if existingKubeadmConfig != nil {
		bootstrapConfig.Name = existingKubeadmConfig.Name
		bootstrapConfig.UID = existingKubeadmConfig.UID
	}
	return bootstrapConfig
}

func (r *KubeadmControlPlaneReconciler) createInfraMachine(ctx context.Context, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster) (*corev1.ObjectReference, error) {
	// Since the cloned resource should eventually have a controller ref for the Machine, we create an
	// OwnerReference here without the Controller field set
	infraCloneOwner := &metav1.OwnerReference{
		APIVersion: controlplanev1.GroupVersion.String(),
		Kind:       "KubeadmControlPlane",
		Name:       kcp.Name,
		UID:        kcp.UID,
	}

	infraTemplateRef := kcp.Spec.MachineTemplate.InfrastructureRef
	// Clone the infrastructure template
	infraRef, err := external.ApplyFromTemplate(ctx, &external.ApplyFromTemplateInput{
		Manager:     kcpManagerName,
		Client:      r.Client,
		TemplateRef: &infraTemplateRef,
		Namespace:   kcp.Namespace,
		OwnerRef:    infraCloneOwner,
		ClusterName: cluster.Name,
		Labels:      internal.ControlPlaneMachineLabelsForCluster(kcp, cluster.Name),
		Annotations: kcp.Spec.MachineTemplate.ObjectMeta.Annotations,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create Infrastructure Machine object from Template %s",
			klog.KRef(infraTemplateRef.Namespace, infraTemplateRef.Name))
	}
	return infraRef, nil
}

func (r *KubeadmControlPlaneReconciler) updateInfraMachine(ctx context.Context, infraMachine *unstructured.Unstructured, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster) (*corev1.ObjectReference, error) {
	owner := &metav1.OwnerReference{
		APIVersion: controlplanev1.GroupVersion.String(),
		Kind:       "KubeadmControlPlane",
		Name:       kcp.Name,
		UID:        kcp.UID,
	}
	// Note: We do not want to pick up the InfraMachineTemplate from KCP as that could have changed. If the template
	// changes, the machines and the inframachines will be rolledout. Here, we should only pick up the in-place
	// propagating changes. Therefore, reuse the template the infra machine was created from.
	infraTemplateRef, err := getTemplateRefFromInfraMachine(infraMachine, kcp)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update InfrastructureMachine %s: failed to get InfrastructureMachineTemplate", klog.KObj(infraMachine))
	}
	updatedInfraRef, err := external.ApplyFromTemplate(ctx, &external.ApplyFromTemplateInput{
		Manager:     kcpManagerName,
		Name:        infraMachine.GetName(),
		UID:         infraMachine.GetUID(),
		Client:      r.Client,
		TemplateRef: infraTemplateRef,
		Namespace:   kcp.Namespace,
		OwnerRef:    owner,
		ClusterName: cluster.Name,
		Labels:      internal.ControlPlaneMachineLabelsForCluster(kcp, cluster.Name),
		Annotations: kcp.Spec.MachineTemplate.ObjectMeta.Annotations,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update InfrastructureMachine %s object from Template %s",
			klog.KObj(infraMachine),
			klog.KRef(infraTemplateRef.Namespace, infraTemplateRef.Name))
	}
	return updatedInfraRef, nil
}

func getTemplateRefFromInfraMachine(infraMachine *unstructured.Unstructured, kcp *controlplanev1.KubeadmControlPlane) (*corev1.ObjectReference, error) {
	annotations := infraMachine.GetAnnotations()
	infraMachineTemplateName, ok := annotations[clusterv1.TemplateClonedFromNameAnnotation]
	if !ok {
		return nil, errors.Errorf("failed to identify the template used to create InfrastructureMachine %s: missing %q annotation", klog.KObj(infraMachine), clusterv1.TemplateClonedFromNameAnnotation)
	}
	return &corev1.ObjectReference{
		Kind:       infraMachine.GetKind() + clusterv1.TemplateSuffix,
		Namespace:  infraMachine.GetNamespace(),
		Name:       infraMachineTemplateName,
		APIVersion: kcp.Spec.MachineTemplate.InfrastructureRef.APIVersion,
	}, nil

}

func (r *KubeadmControlPlaneReconciler) createMachine(ctx context.Context, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster, infraRef, bootstrapRef *corev1.ObjectReference, failureDomain *string) error {
	machine, err := r.computeDesiredMachine(kcp, cluster, infraRef, bootstrapRef, failureDomain, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create Machine: failed to compute desired machine")
	}
	patchOptions := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(kcpManagerName),
	}
	if err := r.Client.Patch(ctx, machine, client.Apply, patchOptions...); err != nil {
		return errors.Wrap(err, "failed to create Machine: Apply failed")
	}
	return nil
}

func (r *KubeadmControlPlaneReconciler) updateMachine(ctx context.Context, machine *clusterv1.Machine, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster) (*clusterv1.Machine, error) {
	updatedMachine, err := r.computeDesiredMachine(
		kcp, cluster,
		&machine.Spec.InfrastructureRef, machine.Spec.Bootstrap.ConfigRef,
		machine.Spec.FailureDomain, machine,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update Machine: failed to compute desired machine")
	}
	patchOptions := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(kcpManagerName),
	}
	if err := r.Client.Patch(ctx, updatedMachine, client.Apply, patchOptions...); err != nil {
		return nil, errors.Wrap(err, "failed to update Machine: Apply failed")
	}
	return updatedMachine, nil
}

// computeDesiredMachine computes the desired Machine.
// This Machine will be used during reconciliation to:
// * create a Machine
// * update an existing Machine
// Because we are using Server-Side-Apply we always have to calculate the full object.
// There are small differences in how we calculate the Machine depending on if it
// is a create or update. Example: for a new Machine we have to calculate a new name,
// while for an existing Machine we have to use the name of the existing Machine.
func (r *KubeadmControlPlaneReconciler) computeDesiredMachine(kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster, infraRef, bootstrapRef *corev1.ObjectReference, failureDomain *string, existingMachine *clusterv1.Machine) (*clusterv1.Machine, error) {
	var machineName string
	var machineUID types.UID
	var machineFailureDomain *string
	var version *string
	annotations := map[string]string{}
	if existingMachine == nil {
		// Creating a new machine
		machineName = names.SimpleNameGenerator.GenerateName(kcp.Name + "-")
		machineFailureDomain = failureDomain
		version = &kcp.Spec.Version

		// Machine's bootstrap config may be missing ClusterConfiguration if it is not the first machine in the control plane.
		// We store ClusterConfiguration as annotation here to detect any changes in KCP ClusterConfiguration and rollout the machine if any.
		// Nb. This annotation is read when comparing the KubeadmBootstrapConfig to check if a machine needs to be rolled out.
		clusterConfig, err := json.Marshal(kcp.Spec.KubeadmConfigSpec.ClusterConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal cluster configuration")
		}
		annotations[controlplanev1.KubeadmClusterConfigurationAnnotation] = string(clusterConfig)

		// In case this machine is being created as a consequence of a remediation, then add an annotation
		// tracking remediating data.
		// NOTE: This is required in order to track remediation retries.
		if remediationData, ok := kcp.Annotations[controlplanev1.RemediationInProgressAnnotation]; ok {
			annotations[controlplanev1.RemediationForAnnotation] = remediationData
		}
	} else {
		// Updating an existing machine
		machineName = existingMachine.Name
		machineUID = existingMachine.UID
		machineFailureDomain = existingMachine.Spec.FailureDomain
		version = existingMachine.Spec.Version

		// For existing machine only set the ClusterConfiguration annotation if the machine already has it.
		// We should not add the annotation if it was missing in the first place because we do not have enough
		// information.
		if clusterConfig, ok := existingMachine.Annotations[controlplanev1.KubeadmClusterConfigurationAnnotation]; ok {
			annotations[controlplanev1.KubeadmClusterConfigurationAnnotation] = clusterConfig
		}

		// If the machine already has remediation date then preserve it.
		// NOTE: This is required in order to track remediation retries.
		if remediationData, ok := existingMachine.Annotations[controlplanev1.RemediationInProgressAnnotation]; ok {
			annotations[controlplanev1.RemediationForAnnotation] = remediationData
		}
	}

	// Construct the basic Machine.
	desiredMachine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:       machineUID,
			Name:      machineName,
			Namespace: kcp.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KubeadmControlPlane")),
			},
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName:       cluster.Name,
			Version:           version,
			FailureDomain:     machineFailureDomain,
			InfrastructureRef: *infraRef,
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: bootstrapRef,
			},
		},
	}

	// Set the in-place mutable fields.
	// When we create a new Machine we will just create the Machine with those fields.
	// When we update an existing Machine will we update the fields on the existing Machine (in-place mutate).

	// Set labels
	desiredMachine.Labels = internal.ControlPlaneMachineLabelsForCluster(kcp, cluster.Name)
	// Note: MustFormatValue is used here as the label value can be a hash if the control plane
	// name is longer than 63 characters.
	desiredMachine.Labels[clusterv1.MachineControlPlaneNameLabel] = labels.MustFormatValue(kcp.Name)

	// Set annotations
	// Add the annotations from the MachineTemplate.
	// Note: we intentionally don't use the map directly to ensure we don't modify the map in KCP.
	for k, v := range kcp.Spec.MachineTemplate.ObjectMeta.Annotations {
		annotations[k] = v
	}
	desiredMachine.SetAnnotations(annotations)

	// Set other in-place mutable fields
	desiredMachine.Spec.NodeDrainTimeout = kcp.Spec.MachineTemplate.NodeDrainTimeout
	desiredMachine.Spec.NodeDeletionTimeout = kcp.Spec.MachineTemplate.NodeDeletionTimeout
	desiredMachine.Spec.NodeVolumeDetachTimeout = kcp.Spec.MachineTemplate.NodeVolumeDetachTimeout

	return desiredMachine, nil
}
