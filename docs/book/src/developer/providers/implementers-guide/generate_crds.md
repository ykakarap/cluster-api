### Create a repository

```bash
mkdir cluster-api-provider-mailgun
cd src/sigs.k8s.io/cluster-api-provider-mailgun
git init
```

You'll then need to set up [go modules][gomod]

```bash
$ go mod init github.com/liztio/cluster-api-provider-mailgun
go: creating new go.mod: module github.com/liztio/cluster-api-provider-mailgun
```
[gomod]: https://github.com/golang/go/wiki/Modules#how-to-define-a-module

### Generate scaffolding

```bash
kubebuilder init --domain cluster.x-k8s.io
```

`kubebuilder init` will create the basic repository layout, including a simple containerized manager.
It will also initialize the external go libraries that will be required to build your project.

Commit your changes so far:

```bash
git commit -m "Generate scaffolding."
```

### Generate provider resources for Clusters and Machines

Here you will be asked if you want to generate resources and controllers.
You'll want both of them:

```bash
kubebuilder create api --group infrastructure --version v1alpha3 --kind MailgunCluster
kubebuilder create api --group infrastructure --version v1alpha3 --kind MailgunMachine
```

```
Create Resource under pkg/apis [y/n]?
y
Create Controller under pkg/controller [y/n]?
y
```

### Add Status subresource

The [status subresource][status] lets Spec and Status requests for custom resources be addressed separately so requests don't conflict with each other.
It also lets you split RBAC rules between Spec and Status.
It's stable in Kubernetes as of [v1.16][rbac], but you will have to [manually enable it in Kubebuilder][kbstatus].

Add the `subresource:status` annotation to your `<provider>cluster_types.go` `<provider>machine_types.go`

```go
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// MailgunCluster is the Schema for the mailgunclusters API
type MailgunCluster struct {
```

```go
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// MailgunMachine is the Schema for the mailgunmachines API
type MailgunMachine struct {
```

And regenerate the CRDs:
```shell
make manifests
```

[status]:  https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#status-subresource
[rbac]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#customresourcesubresources-v1beta1-apiextensions-k8s-io
[kbstatus]: https://book.kubebuilder.io/reference/generating-crd.html?highlight=status#status

### Apply further customizations

The cluster API CRDs should be further customized:

- [Apply the contract version label to support conversions](../contracts.md#api-version-labels)
- [Ensure you are compliant with the clusterctl provider contract](../../../clusterctl/provider-contract.md#components-yaml)

### Commit your changes

```bash
git add .
git commit -m "Generate Cluster and Machine resources."
```
