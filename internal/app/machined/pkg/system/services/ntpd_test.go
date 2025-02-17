/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
)

func TestNTPdInterfaces(t *testing.T) {
	assert.Implements(t, (*system.APIStartableService)(nil), new(services.NTPd))
	assert.Implements(t, (*system.APIRestartableService)(nil), new(services.NTPd))
}
