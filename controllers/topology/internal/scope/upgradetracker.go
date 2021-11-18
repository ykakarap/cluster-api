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

package scope

import "k8s.io/apimachinery/pkg/util/sets"

const maxMachineDeploymentUpgradeConcurrency = 1

// UpgradeTracker is a helper to capture the upgrade status and make upgrade decisions.
type UpgradeTracker struct {
	ControlPlane       ControlPlaneUpgradeTracker
	MachineDeployments MachineDeploymentUpgradeTracker
}

// TopologyUpgrading returns true if the control plane or machine deployments are
// upgrading/rollingout. Returns false otherwise.
func (t *UpgradeTracker) TopologyUpgrading() bool {
	return !t.ControlPlane.Stable || !t.MachineDeployments.Stable()
}

type ControlPlaneUpgradeTracker struct {
	// Stable captures the state of control plane.
	// It is false if the control plane is upgrading, scaling or about to upgrade.
	// True, otherwise.
	Stable bool
}

// MachineDeploymentUpgradeTracker holds the current upgrade status and makes upgrade
// decisions for MachineDeployments.
type MachineDeploymentUpgradeTracker struct {
	MachineDeploymentRollingOut      []string
	machineDeploymentsReadyToUpgrade sets.String
}

// NewUpgradeTracker returns an upgrade tracker with empty tracking information.
func NewUpgradeTracker() *UpgradeTracker {
	return &UpgradeTracker{
		MachineDeployments: MachineDeploymentUpgradeTracker{
			machineDeploymentsReadyToUpgrade: sets.NewString(),
		},
	}
}

// Stable returns true of none of the machine deployments are rolling out or
// are about to be upgraded.
// Returns false otherwise.
func (m *MachineDeploymentUpgradeTracker) Stable() bool {
	return len(m.NamesList()) == 0
}

// NamesList returns the list of the machine deployments that are already rolling out
// or are ready to be upgraded.
func (m *MachineDeploymentUpgradeTracker) NamesList() []string {
	return append(m.machineDeploymentsReadyToUpgrade.List(), m.MachineDeploymentRollingOut...)
}

// Insert adds name to the set of MachineDeployments that will be upgraded.
func (m *MachineDeploymentUpgradeTracker) Insert(name string) {
	m.machineDeploymentsReadyToUpgrade.Insert(name)
}

// AllowUpgrade returns true if a MachineDeployment is allowed to upgrade,
// returns false otherwise.
func (m *MachineDeploymentUpgradeTracker) AllowUpgrade() bool {
	return m.machineDeploymentsReadyToUpgrade.Len() < maxMachineDeploymentUpgradeConcurrency
}
