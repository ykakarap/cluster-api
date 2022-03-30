package controllers

import (
	"context"
	"fmt"
	"os"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api/api/v1beta1/index"
	"sigs.k8s.io/cluster-api/internal/test/envtest"
)

var (
	env *envtest.Environment
	ctx = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	setupIndexes := func(ctx context.Context, mgr ctrl.Manager) {
		if err := index.AddDefaultIndexes(ctx, mgr); err != nil {
			panic(fmt.Sprintf("unable to setup index: %v", err))
		}
	}
	// FIXME: set up webhook here.
	//
	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M:            m,
		SetupEnv:     func(e *envtest.Environment) { env = e },
		SetupIndexes: setupIndexes,
		SetupReconcilers: func(ctx context.Context, mgr ctrl.Manager) {
		},
	}))
}
