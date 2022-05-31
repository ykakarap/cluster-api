/*
Copyright 2022 The Kubernetes Authors.

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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/patches/variables"
	patchvariables "sigs.k8s.io/cluster-api/internal/controllers/topology/cluster/patches/variables"
	runtimecatalog "sigs.k8s.io/cluster-api/internal/runtime/catalog"
	infrav1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
)

var (
	catalog = runtimecatalog.New()
	scheme  = runtime.NewScheme()
	decoder runtime.Decoder

	setupLog = ctrl.Log.WithName("setup")

	webhookPort    int
	webhookCertDir string
	logOptions     = logs.NewOptions()
)

func init() {
	_ = infrav1.AddToScheme(scheme)
	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder(infrav1.GroupVersion)

	_ = runtimehooksv1.AddToCatalog(catalog)

}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logOptions.AddFlags(fs)

	fs.IntVar(&webhookPort, "webhook-port", 9443,
		"Webhook Server port")

	fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")
}

func main() {
	InitFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if err := logOptions.ValidateAndApply(); err != nil {
		setupLog.Error(err, "unable to start extension")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	// FIXME(sbueringer) think about defaults, cf with CR.
	srv := webhook.Server{
		Host:          "",
		Port:          webhookPort,
		CertDir:       webhookCertDir,
		CertName:      "tls.crt",
		KeyName:       "tls.key",
		WebhookMux:    http.NewServeMux(),
		TLSMinVersion: "1.2",
	}

	operation1Handler, err := NewHandlerBuilder().
		WithCatalog(catalog).
		AddDiscovery(runtimehooksv1.Discovery, doDiscovery). // TODO: this is not strongly typed, but there are type checks when the service starts
		AddExtension(runtimehooksv1.GeneratePatches, "generate-patches", generatePatches).
		AddExtension(runtimehooksv1.ValidateTopology, "validate-topology", validateTopology).
		// TODO: test with more services
		Build()
	if err != nil {
		panic(err)
	}

	srv.WebhookMux.Handle("/", operation1Handler)

	setupLog.Info("starting RuntimeExtension")
	if err := srv.StartStandalone(ctx, nil); err != nil {
		panic(err)
	}
}

// TODO: consider registering extensions with all required data and then auto-generating the discovery func based on that.
// If we want folks to write it manually, make it nicer to do.
func doDiscovery(request *runtimehooksv1.DiscoveryRequest, response *runtimehooksv1.DiscoveryResponse) error {
	fmt.Println("Discovery/v1alpha1 called")

	response.Status = runtimehooksv1.ResponseStatusSuccess
	response.Handlers = append(response.Handlers, runtimehooksv1.ExtensionHandler{
		Name: "generate-patches",
		RequestHook: runtimehooksv1.GroupVersionHook{
			APIVersion: runtimehooksv1.GroupVersion.String(),
			Hook:       "GeneratePatches",
		},
		TimeoutSeconds: pointer.Int32(10),
		FailurePolicy:  toPtr(runtimehooksv1.FailurePolicyFail),
	})
	response.Handlers = append(response.Handlers, runtimehooksv1.ExtensionHandler{
		Name: "validate-topology",
		RequestHook: runtimehooksv1.GroupVersionHook{
			APIVersion: runtimehooksv1.GroupVersion.String(),
			Hook:       "ValidateTopology",
		},
		TimeoutSeconds: pointer.Int32(10),
		FailurePolicy:  toPtr(runtimehooksv1.FailurePolicyFail),
	})

	return nil
}

func generatePatches(req *runtimehooksv1.GeneratePatchesRequest, resp *runtimehooksv1.GeneratePatchesResponse) error {
	// FIXME(sbueringer): try to implement actual patching + validation below, includes
	// * parse object
	// * similar to inline patching e2e test: modify object based on variables (we probably would need some libs if we do it right)

	globalVariables := toMap(req.Variables)

	for _, requestItem := range req.Items {
		templateVariables, err := mergeVariableMaps(globalVariables, toMap(requestItem.Variables))
		if err != nil {
			return err
		}

		obj, _, err := decoder.Decode(requestItem.Object.Raw, nil, requestItem.Object.Object)
		if err != nil {
			// Continue, object has a type which hasn't been registered with the scheme.
			continue
		}

		original := obj.DeepCopyObject()
		var modified runtime.Object

		switch v := obj.(type) {
		case *infrav1.DockerClusterTemplate:
			if err := patchDockerClusterTemplate(v, templateVariables); err != nil {
				return err
			}
			modified = v
		}

		if modified == nil {
			// No patching was done, let's continue with the next object.
			continue
		}

		patch, err := createPatch(original, modified)
		if err != nil {
			return err
		}

		resp.Items = append(resp.Items, runtimehooksv1.GeneratePatchesResponseItem{
			UID:       requestItem.UID,
			PatchType: runtimehooksv1.JSONPatchType,
			Patch:     patch,
		})

		fmt.Printf("Generated patch (uid: %q): %q\n", requestItem.UID, string(patch))
	}

	resp.Status = runtimehooksv1.ResponseStatusSuccess
	fmt.Println("GeneratePatches called")
	return nil
}

func createPatch(original, modified runtime.Object) ([]byte, error) {
	marshalledOriginal, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	marshalledModified, err := json.Marshal(modified)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(marshalledOriginal, marshalledModified)
	if err != nil {
		return nil, err
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	return patchBytes, nil
}

func patchDockerClusterTemplate(dockerClusterTemplate *infrav1.DockerClusterTemplate, templateVariables map[string]apiextensionsv1.JSON) error {
	value, err := variables.GetVariableValue(templateVariables, "lbImageRepository")
	if err != nil {
		// FIXME(sbueringer): need better error semantics like variable not found/set)
		return err
	}
	stringValue, err := strconv.Unquote(string(value.Raw))
	if err != nil {
		return err
	}

	dockerClusterTemplate.Spec.Template.Spec.LoadBalancer.ImageRepository = stringValue
	return nil
}

func validateTopology(req *runtimehooksv1.ValidateTopologyRequest, resp *runtimehooksv1.ValidateTopologyResponse) error {
	fmt.Println("ValidateTopology called")
	resp.Status = runtimehooksv1.ResponseStatusSuccess
	return nil
}

func toPtr(f runtimehooksv1.FailurePolicy) *runtimehooksv1.FailurePolicy {
	return &f
}

// FIXME(sbueringer): dedupliate

// toMap converts a list of Variables to a map of JSON (name is the map key).
func toMap(variables []runtimehooksv1.Variable) map[string]apiextensionsv1.JSON {
	variablesMap := map[string]apiextensionsv1.JSON{}

	for i := range variables {
		variablesMap[variables[i].Name] = variables[i].Value
	}
	return variablesMap
}

// mergeVariableMaps merges variables.
// NOTE: In case a variable exists in multiple maps, the variable from the latter map is preserved.
// NOTE: The builtin variable object is merged instead of simply overwritten.
func mergeVariableMaps(variableMaps ...map[string]apiextensionsv1.JSON) (map[string]apiextensionsv1.JSON, error) {
	res := make(map[string]apiextensionsv1.JSON)

	for _, variableMap := range variableMaps {
		for variableName, variableValue := range variableMap {
			// If the variable already exits and is the builtin variable, merge it.
			if _, ok := res[variableName]; ok && variableName == patchvariables.BuiltinsName {
				mergedV, err := mergeBuiltinVariables(res[variableName], variableValue)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to merge builtin variables")
				}
				res[variableName] = *mergedV
				continue
			}
			res[variableName] = variableValue
		}
	}

	return res, nil
}

// mergeBuiltinVariables merges builtin variable objects.
// NOTE: In case a variable exists in multiple builtin variables, the variable from the latter map is preserved.
func mergeBuiltinVariables(variableList ...apiextensionsv1.JSON) (*apiextensionsv1.JSON, error) {
	builtins := &patchvariables.Builtins{}

	// Unmarshal all variables into builtins.
	// NOTE: This accumulates the fields on the builtins.
	// Fields will be overwritten by later Unmarshals if fields are
	// set on multiple variables.
	for _, variable := range variableList {
		if err := json.Unmarshal(variable.Raw, builtins); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal builtin variable")
		}
	}

	// Marshal builtins to JSON.
	builtinVariableJSON, err := json.Marshal(builtins)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal builtin variable")
	}

	return &apiextensionsv1.JSON{
		Raw: builtinVariableJSON,
	}, nil
}
