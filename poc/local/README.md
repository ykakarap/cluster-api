
# POC

Start dev-env + controller:

```bash
./hack/kind-install-for-capd.sh
tilt up

mkdir -p /tmp/k8s-webhook-server-${controller}
k -n ${controller}-system get secret ${controller}-webhook-service-cert -o json | jq '.data."tls.crt"' -r | base64 -d > /tmp/k8s-webhook-server-${controller}/tls.crt
k -n ${controller}-system get secret ${controller}-webhook-service-cert -o json | jq '.data."tls.key"' -r | base64 -d > /tmp/k8s-webhook-server-${controller}/tls.key

# Start controller with:
# --webhook-cert-dir=/tmp/k8s-webhook-server-capi/
# --feature-gates=MachinePool=true,ClusterResourceSet=true,ClusterTopology=true,RuntimeSDK=true
# --metrics-bind-addr=localhost:8080
# --metrics-bind-addr=0.0.0.0:8080
# --logging-format=json
# --v=2
```

Deploy

```bash
# Start rte-implementation-v1alpha1

# Deploy Extension
kubectl apply -f ./extension.yaml
```
