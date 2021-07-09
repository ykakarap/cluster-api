/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha4

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s *KubeadmControlPlaneSpec) setDefaults(namespace string) {
	if s.Replicas == nil {
		replicas := int32(1)
		s.Replicas = &replicas
	}

	if s.MachineTemplate.InfrastructureRef.Namespace == "" {
		s.MachineTemplate.InfrastructureRef.Namespace = namespace
	}

	if !strings.HasPrefix(s.Version, "v") {
		s.Version = "v" + s.Version
	}

	ios1 := intstr.FromInt(1)

	if s.RolloutStrategy == nil {
		s.RolloutStrategy = &RolloutStrategy{}
	}

	// Enforce RollingUpdate strategy and default MaxSurge if not set.
	if s.RolloutStrategy != nil {
		if len(s.RolloutStrategy.Type) == 0 {
			s.RolloutStrategy.Type = RollingUpdateStrategyType
		}
		if s.RolloutStrategy.Type == RollingUpdateStrategyType {
			if s.RolloutStrategy.RollingUpdate == nil {
				s.RolloutStrategy.RollingUpdate = &RollingUpdate{}
			}
			s.RolloutStrategy.RollingUpdate.MaxSurge = intstr.ValueOrDefault(s.RolloutStrategy.RollingUpdate.MaxSurge, ios1)
		}
	}
}
