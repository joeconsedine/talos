/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package tls

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/gen"
)

type renewingLocalCertificateProvider struct {
	embeddableCertificateProvider

	caKey []byte
	caCrt []byte

	generator *gen.LocalGenerator
}

// NewLocalRenewingFileCertificateProvider returns a new CertificateProvider
// which manages and updates its certificates using a local key.
func NewLocalRenewingFileCertificateProvider(caKey, caCrt []byte, hostname string, ips []net.IP) (CertificateProvider, error) {
	g, err := gen.NewLocalGenerator(caKey, caCrt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TLS generator")
	}

	provider := &renewingLocalCertificateProvider{
		caKey:     caKey,
		caCrt:     caCrt,
		generator: g,
	}

	provider.embeddableCertificateProvider = embeddableCertificateProvider{
		hostname:   hostname,
		ips:        ips,
		updateFunc: provider.update,
	}

	var (
		ca   []byte
		cert tls.Certificate
	)

	if ca, cert, err = provider.updateFunc(); err != nil {
		return nil, errors.Wrap(err, "failed to create initial certificate")
	}

	if err = provider.UpdateCertificates(ca, &cert); err != nil {
		return nil, err
	}

	// nolint: errcheck
	go provider.manageUpdates(context.Background())

	return provider, nil
}

// nolint: dupl
func (p *renewingLocalCertificateProvider) update() (ca []byte, cert tls.Certificate, err error) {
	var (
		crt      []byte
		csr      *x509.CertificateSigningRequest
		identity *x509.PEMEncodedCertificateAndKey
	)

	csr, identity, err = x509.NewCSRAndIdentity(p.hostname, p.ips)
	if err != nil {
		return nil, cert, err
	}

	if ca, crt, err = p.generator.Identity(csr); err != nil {
		return nil, cert, errors.Wrap(err, "failed to generate identity")
	}

	identity.Crt = crt

	cert, err = tls.X509KeyPair(identity.Crt, identity.Key)
	if err != nil {
		return nil, cert, errors.Wrap(err, "failed to parse cert and key into a TLS Certificate")
	}

	return ca, cert, nil
}
