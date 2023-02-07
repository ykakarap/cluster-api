/*
Copyright 2023 The Kubernetes Authors.

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

// Package ssa contains utils related to ssa.
package ssa

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CleanUpManagedFieldsForSSACompatibility deletes the managedField entries on the object that belong to "manager"(Operation=Update)
// if there is no field managed by "managerName".
// It adds an "empty" entry in managedFields of the object if no field is currently managed by "managerName".
func CleanUpManagedFieldsForSSACompatibility(ctx context.Context, obj client.Object, managerName string, c client.Client) error {
	if HasFieldsManagedBy(obj, managerName) {
		return nil
	}

	// Since there is no field managed by "managerName" it means that
	// this object has not been processed after adopting SSA.
	// Here, drop the managed fields that were managed by the "manager" manager and add an empty entry
	// for "managerName".
	// This will ensure that "managerName" will be able to modify the fields that
	// were originally owned by "manager".
	base := obj.DeepCopyObject().(client.Object)

	// Remove managedFieldEntry for manager=manager and operation=update to prevent having two managers holding values.
	originalManagedFields := obj.GetManagedFields()
	managedFields := make([]metav1.ManagedFieldsEntry, 0, len(originalManagedFields))
	for i := range originalManagedFields {
		if originalManagedFields[i].Manager == "manager" &&
			originalManagedFields[i].Operation == metav1.ManagedFieldsOperationUpdate {
			continue
		}
		managedFields = append(managedFields, originalManagedFields[i])
	}

	// Add a seeding managedFieldEntry for SSA executed by the management controller, to prevent SSA to create/infer
	// a default managedFieldEntry when the first SSA is applied.
	// More specifically, if an existing object doesn't have managedFields when applying the first SSA the API server
	// creates an entry with operation=Update (kind of guessing where the object comes from), but this entry ends up
	// acting as a co-ownership and we want to prevent this.
	// NOTE: fieldV1Map cannot be empty, so we add metadata.name which will be cleaned up at the first SSA patch.
	fieldV1Map := map[string]interface{}{
		"f:metadata": map[string]interface{}{
			"f:name": map[string]interface{}{},
		},
	}
	fieldV1, err := json.Marshal(fieldV1Map)
	if err != nil {
		return errors.Wrap(err, "failed to create seeding fieldV1Map for cleaning up legacy managed fields")
	}
	now := metav1.Now()
	managedFields = append(managedFields, metav1.ManagedFieldsEntry{
		Manager:    managerName,
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Time:       &now,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
	})

	obj.SetManagedFields(managedFields)

	return c.Patch(ctx, obj, client.MergeFrom(base))
}

// HasFieldsManagedBy returns true if any of the fields in obj are managed by manager.
func HasFieldsManagedBy(obj client.Object, manager string) bool {
	managedFields := obj.GetManagedFields()
	for _, mf := range managedFields {
		if mf.Manager == manager {
			return true
		}
	}
	return false
}
