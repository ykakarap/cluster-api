# Overview

In order to demonstrate how to develop a new Cluster API provider we will use
`kubebuilder` to create an example provider. For more information on `kubebuilder`
and CRDs in general we highly recommend reading the [Kubebuilder Book][kubebuilder-book].
Much of the information here was adapted directly from it.

This is an _infrastructure_ provider - tasked with managing provider-specific resources for clusters and machines.
There are also [bootstrap providers][bootstrap], which turn machines into Kubernetes nodes.

[bootstrap]: ../../../reference/providers.md#bootstrap

## Prerequisites

- Install [`kubectl`][kubectl-install]
- Install [`kustomize`][install-kustomize]
- Install [`kubebuilder`][install-kubebuilder]

### tl;dr

{{#tabs name:"kubectl and kustomize" tabs:"MacOS,Linux"}}
{{#tab MacOS}}

```bash
# Install kubectl
brew install kubernetes-cli

# Install kustomize
brew install kustomize
```
{{#/tab }}
{{#tab Linux}}

```bash
# Install kubectl
KUBECTL_VERSION=$(curl -sfL https://dl.k8s.io/release/stable.txt)
curl -fLO https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl

# Install kustomize
OS_TYPE=linux
FILE=kustomize_*_${OS_TYPE}_amd64.tar.gz
curl -sf https://api.github.com/repos/kubernetes-sigs/kustomize/releases/latest |\
  grep browser_download |\
  grep ${OS_TYPE} |\
  cut -d '"' -f 4 |\
  xargs curl -f -O -L
tar zxvf $FILE; rm -f $FILE
sudo mv kustomize /usr/local/bin/kustomize
sudo chmod u+x /usr/local/bin/kustomize
```

{{#/tab }}
{{#/tabs }}

```
# Install Kubebuilder
os=$(go env GOOS)
arch=$(go env GOARCH)

# download kubebuilder and extract it to tmp
curl -sL https://go.kubebuilder.io/dl/2.1.0/${os}/${arch} | tar -xz -C /tmp/

# move to a long-term location and put it on your path
# (you'll need to set the KUBEBUILDER_ASSETS env var if you put it somewhere else)
sudo mv /tmp/kubebuilder_2.1.0_${os}_${arch} /usr/local/kubebuilder
export PATH=$PATH:/usr/local/kubebuilder/bin
```

[kubebuilder-book]: https://book.kubebuilder.io/
[kubectl-install]: http://kubernetes.io/docs/user-guide/prereqs/
[install-kustomize]: https://kubectl.docs.kubernetes.io/installation/kustomize/
[install-kubebuilder]:  https://book.kubebuilder.io/quick-start.html#installation
