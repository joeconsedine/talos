/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package address

import (
	"context"
	"net"
	"time"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/config/machine"
)

// Static implements the Addressing interface
type Static struct {
	Device *machine.Device
	NetIf  *net.Interface
}

// Discover doesnt do anything in the static configuration since all
// the necessary configuration data is supplied via config.
func (s *Static) Discover(ctx context.Context) error {
	return nil
}

// Name returns back the name of the address method.
func (s *Static) Name() string {
	return "static"
}

// Address returns the IP address
func (s *Static) Address() *net.IPNet {
	// nolint: errcheck
	ip, ipn, _ := net.ParseCIDR(s.Device.CIDR)
	ipn.IP = ip

	return ipn
}

// Mask returns the netmask.
func (s *Static) Mask() net.IPMask {
	// nolint: errcheck
	_, ipnet, _ := net.ParseCIDR(s.Device.CIDR)
	return ipnet.Mask
}

// MTU returns the specified MTU.
func (s *Static) MTU() uint32 {
	mtu := uint32(s.Device.MTU)
	if mtu == 0 {
		mtu = uint32(s.NetIf.MTU)
	}

	return mtu
}

// TTL returns the address lifetime. Since this is static, there is
// no TTL (0).
func (s *Static) TTL() time.Duration {
	return 0
}

// Family qualifies the address as ipv4 or ipv6
func (s *Static) Family() int {
	if s.Address().IP.To4() != nil {
		return unix.AF_INET
	}

	return unix.AF_INET6
}

// Scope sets the address scope
func (s *Static) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Routes aggregates the specified routes for a given device configuration
// TODO: do we need to be explicit on route vs gateway?
func (s *Static) Routes() (routes []*Route) {
	for _, route := range s.Device.Routes {
		// nolint: errcheck
		_, ipnet, _ := net.ParseCIDR(route.Network)

		routes = append(routes, &Route{Dest: ipnet, Router: net.ParseIP(route.Gateway)})
	}

	return routes
}

// Resolvers returns the DNS resolvers
// TODO: Currently we dont support specifying resolvers via config
func (s *Static) Resolvers() []net.IP {
	// TODO: Think about how we want to expose this via config
	return []net.IP{}
}

// Hostname returns the hostname
// TODO: Should we put kernel.get(hostname param) here?
func (s *Static) Hostname() string {
	return ""
}

// Link returns the underlying net.Interface that this address
// method is configured for
func (s Static) Link() *net.Interface {
	return s.NetIf
}

// Valid denotes if this address method should be used.
func (s *Static) Valid() bool {
	return true
}
