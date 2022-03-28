/*
Copyright 2021 The Kubernetes Authors.

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

package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api/internal/runtime/catalog"
)

type F interface{}

type HandlerBuilder struct {
	catalog *catalog.Catalog
	svcToF  map[catalog.Hook]F
}

func NewHandlerBuilder() *HandlerBuilder {
	return &HandlerBuilder{
		svcToF: map[catalog.Hook]F{},
	}
}

func (bld *HandlerBuilder) WithCatalog(c *catalog.Catalog) *HandlerBuilder {
	bld.catalog = c
	return bld
}

func (bld *HandlerBuilder) AddService(svc catalog.Hook, f F) *HandlerBuilder {
	bld.svcToF[svc] = f
	return bld
}

func (bld *HandlerBuilder) Build() (http.Handler, error) {
	if bld.catalog == nil {

	}

	r := mux.NewRouter()

	for svc, f := range bld.svcToF {

		gvh, err := bld.catalog.GroupVersionHook(svc)
		if err != nil {
			return nil, err
		}

		in, err := bld.catalog.NewRequest(gvh)
		if err != nil {
			return nil, err
		}

		out, err := bld.catalog.NewResponse(gvh)
		if err != nil {
			return nil, err
		}

		// TODO: add context
		if err := validateF(f, in, out); err != nil {
			return nil, err
		}

		fWrapper := func(w http.ResponseWriter, r *http.Request) {

			reqBody, err := ioutil.ReadAll(r.Body)
			if err != nil {
				// TODO: handle error
			}

			in, err := bld.catalog.NewRequest(gvh)
			if err != nil {
				// TODO: handle error
			}

			if err := json.Unmarshal(reqBody, in); err != nil {
				// TODO: handle error
			}

			out, err := bld.catalog.NewResponse(gvh)
			if err != nil {
				// TODO: handle error
			}

			// TODO: build new context with correlation ID and pass it to the call
			// TODO: context with Cancel to enforce timeout? enforce timeout on caller side? both?

			v := reflect.ValueOf(f)
			ret := v.Call([]reflect.Value{
				reflect.ValueOf(in),
				reflect.ValueOf(out),
			})

			if !ret[0].IsNil() {
				// TODO: handle error
			}

			respBody, err := json.Marshal(out)
			if err != nil {
				// TODO: handle error
			}

			w.WriteHeader(http.StatusOK)
			w.Write(respBody)
		}

		r.HandleFunc(catalog.GVHToPath(gvh, "TODO: name"), fWrapper).Methods("POST")
	}

	return r, nil
}

func validateF(f interface{}, params ...interface{}) error {
	funcType := reflect.TypeOf(f)

	if funcType.NumIn() != len(params) {
		return errors.New("InvocationCausedPanic called with a function and an incorrect number of parameter(s).")
	}

	for paramIndex, paramValue := range params {
		expectedType := funcType.In(paramIndex)
		actualType := reflect.TypeOf(paramValue)

		if actualType != expectedType {
			return errors.Errorf("InvocationCausedPanic called with a mismatched parameter type [parameter #%v: expected %v; got %v].", paramIndex, expectedType, actualType)
		}
	}

	// TODO: check return is error

	return nil
}
