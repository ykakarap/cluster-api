package cluster

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/util/patch"
)

const HookTrackerAnnotationKey = "hooks.x-cluster.k8s.io/tracking"

var registry *Registry

type ResultType string

type Result struct {
	RetryAfterSeconds int
	Error             error
}

// Hooks
type BeforeClusterUpgradeHook struct{}
type AfterClusterUpgradeHook struct{}
type BeforeClusterCreateHook struct{}
type BeforeClusterDeleteHook struct{}
type AfterFirstControlPlaneReadyHook struct{}

type Extension struct {
	Name    string
	Results []*Result
	index   int
}

func (e *Extension) Handle() *Result {
	res := e.Results[min(e.index, len(e.Results)-1)]
	e.index++
	return res
}

type Registry struct {
	hooksMap map[reflect.Type]*Extension
	client   client.Client
}

func (r *Registry) Register(hook interface{}, ext *Extension) {
	hookType := reflect.TypeOf(hook)
	r.hooksMap[hookType] = ext
}

func (r *Registry) SetClient(client client.Client) {
	if r.client == nil {
		r.client = client
	}
}

func (r *Registry) Call(hook interface{}, obj client.Object) (*Result, error) {
	hookType := reflect.TypeOf(hook)
	extension, ok := r.hooksMap[hookType]
	if !ok {
		// no extensions are registered against this hook
		// return a success here
		return &Result{
			RetryAfterSeconds: 0,
			Error:             nil,
		}, nil
	}
	res := extension.Handle()
	fmt.Printf("Runtime Extension %q returned result %+v\n", extension.Name, res)
	if res.Error == nil && res.RetryAfterSeconds == 0 {
		// If the hook is called successfully then we can drop it form the tracker
		r.Done(hook, obj)
	}
	return res, res.Error
}

func (r *Registry) Track(hook interface{}, obj client.Object) (retErr error) {
	patchHelper, err := patch.NewHelper(obj, r.client)
	if err != nil {
		return errors.Wrap(err, "failed to create a patch helper")
	}
	defer func() {
		if err := patchHelper.Patch(context.TODO(), obj); err != nil {
			retErr = errors.Wrap(err, "failed to patch the object")
		}
	}()
	hookName := reflect.TypeOf(hook).Name()
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	tracker := annotations[HookTrackerAnnotationKey]
	tracker = addToAnnotation(tracker, hookName)
	annotations[HookTrackerAnnotationKey] = tracker
	obj.SetAnnotations(annotations)

	// Add finalizer if the hook requires one. Only for BeforeClusterDelete.
	if hasFinalizer(hook) {
		finalizers := obj.GetFinalizers()
		if finalizers == nil {
			finalizers = []string{}
		}
		finalizers = append(finalizers, hookName)
		obj.SetFinalizers(finalizers)
	}
	return nil
}

func (r *Registry) Tracked(hook interface{}, obj client.Object) bool {
	hookName := reflect.TypeOf(hook).Name()
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	return isInAnnotation(
		annotations[HookTrackerAnnotationKey],
		hookName,
	)
}

func (r *Registry) Done(hook interface{}, obj client.Object) (retErr error) {
	patchHelper, err := patch.NewHelper(obj, r.client)
	if err != nil {
		return errors.Wrap(err, "failed to create a patch helper")
	}
	defer func() {
		if err := patchHelper.Patch(context.TODO(), obj); err != nil {
			retErr = errors.Wrap(err, "failed to patch the object")
		}
	}()
	hookName := reflect.TypeOf(hook).Name()
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	tracker := annotations[HookTrackerAnnotationKey]
	tracker = removeFromAnnotation(tracker, hookName)
	annotations[HookTrackerAnnotationKey] = tracker
	obj.SetAnnotations(annotations)

	// Remove finalizer if the hook requires one. Only for BeforeClusterDelete.
	if hasFinalizer(hook) {
		finalizers := obj.GetFinalizers()
		if finalizers == nil {
			finalizers = []string{}
		}
		for i, finalizer := range finalizers {
			if finalizer == reflect.TypeOf(hook).Name() {
				finalizers = append(finalizers[:i], finalizers[i+1:]...)
			}
		}
		obj.SetFinalizers(finalizers)
	}
	return nil
}

// BeforeClusterUpgradeExtension
var BeforeClusterUpgradeExtension = &Extension{
	Name: "BeforeClusterUpgradeExtension",
	Results: []*Result{
		{30, nil}, // Success - retry after 30 sec
		{30, nil}, // Success - retry after 30 s
		{0, nil},  // Success
	},
}

var AfterClusterUpgradeExtension = &Extension{
	Name: "AfterClusterUpgradeExtension",
	Results: []*Result{
		{0, nil}, // Success
	},
}

var BeforeClusterCreateExtension = &Extension{
	Name: "BeforeClusterCreateExtension",
	Results: []*Result{
		{30, nil}, // Success - retry after 30 sec
		{30, nil}, // Success - retry after 30 s
		{0, nil},  // Success
	},
}

var AfterFirstControlPlaneReadyExtension = &Extension{
	Name: "AfterFirstControlPlaneReadyExtension",
	Results: []*Result{
		{0, nil}, // Success
	},
}

var BeforeClusterDeleteExtension = &Extension{
	Name: "BeforeClusterDeleteExtension",
	Results: []*Result{
		{30, nil}, // Success - retry after 30 sec
		{30, nil}, // Success - retry after 30 s
		{0, nil},  // Success
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func addToAnnotation(list, hook string) string {
	hooks := strings.Split(list, ",")
	res := addToListIfMissing(hooks, hook)
	return strings.Join(res, ",")
}

func removeFromAnnotation(list, hook string) string {
	hooks := strings.Split(list, ",")
	res := removeFromList(hooks, hook)
	return strings.Join(res, ",")
}

func isInAnnotation(list, hook string) bool {
	hooks := strings.Split(list, ",")
	for _, v := range hooks {
		if v == hook {
			return true
		}
	}
	return false
}

func addToListIfMissing(list []string, item string) []string {
	found := false
	for _, v := range list {
		if v == item {
			found = true
			break
		}
	}
	if !found {
		return append(list, item)
	}
	return list
}

func removeFromList(list []string, item string) []string {
	res := []string{}
	for _, v := range list {
		if v != item {
			res = append(res, v)
		}
	}
	return res
}

func init() {
	registry = &Registry{
		hooksMap: map[reflect.Type]*Extension{},
	}
	registry.Register(BeforeClusterUpgradeHook{}, BeforeClusterUpgradeExtension)
	registry.Register(AfterClusterUpgradeHook{}, AfterClusterUpgradeExtension)
	registry.Register(BeforeClusterCreateHook{}, BeforeClusterCreateExtension)
	registry.Register(AfterFirstControlPlaneReadyHook{}, AfterFirstControlPlaneReadyExtension)
	registry.Register(BeforeClusterDeleteHook{}, BeforeClusterDeleteExtension)
}

func hasFinalizer(hook interface{}) bool {
	return reflect.TypeOf(hook) == reflect.TypeOf(BeforeClusterDeleteExtension)
}
