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

package v1alpha1

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type invalidResponseObject struct{}

func (o invalidResponseObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (o invalidResponseObject) DeepCopyObject() runtime.Object {
	return o
}

type statusResponseObject struct {
	Status ResponseStatus
}

func (o statusResponseObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (o statusResponseObject) DeepCopyObject() runtime.Object {
	return o
}

func TestIsFailure(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		want    bool
		wantErr bool
	}{
		{
			name: "valid response object (pointer)  with failure status should return true",
			obj: &BeforeClusterCreateResponse{
				Status: ResponseStatusFailure,
			},
			want: true,
		},
		{
			name: "valid response object (struct)  with failure status should return true",
			obj: statusResponseObject{
				Status: ResponseStatusFailure,
			},
			want: true,
		},
		{
			name: "valid response object with success status should return false",
			obj: &BeforeClusterCreateResponse{
				Status: ResponseStatusSuccess,
			},
			want: false,
		},
		{
			name:    "invalid response (pointer) object should error",
			obj:     &invalidResponseObject{},
			wantErr: true,
		},
		{
			name:    "invalid response (struct) object should error",
			obj:     invalidResponseObject{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			res, err := IsFailure(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(Equal(tt.want))
			}
		})
	}
}

func TestSetStatus(t *testing.T) {
	tests := []struct {
		name    string
		obj     *BeforeClusterCreateResponse
		status  ResponseStatus
		wantErr bool
	}{
		{
			name:    "setting the status should succeeded",
			obj:     &BeforeClusterCreateResponse{},
			status:  ResponseStatusSuccess,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := SetStatus(tt.obj, tt.status)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(tt.obj.Status).To(Equal(tt.status))
			}
		})
	}
}

func TestSetRetryAfterSeconds(t *testing.T) {
	g := NewWithT(t)
	obj := &BeforeClusterCreateResponse{}
	g.Expect(SetRetryAfterSeconds(obj, 10)).Should(Succeed())
	g.Expect(obj.RetryAfterSeconds).To(Equal(10))
}
