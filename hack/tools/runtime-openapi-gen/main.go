package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
)

// TODO: move to tools.
// TODO: use klogs.
// TODO: output flag for json/yaml
// TODO: output path flag

var c = catalog.New()

func init() {
	// TODO: how to make this dynamic (automatically discover all the extensions or use input paths)
	_ = v1alpha1.AddToCatalog(c)
	//_ = v1alpha2.AddToCatalog(c)
	//_ = v1alpha3.AddToCatalog(c)
}

func main() {
	openAPI, err := c.OpenAPI()
	if err != nil {
		panic(fmt.Sprintf("OpenAPI error: %v", err))
	}

	openAPIBytes, err := yaml.Marshal(openAPI)
	if err != nil {
		panic(fmt.Sprintf("MarshalIndent error: %v", err))
	}

	err = os.WriteFile("openapi.yaml", openAPIBytes, 0600)
	if err != nil {
		panic(fmt.Sprintf("WriteFile error: %v", err))
	}

	// Marshal the swagger spec into JSON, then write it out.
	//openAPIBytes, err := json.MarshalIndent(openAPI, " ", " ")
	//if err != nil {
	//	panic(fmt.Sprintf("MarshalIndent error: %v", err))
	//}
	//

	//err = os.WriteFile("openapi.json", openAPIBytes, 0600)
	//if err != nil {
	//	panic(fmt.Sprintf("WriteFile error: %v", err))
	//}
}
