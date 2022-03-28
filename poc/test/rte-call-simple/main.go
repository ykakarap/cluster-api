package main

import (
	"context"
	"fmt"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	runtimev1 "sigs.k8s.io/cluster-api/exp/runtime/api/v1beta1"
	"sigs.k8s.io/cluster-api/exp/runtime/controllers"
	"sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha3"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
	rtclient "sigs.k8s.io/cluster-api/internal/runtime/client"
	"sigs.k8s.io/cluster-api/internal/runtime/registry"
)

// go run rte/test/rte-call-simple/main.go

var c = catalog.New()

func init() {
	_ = v1alpha3.AddToCatalog(c)
}

func main() {
	ctx := context.Background()

	var mgr ctrl.Manager

	r := registry.New()

	runtimeClient := rtclient.New(rtclient.Options{
		Catalog:  c,
		Registry: r,
	})

	if err := (&controllers.ExtensionReconciler{
		Client:        mgr.GetClient(),
		Registry:      r,
		RuntimeClient: runtimeClient,
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		os.Exit(1)
	}

	ext := &runtimev1.Extension{}

	runtimeExtensions, err := runtimeClient.Extension(ext).Discover()
	if err != nil {
		panic(err)
	}
	fmt.Println(runtimeExtensions)

	in := &v1alpha3.DiscoveryHookRequest{First: 1, Second: "Hello CAPI runtime extensions!"}
	out := &v1alpha3.DiscoveryHookResponse{}

	runtimeClient.Hook(v1alpha3.DiscoveryHook).Call(ctx, "http-proxy.patch", in, out)

	runtimeClient.Hook(v1alpha3.DiscoveryHook).CallAll(ctx, in, out)

	//runtimeClient = http.NewClientBuilder().
	//	WithCatalog(c).
	//	Host(fmt.Sprintf("http://%s", net.JoinHostPort("127.0.0.1", "8083"))).
	//	Build()
	//
	//
	//if err := runtimeClient.ServiceOld(hook, rtclient.SpecVersion("v1alpha3")).Invoke(ctx, in, out); err != nil {
	//	panic(err)
	//}

	fmt.Printf("Result: %v\n", out.Message)
}
