/*
Copyright 2019 The Kubernetes Authors.

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

package repository

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/internal/scheme"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/internal/util"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"
)

const (
	namespaceKind                      = "Namespace"
	clusterRoleKind                    = "ClusterRole"
	clusterRoleBindingKind             = "ClusterRoleBinding"
	roleBindingKind                    = "RoleBinding"
	validatingWebhookConfigurationKind = "ValidatingWebhookConfiguration"
	mutatingWebhookConfigurationKind   = "MutatingWebhookConfiguration"
	customResourceDefinitionKind       = "CustomResourceDefinition"
	deploymentKind                     = "Deployment"

	WebhookNamespaceName = "capi-webhook-system"

	controllerContainerName = "manager"
	namespaceArgPrefix      = "--namespace="
)

// variableRegEx defines the regexp used for searching variables inside a YAML
var variableRegEx = regexp.MustCompile(`\${\s*([A-Z0-9_]+)\s*}`)

// Components wraps a YAML file that defines the provider components
// to be installed in a management cluster (CRD, Controller, RBAC etc.)
// It is important to notice that clusterctl applies a set of processing steps to the “raw” component YAML read
// from the provider repositories:
// 1. Checks for all the variables in the component YAML file and replace with corresponding config values
// 2. Ensure all the provider components are deployed in the target namespace (apply only to namespaced objects)
// 3. Ensure all the ClusterRoleBinding which are referencing namespaced objects have the name prefixed with the namespace name
// 4. Set the watching namespace for the provider controller
// 5. Adds labels to all the components in order to allow easy identification of the provider objects
type Components interface {
	// configuration of the provider the provider components belongs to.
	config.Provider

	// Version of the provider.
	Version() string

	// Variables required by the provider components.
	// This value is derived by the component YAML.
	Variables() []string

	// Images required to install the provider components.
	// This value is derived by the component YAML.
	Images() []string

	// TargetNamespace where the provider components will be installed.
	// By default this value is derived by the component YAML, but it is possible to override it
	// during the creation of the Components object.
	TargetNamespace() string

	// WatchingNamespace defines the namespace where the provider controller is is watching (empty means all namespaces).
	// By default this value is derived by the component YAML, but it is possible to override it
	// during the creation of the Components object.
	WatchingNamespace() string

	// InventoryObject returns the clusterctl inventory object representing the provider that will be
	// generated by this components.
	InventoryObject() clusterctlv1.Provider

	// Yaml return the provider components in the form of a YAML file.
	Yaml() ([]byte, error)

	// InstanceObjs return the instance specific components in the form of a list of Unstructured objects.
	InstanceObjs() []unstructured.Unstructured

	// SharedObjs returns CRDs, web-hooks and all the components shared across instances in the form of a list of Unstructured objects.
	SharedObjs() []unstructured.Unstructured
}

// components implement Components
type components struct {
	config.Provider
	version           string
	variables         []string
	images            []string
	targetNamespace   string
	watchingNamespace string
	instanceObjs      []unstructured.Unstructured
	sharedObjs        []unstructured.Unstructured
}

// ensure components implement Components
var _ Components = &components{}

func (c *components) Version() string {
	return c.version
}

func (c *components) Variables() []string {
	return c.variables
}

func (c *components) Images() []string {
	return c.images
}

func (c *components) TargetNamespace() string {
	return c.targetNamespace
}

func (c *components) WatchingNamespace() string {
	return c.watchingNamespace
}

func (c *components) InventoryObject() clusterctlv1.Provider {
	labels := getCommonLabels(c.Provider)
	labels[clusterctlv1.ClusterctlCoreLabelName] = "inventory"

	return clusterctlv1.Provider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterctlv1.GroupVersion.String(),
			Kind:       "Provider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.targetNamespace,
			Name:      c.ManifestLabel(),
			Labels:    labels,
		},
		ProviderName:     c.Name(),
		Type:             string(c.Type()),
		Version:          c.version,
		WatchedNamespace: c.watchingNamespace,
	}
}

func (c *components) InstanceObjs() []unstructured.Unstructured {
	return c.instanceObjs
}

func (c *components) SharedObjs() []unstructured.Unstructured {
	return c.sharedObjs
}

func (c *components) Yaml() ([]byte, error) {
	objs := []unstructured.Unstructured{}
	objs = append(objs, c.sharedObjs...)
	objs = append(objs, c.instanceObjs...)

	return utilyaml.FromUnstructured(objs)
}

// ComponentsOptions is the inputs needed by the NewComponents
type ComponentsOptions struct {
	Version           string
	TargetNamespace   string
	WatchingNamespace string
	// Allows for skipping variable replacement in the component YAML
	SkipVariables bool
}

// NewComponents returns a new objects embedding a component YAML file
//
// It is important to notice that clusterctl applies a set of processing steps to the “raw” component YAML read
// from the provider repositories:
// 1. Checks for all the variables in the component YAML file and replace with corresponding config values
// 2. The variables replacement can be skipped using the SkipVariables flag in the input options
// 3. Ensure all the provider components are deployed in the target namespace (apply only to namespaced objects)
// 4. Ensure all the ClusterRoleBinding which are referencing namespaced objects have the name prefixed with the namespace name
// 5. Set the watching namespace for the provider controller
// 6. Adds labels to all the components in order to allow easy identification of the provider objects
func NewComponents(provider config.Provider, configClient config.Client, rawyaml []byte, options ComponentsOptions) (*components, error) {
	// Inspect the yaml read from the repository for variables.
	variables := inspectVariables(rawyaml)

	// Replace variables with corresponding values read from the config
	yaml, err := replaceVariables(rawyaml, variables, configClient.Variables(), options.SkipVariables)
	if err != nil {
		return nil, errors.Wrap(err, "failed to perform variable substitution")
	}

	// Transform the yaml in a list of objects, so following transformation can work on typed objects (instead of working on a string/slice of bytes)
	objs, err := utilyaml.ToUnstructured(yaml)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse yaml")
	}

	// Apply image overrides, if defined
	objs, err = util.FixImages(objs, func(image string) (string, error) {
		return configClient.ImageMeta().AlterImage(provider.ManifestLabel(), image)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to apply image overrides")
	}

	// Inspect the list of objects for the images required by the provider component.
	images, err := util.InspectImages(objs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect required images")
	}

	// splits the component resources from the shared resources.
	// This is required because a component yaml is designed for allowing users to create a single instance of a provider
	// by running kubectl apply, while multi-tenant installations requires manual modifications to the yaml file.
	// clusterctl does such modification for the user, and in order to do so, it is required to split objects in two sets;
	// components resources are processed in order to make instance specific modifications, while identifying labels
	// are applied to shared resources.
	instanceObjs, sharedObjs := splitInstanceAndSharedResources(objs)

	// inspect the list of objects for the default target namespace
	// the default target namespace is the namespace object defined in the component yaml read from the repository, if any
	defaultTargetNamespace, err := inspectTargetNamespace(instanceObjs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect default target namespace")
	}

	// Ensures all the provider components are deployed in the target namespace (apply only to namespaced objects)
	// if targetNamespace is not specified, then defaultTargetNamespace is used. In case both targetNamespace and defaultTargetNamespace
	// are empty, an error is returned

	if options.TargetNamespace == "" {
		options.TargetNamespace = defaultTargetNamespace
	}

	if options.TargetNamespace == "" {
		return nil, errors.New("target namespace can't be defaulted. Please specify a target namespace")
	}

	// add a Namespace object if missing (ensure the targetNamespace will be created)
	instanceObjs = addNamespaceIfMissing(instanceObjs, options.TargetNamespace)

	// fix Namespace name in all the objects
	instanceObjs = fixTargetNamespace(instanceObjs, options.TargetNamespace)

	// ensures all the ClusterRole and ClusterRoleBinding have the name prefixed with the namespace name and that
	// all the clusterRole/clusterRoleBinding namespaced subjects refers to targetNamespace
	// Nb. Making all the RBAC rules "namespaced" is required for supporting multi-tenancy
	instanceObjs, err = fixRBAC(instanceObjs, options.TargetNamespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fix ClusterRoleBinding names")
	}

	// inspect the list of objects for the default watching namespace
	// the default watching namespace is the namespace the controller is set for watching in the component yaml read from the repository, if any
	defaultWatchingNamespace, err := inspectWatchNamespace(instanceObjs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect default watching namespace")
	}

	// if the requested watchingNamespace is different from the defaultWatchingNamespace, fix it
	if defaultWatchingNamespace != options.WatchingNamespace {
		instanceObjs, err = fixWatchNamespace(instanceObjs, options.WatchingNamespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set watching namespace")
		}
	}

	// Add common labels to both the obj groups.
	instanceObjs = addCommonLabels(instanceObjs, provider)
	sharedObjs = addCommonLabels(sharedObjs, provider)

	// Add an identifying label to shared components so next invocation of init, clusterctl delete and clusterctl upgrade can act accordingly.
	// Additionally, the capi-webhook-system namespace gets detached from any provider, so we prevent that deleting
	// a provider can delete all the web-hooks.
	sharedObjs = fixSharedLabels(sharedObjs)

	return &components{
		Provider:          provider,
		version:           options.Version,
		variables:         variables,
		images:            images,
		targetNamespace:   options.TargetNamespace,
		watchingNamespace: options.WatchingNamespace,
		instanceObjs:      instanceObjs,
		sharedObjs:        sharedObjs,
	}, nil
}

// splitInstanceAndSharedResources divides the objects contained in the component yaml into two sets, instance specific objects
// and objects shared across many instances.
func splitInstanceAndSharedResources(objs []unstructured.Unstructured) (instanceObjs []unstructured.Unstructured, sharedObjs []unstructured.Unstructured) {
	for _, o := range objs {
		// CRDs, and web-hook objects are shared among instances.
		if o.GetKind() == customResourceDefinitionKind ||
			o.GetKind() == mutatingWebhookConfigurationKind ||
			o.GetKind() == validatingWebhookConfigurationKind {
			sharedObjs = append(sharedObjs, o)
			continue
		}

		// Web-hook objects are backed by a controller handling the web-hook calls; byt definition this
		// controller and everything releted to it (eg. services, certificates) it is expected to be deployed in well
		// know namespace named capi-webhook-system.
		// So this namespace and all the objected belonging to it are considered shared resources.
		if o.GetKind() == namespaceKind && o.GetName() == WebhookNamespaceName {
			sharedObjs = append(sharedObjs, o)
			continue
		}

		if util.IsResourceNamespaced(o.GetKind()) && o.GetNamespace() == WebhookNamespaceName {
			sharedObjs = append(sharedObjs, o)
			continue
		}

		// Everything else is considered an instance specific object.
		instanceObjs = append(instanceObjs, o)
	}
	return
}

func inspectVariables(data []byte) []string {
	variables := sets.NewString()
	match := variableRegEx.FindAllStringSubmatch(string(data), -1)

	for _, m := range match {
		submatch := m[1]
		if !variables.Has(submatch) {
			variables.Insert(submatch)
		}
	}

	ret := variables.List()
	sort.Strings(ret)
	return ret
}

func replaceVariables(yaml []byte, variables []string, configVariablesClient config.VariablesClient, skipVariables bool) ([]byte, error) {
	tmp := string(yaml)
	var missingVariables []string
	for _, key := range variables {
		val, err := configVariablesClient.Get(key)
		if err != nil {
			missingVariables = append(missingVariables, key)
			continue
		}
		exp := regexp.MustCompile(`\$\{\s*` + regexp.QuoteMeta(key) + `\s*\}`)
		tmp = exp.ReplaceAllLiteralString(tmp, val)
	}
	if !skipVariables && len(missingVariables) > 0 {
		return nil, errors.Errorf("value for variables [%s] is not set. Please set the value using os environment variables or the clusterctl config file", strings.Join(missingVariables, ", "))
	}

	return []byte(tmp), nil
}

// inspectTargetNamespace identifies the name of the namespace object contained in the components YAML, if any.
// In case more than one Namespace object is identified, an error is returned.
func inspectTargetNamespace(objs []unstructured.Unstructured) (string, error) {
	namespace := ""
	for _, o := range objs {
		// if the object has Kind Namespace
		if o.GetKind() == namespaceKind {
			// grab the name (or error if there is more than one Namespace object)
			if namespace != "" {
				return "", errors.New("Invalid manifest. There should be no more than one resource with Kind Namespace in the provider components yaml")
			}
			namespace = o.GetName()
		}
	}
	return namespace, nil
}

// addNamespaceIfMissing adda a Namespace object if missing (this ensure the targetNamespace will be created)
func addNamespaceIfMissing(objs []unstructured.Unstructured, targetNamespace string) []unstructured.Unstructured {
	namespaceObjectFound := false
	for _, o := range objs {
		// if the object has Kind Namespace, fix the namespace name
		if o.GetKind() == namespaceKind {
			namespaceObjectFound = true
		}
	}

	// if there isn't an object with Kind Namespace, add it
	if !namespaceObjectFound {
		objs = append(objs, unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": namespaceKind,
				"metadata": map[string]interface{}{
					"name": targetNamespace,
				},
			},
		})
	}

	return objs
}

// fixTargetNamespace ensures all the provider components are deployed in the target namespace (apply only to namespaced objects).
func fixTargetNamespace(objs []unstructured.Unstructured, targetNamespace string) []unstructured.Unstructured {
	for _, o := range objs {
		// if the object has Kind Namespace, fix the namespace name
		if o.GetKind() == namespaceKind {
			o.SetName(targetNamespace)
		}

		// if the object is namespaced, set the namespace name
		if util.IsResourceNamespaced(o.GetKind()) {
			o.SetNamespace(targetNamespace)
		}
	}

	return objs
}

// fixRBAC ensures all the ClusterRole and ClusterRoleBinding have the name prefixed with the namespace name and that
// all the clusterRole/clusterRoleBinding namespaced subjects refers to targetNamespace
func fixRBAC(objs []unstructured.Unstructured, targetNamespace string) ([]unstructured.Unstructured, error) {
	renamedClusterRoles := map[string]string{}
	for _, o := range objs {
		// if the object has Kind ClusterRole
		if o.GetKind() == clusterRoleKind {
			// assign a namespaced name
			currentName := o.GetName()
			newName := fmt.Sprintf("%s-%s", targetNamespace, currentName)
			o.SetName(newName)

			renamedClusterRoles[currentName] = newName
		}
	}

	for i := range objs {
		o := objs[i]
		switch o.GetKind() {
		case clusterRoleBindingKind: // if the object has Kind ClusterRoleBinding
			// Convert Unstructured into a typed object
			b := &rbacv1.ClusterRoleBinding{}
			if err := scheme.Scheme.Convert(&o, b, nil); err != nil {
				return nil, err
			}

			// assign a namespaced name
			b.Name = fmt.Sprintf("%s-%s", targetNamespace, b.Name)

			// ensure that namespaced subjects refers to targetNamespace
			for k := range b.Subjects {
				if b.Subjects[k].Namespace != "" {
					b.Subjects[k].Namespace = targetNamespace
				}
			}

			// if the referenced ClusterRole was renamed, change the RoleRef
			if newName, ok := renamedClusterRoles[b.RoleRef.Name]; ok {
				b.RoleRef.Name = newName
			}

			// Convert ClusterRoleBinding back to Unstructured
			if err := scheme.Scheme.Convert(b, &o, nil); err != nil {
				return nil, err
			}
			objs[i] = o

		case roleBindingKind: // if the object has Kind RoleBinding
			// Convert Unstructured into a typed object
			b := &rbacv1.RoleBinding{}
			if err := scheme.Scheme.Convert(&o, b, nil); err != nil {
				return nil, err
			}

			// ensure that namespaced subjects refers to targetNamespace
			for k := range b.Subjects {
				if b.Subjects[k].Namespace != "" {
					b.Subjects[k].Namespace = targetNamespace
				}
			}

			// Convert RoleBinding back to Unstructured
			if err := scheme.Scheme.Convert(b, &o, nil); err != nil {
				return nil, err
			}
			objs[i] = o
		}
	}

	return objs, nil
}

// inspectWatchNamespace inspects the list of components objects for the default watching namespace
// the default watching namespace is the namespace the controller is set for watching in the component yaml read from the repository, if any
func inspectWatchNamespace(objs []unstructured.Unstructured) (string, error) {
	namespace := ""
	// look for resources of kind Deployment
	for i := range objs {
		o := objs[i]
		if o.GetKind() != deploymentKind {
			continue
		}

		// Convert Unstructured into a typed object
		d := &appsv1.Deployment{}
		if err := scheme.Scheme.Convert(&o, d, nil); err != nil {
			return "", err
		}

		// look for a container with name "manager"
		for _, c := range d.Spec.Template.Spec.Containers {
			if c.Name != controllerContainerName {
				continue
			}

			// look for the --namespace command arg
			for _, a := range c.Args {
				if strings.HasPrefix(a, namespaceArgPrefix) {
					n := strings.TrimPrefix(a, namespaceArgPrefix)
					if namespace != "" && n != namespace {
						return "", errors.New("Invalid manifest. All the controllers should watch have the same --namespace command arg in the provider components yaml")
					}
					namespace = n
				}
			}
		}
	}

	return namespace, nil
}

func fixWatchNamespace(objs []unstructured.Unstructured, watchingNamespace string) ([]unstructured.Unstructured, error) {
	// look for resources of kind Deployment
	for i := range objs {
		o := objs[i]
		if o.GetKind() != deploymentKind {
			continue
		}

		// Convert Unstructured into a typed object
		d := &appsv1.Deployment{}
		if err := scheme.Scheme.Convert(&o, d, nil); err != nil {
			return nil, err
		}

		// look for a container with name "manager"
		for j, c := range d.Spec.Template.Spec.Containers {
			if c.Name == controllerContainerName {

				// look for the --namespace command arg
				found := false
				for k, a := range c.Args {
					// if it exist
					if strings.HasPrefix(a, namespaceArgPrefix) {
						found = true

						// replace the command arg with the desired value or delete the arg if the controller should watch for objects in all the namespaces
						if watchingNamespace != "" {
							c.Args[k] = fmt.Sprintf("%s%s", namespaceArgPrefix, watchingNamespace)
							continue
						}
						c.Args = remove(c.Args, k)
					}
				}

				// If it doesn't exist, and the controller should watch for objects in a specific namespace, set the command arg.
				if !found && watchingNamespace != "" {
					c.Args = append(c.Args, fmt.Sprintf("%s%s", namespaceArgPrefix, watchingNamespace))
				}
			}

			d.Spec.Template.Spec.Containers[j] = c
		}

		// Convert Deployment back to Unstructured
		if err := scheme.Scheme.Convert(d, &o, nil); err != nil {
			return nil, err
		}
		objs[i] = o
	}
	return objs, nil
}

func remove(slice []string, i int) []string {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}

// addCommonLabels ensures all the provider components have a consistent set of labels
func addCommonLabels(objs []unstructured.Unstructured, provider config.Provider) []unstructured.Unstructured {
	for _, o := range objs {
		labels := o.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range getCommonLabels(provider) {
			labels[k] = v
		}
		o.SetLabels(labels)
	}

	return objs
}

func getCommonLabels(provider config.Provider) map[string]string {
	return map[string]string{
		clusterctlv1.ClusterctlLabelName: "",
		clusterv1.ProviderLabelName:      provider.ManifestLabel(),
	}
}

// fixSharedLabels ensures all the shared components have an identifying label so next invocation of init, clusterctl delete
// and clusterctl upgrade can act accordingly.
func fixSharedLabels(objs []unstructured.Unstructured) []unstructured.Unstructured {
	for _, o := range objs {
		labels := o.GetLabels()
		labels[clusterctlv1.ClusterctlResourceLifecyleLabelName] = string(clusterctlv1.ResourceLifecycleShared)

		// the capi-webhook-system namespace is shared among many providers, so removing the ProviderLabelName label.
		if o.GetKind() == namespaceKind && o.GetName() == WebhookNamespaceName {
			delete(labels, clusterv1.ProviderLabelName)
		}
		o.SetLabels(labels)
	}

	return objs
}
