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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	authorization "k8s.io/api/authorization/v1"
	certv1beta1 "k8s.io/api/certificates/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/remote"
	interfaces "sigs.k8s.io/cluster-api/nodeattestation/verifier"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
)

const (
	nodeUserPrefix = "system:node"
	nodeGroup      = "system:nodes"

	usernameField = "spec.username"
)

var (
	kubeletClientUsages = []certv1beta1.KeyUsage{
		certv1beta1.UsageDigitalSignature,
		certv1beta1.UsageKeyEncipherment,
		certv1beta1.UsageClientAuth,
	}
	kubeletServerUsages = []certv1beta1.KeyUsage{
		certv1beta1.UsageKeyEncipherment,
		certv1beta1.UsageDigitalSignature,
		certv1beta1.UsageServerAuth,
	}

	kubeletClientRecognizer = csrRecognizer{
		name:       "kubelet client certificate with TPM attestation and SubjectAccessReview",
		recognize:  isNodeClientCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
	}

	kubeletServerRecognizer = csrRecognizer{
		name:       "kubelet server certificate SubjectAccessReview",
		recognize:  isNodeServerCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
	}
)

// NodeAttestationReconciler reconciles a CSR object and its attestation data.
type NodeAttestationReconciler struct {
	Client           client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker

	externalApprover interfaces.ClusterAPIApprover
	recognizers      []csrRecognizer
}

type csrRecognizer struct {
	name       string
	recognize  func(csr *certv1beta1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool
	permission authorization.ResourceAttributes
}

func (r *NodeAttestationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Machine{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	return nil
}

func (r *NodeAttestationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the machine object.
	machine := &clusterv1.Machine{}
	if err := r.Client.Get(ctx, req.NamespacedName, machine); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			// TODO (yastij): delete pending CSRs
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, machine.ObjectMeta)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// If the owner cluster is already deleted or in deletion process, delete it ClusterResourceSetBinding
	if apierrors.IsNotFound(err) || !cluster.DeletionTimestamp.IsZero() {
		log.Info("deleting ClusterResourceSetBinding because the owner Cluster no longer exists")

		return ctrl.Result{}, err
	}

	r.watchCerficateSigningRequests(ctx, cluster)

	remoteClient, err := r.Tracker.GetClient(ctx, util.ObjectKey(cluster))
	if err != nil {
		return ctrl.Result{}, err
	}

	csrList := &certv1beta1.CertificateSigningRequestList{}
	if err := remoteClient.List(ctx, csrList,
		client.MatchingLabels{clusterv1.ClusterLabelName: machine.Spec.ClusterName},
		// TODO: this way of looking up the CSR is wrong. The spec.username in the CSR will not be the nodename
		// it will be the name of the bootstrap token. We should look at other ways to find the node from the CSR.
		// May be add the nodename in the annotation section or the labels section of the CSR?
		// We need the node name here so that we can find the correct CSR to reconcile?
		// Cant we just reconcile all CSRs? (That is probably what csrapprover controller does in k8s)
		client.MatchingFields{usernameField: strings.Join([]string{nodeUserPrefix, machine.Name}, ":")}); err != nil {
		return ctrl.Result{}, err
	}

	for _, csr := range csrList.Items {
		x509cert, err := parseCSR(csr.Spec.Request)
		if err != nil {
			return ctrl.Result{}, err
		}
		// TODO move back to a slice
		authorized := false
		if kubeletClientRecognizer.recognize(&csr, x509cert) {
			authorized, err = r.externalApprover.VerifyKubeletClientAttestationData(&csr)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else if kubeletServerRecognizer.recognize(&csr, x509cert) {
			authorized, err = r.externalApprover.VerifyKubeletServingAttestationData(&csr)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		fmt.Printf("%#v\n", authorized)
	}

	return ctrl.Result{}, nil
}

func (r *NodeAttestationReconciler) authorize(ctx context.Context, csr *certv1beta1.CertificateSigningRequest, rattrs authorization.ResourceAttributes, cluster *clusterv1.Cluster) (bool, error) {
	extra := make(map[string]authorization.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorization.ExtraValue(v)
	}

	sar := &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			User:               csr.Spec.Username,
			UID:                csr.Spec.UID,
			Groups:             csr.Spec.Groups,
			Extra:              extra,
			ResourceAttributes: &rattrs,
		},
	}
	client, err := r.Tracker.GetClient(ctx, util.ObjectKey(cluster))
	if err != nil {
		return false, err
	}
	if err := client.Create(ctx, sar); err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

// ParseCSR extracts the CSR from the bytes and decodes it.
func parseCSR(pemBytes []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, errors.New("PEM block type must be CERTIFICATE REQUEST")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	return csr, nil
}

func validateCSR(csr certv1beta1.CertificateSigningRequest, machine *clusterv1.Machine, cluster *clusterv1.Cluster) error {

	if csr.Spec.Username == "" || len(csr.Spec.Groups) == 0 {
		return fmt.Errorf("username and/or groups are not set for csr %v", csr.Name)
	}

	nodeName := strings.TrimPrefix(nodeUserPrefix, csr.Spec.Username)

	if nodeName != machine.Name {
		return fmt.Errorf("nodeName: %v doesn't match the machine name %v", nodeName, machine.Name)
	}

	//	for _, group := range csr.Spec.Groups =

	return nil
}

func (r *NodeAttestationReconciler) watchCerficateSigningRequests(ctx context.Context, cluster *clusterv1.Cluster) error {
	// If there is no tracker, don't watch remote nodes
	if r.Tracker == nil {
		return nil
	}

	if err := r.Tracker.Watch(ctx, remote.WatchInput{
		Name:         "nodeattestation-watchcsr",
		Cluster:      util.ObjectKey(cluster),
		Watcher:      r.controller,
		Kind:         &certv1beta1.CertificateSigningRequest{},
		EventHandler: handler.EnqueueRequestsFromMapFunc(r.csrToMachine),
	}); err != nil {
		return err
	}
	return nil
}

func (r *NodeAttestationReconciler) csrToMachine(o client.Object) []reconcile.Request {
	csr, ok := o.(*certv1beta1.CertificateSigningRequest)
	if !ok {
		panic(fmt.Sprintf("Expected a corev1.Node, got %T", o))
	}

	machineName, namespace, err := r.externalApprover.GetMachine(csr)
	if err != nil {
		return nil
	}

	machine := &clusterv1.Machine{}
	machineKey := types.NamespacedName{Name: machineName, Namespace: namespace}
	if r.Client.Get(context.TODO(), machineKey, machine); err != nil {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: machine.Namespace,
			Name:      machine.Name,
		},
	}}
}

