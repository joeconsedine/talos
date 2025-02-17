/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"
	stdlibnet "net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/trustd/internal/reg"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/startup"
)

var configPath *string

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	configPath = flag.String("config", "", "the path to the config")

	flag.Parse()
}

// nolint: gocyclo
func main() {
	var err error

	if err = startup.RandSeed(); err != nil {
		log.Fatalf("startup: %s", err)
	}

	content, err := config.FromFile(*configPath)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	config, err := config.New(content)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	ips, err := net.IPAddrs()
	if err != nil {
		log.Fatal(err)
	}

	for _, san := range config.Machine().Security().CertSANs() {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	var provider tls.CertificateProvider

	provider, err = tls.NewLocalRenewingFileCertificateProvider(config.Machine().Security().CA().Key, config.Machine().Security().CA().Crt, hostname, ips)
	if err != nil {
		log.Fatalln("failed to create local certificate provider:", err)
	}

	ca, err := provider.GetCA()
	if err != nil {
		log.Fatal(err)
	}

	tlsConfig, err := tls.New(
		tls.WithClientAuthType(tls.ServerOnly),
		tls.WithCACertPEM(ca),
		tls.WithCertificateProvider(provider),
	)
	if err != nil {
		log.Fatalf("failed to create TLS config: %v", err)
	}

	creds := basic.NewTokenCredentials(config.Machine().Security().Token())

	err = factory.ListenAndServe(
		&reg.Registrator{Config: config},
		factory.Port(constants.TrustdPort),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(tlsConfig),
			),
			grpc.UnaryInterceptor(creds.UnaryInterceptor()),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
