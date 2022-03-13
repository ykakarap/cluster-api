package cluster

import (
	"fmt"
	"reflect"
)

var registry *Registry

type ResultType string

type Result struct {
	RetryAfterSeconds int
	Error             error
}

// Hooks
type BeforeClusterUpgradeHook struct{}
type AfterClusterUpgradeHook struct{}

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
}

func (r *Registry) Register(hook interface{}, ext *Extension) {
	hookType := reflect.TypeOf(hook)
	r.hooksMap[hookType] = ext
}

func (r *Registry) Call(hook interface{}) (*Result, error) {
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
	return res, res.Error
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
		{30, nil}, // Success - retry after 30 sec
		{30, nil}, // Success - retry after 30 sec
		{0, nil},  // Success
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	registry = &Registry{
		hooksMap: map[reflect.Type]*Extension{},
	}
	registry.Register(BeforeClusterUpgradeHook{}, BeforeClusterUpgradeExtension)
	registry.Register(AfterClusterUpgradeHook{}, AfterClusterUpgradeExtension)
}