func machineNames(machines []*clusterv1.Machine) []string {
	result := make([]string, 0, len(machines))
	for _, m := range machines {
		result = append(result, m.Name)
	}
	return result
}

func isNodeClientCert(csr *certv1beta1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(csr, x509cr) {
		return false
	}
	if csr.Spec.SignerName != nil && *csr.Spec.SignerName != certv1beta1.KubeAPIServerClientKubeletSignerName {
		return false
	}
	if len(x509cr.EmailAddresses) > 0 || len(x509cr.URIs) > 0 || len(x509cr.DNSNames) > 0 || len(x509cr.IPAddresses) > 0 {
		return false
	}

	return hasExactUsages(csr, kubeletClientUsages)
}

func isNodeServerCert(csr *certv1beta1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(csr, x509cr) {
		return false
	}
	if csr.Spec.SignerName != nil && *csr.Spec.SignerName != certv1beta1.KubeletServingSignerName {
		return false
	}
	if !hasExactUsages(csr, kubeletServerUsages) {
		return false
	}
	return csr.Spec.Username == x509cr.Subject.CommonName
}

func isNodeCert(_ *certv1beta1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return false
	}
	return strings.HasPrefix(x509cr.Subject.CommonName, "system:node:")
}

func hasExactUsages(csr *certv1beta1.CertificateSigningRequest, usages []certv1beta1.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}

	usageMap := map[certv1beta1.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range csr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}
