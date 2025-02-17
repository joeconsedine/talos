/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/config"
)

// UdevdTrigger implements the Service interface. It serves as the concrete type with
// the required methods.
type UdevdTrigger struct{}

// ID implements the Service interface.
func (c *UdevdTrigger) ID(config config.Configurator) string {
	return "udevd-trigger"
}

// PreFunc implements the Service interface.
func (c *UdevdTrigger) PreFunc(ctx context.Context, config config.Configurator) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *UdevdTrigger) PostFunc(config config.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *UdevdTrigger) Condition(config config.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *UdevdTrigger) DependsOn(config config.Configurator) []string {
	return []string{"udevd"}
}

// Runner implements the Service interface.
func (c *UdevdTrigger) Runner(config config.Configurator) (runner.Runner, error) {
	// Set the process arguments.
	args := &runner.Args{
		ID: c.ID(config),
		ProcessArgs: []string{
			"/sbin/udevadm",
			"trigger",
		},
	}

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(process.NewRunner(
		config.Debug(),
		args,
		runner.WithEnv(env),
	),
		restart.WithType(restart.Once),
	), nil
}

// APIStartAllowed implements the APIStartableService interface.
func (c *UdevdTrigger) APIStartAllowed(config config.Configurator) bool {
	return true
}

// APIStopAllowed implements the APIStoppableService interface.
func (c *UdevdTrigger) APIStopAllowed(config config.Configurator) bool {
	return true
}

// APIRestartAllowed implements the APIRestartableService interface.
func (c *UdevdTrigger) APIRestartAllowed(config config.Configurator) bool {
	return true
}
