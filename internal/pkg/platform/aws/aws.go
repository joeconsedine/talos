/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package aws

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/fullsailor/pkcs7"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
)

const (
	// AWSExternalIPEndpoint displays all external addresses associated with the instance
	AWSExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"
	// AWSHostnameEndpoint is the local EC2 endpoint for the hostname.
	AWSHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"
	// AWSPKCS7Endpoint is the local EC2 endpoint for the PKCS7 signature.
	AWSPKCS7Endpoint = "http://169.254.169.254/latest/dynamic/instance-identity/pkcs7"
	// AWSPublicCertificate is the AWS public certificate for the regions
	// provided by an AWS account.
	AWSPublicCertificate = `-----BEGIN CERTIFICATE-----
MIIC7TCCAq0CCQCWukjZ5V4aZzAJBgcqhkjOOAQDMFwxCzAJBgNVBAYTAlVTMRkw
FwYDVQQIExBXYXNoaW5ndG9uIFN0YXRlMRAwDgYDVQQHEwdTZWF0dGxlMSAwHgYD
VQQKExdBbWF6b24gV2ViIFNlcnZpY2VzIExMQzAeFw0xMjAxMDUxMjU2MTJaFw0z
ODAxMDUxMjU2MTJaMFwxCzAJBgNVBAYTAlVTMRkwFwYDVQQIExBXYXNoaW5ndG9u
IFN0YXRlMRAwDgYDVQQHEwdTZWF0dGxlMSAwHgYDVQQKExdBbWF6b24gV2ViIFNl
cnZpY2VzIExMQzCCAbcwggEsBgcqhkjOOAQBMIIBHwKBgQCjkvcS2bb1VQ4yt/5e
ih5OO6kK/n1Lzllr7D8ZwtQP8fOEpp5E2ng+D6Ud1Z1gYipr58Kj3nssSNpI6bX3
VyIQzK7wLclnd/YozqNNmgIyZecN7EglK9ITHJLP+x8FtUpt3QbyYXJdmVMegN6P
hviYt5JH/nYl4hh3Pa1HJdskgQIVALVJ3ER11+Ko4tP6nwvHwh6+ERYRAoGBAI1j
k+tkqMVHuAFcvAGKocTgsjJem6/5qomzJuKDmbJNu9Qxw3rAotXau8Qe+MBcJl/U
hhy1KHVpCGl9fueQ2s6IL0CaO/buycU1CiYQk40KNHCcHfNiZbdlx1E9rpUp7bnF
lRa2v1ntMX3caRVDdbtPEWmdxSCYsYFDk4mZrOLBA4GEAAKBgEbmeve5f8LIE/Gf
MNmP9CM5eovQOGx5ho8WqD+aTebs+k2tn92BBPqeZqpWRa5P/+jrdKml1qx4llHW
MXrs3IgIb6+hUIB+S8dz8/mmO0bpr76RoZVCXYab2CZedFut7qc3WUH9+EUAH5mw
vSeDCOUMYQR7R9LINYwouHIziqQYMAkGByqGSM44BAMDLwAwLAIUWXBlk40xTwSw
7HX32MxXYruse9ACFBNGmdX2ZBrVNGrN9N2f6ROk0k9K
-----END CERTIFICATE-----`
	// AWSUserDataEndpoint is the local EC2 endpoint for the config.
	AWSUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

// AWS is the concrete type that implements the platform.Platform interface.
type AWS struct{}

// IsEC2 uses the EC2 PKCS7 signature to verify the instance by validating it
// against the appropriate AWS public certificate. See
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
func IsEC2() (b bool) {
	resp, err := http.Get(AWSPKCS7Endpoint)
	if err != nil {
		return
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("failed to download PKCS7 signature: %d\n", resp.StatusCode)
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	data = append([]byte("-----BEGIN PKCS7-----\n"), data...)
	data = append(data, []byte("\n-----END PKCS7-----\n")...)

	pemBlock, _ := pem.Decode(data)
	if pemBlock == nil {
		log.Println("failed to decode PEM block")
		return
	}

	p7, err := pkcs7.Parse(pemBlock.Bytes)
	if err != nil {
		log.Printf("failed to parse PKCS7 signature: %v\n", err)
		return
	}

	pemBlock, _ = pem.Decode([]byte(AWSPublicCertificate))
	if pemBlock == nil {
		log.Println("failed to decode PEM block")
		return
	}

	certificate, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		log.Printf("failed to parse X509 certificate: %v\n", err)
		return
	}

	p7.Certificates = []*x509.Certificate{certificate}

	err = p7.Verify()
	if err != nil {
		log.Printf("failed to verify PKCS7 signature: %v", err)
		return
	}

	b = true

	return b
}

// Name implements the platform.Platform interface.
func (a *AWS) Name() string {
	return "AWS"
}

// Configuration implements the platform.Platform interface.
func (a *AWS) Configuration() ([]byte, error) {
	return config.Download(AWSUserDataEndpoint)
}

// Mode implements the platform.Platform interface.
func (a *AWS) Mode() runtime.Mode {
	return runtime.Cloud
}

// Hostname gets the hostname from the AWS metadata endpoint.
func (a *AWS) Hostname() (hostname []byte, err error) {
	resp, err := http.Get(AWSHostnameEndpoint)
	if err != nil {
		return
	}
	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return hostname, fmt.Errorf("failed to fetch hostname from metadata service: %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}

// ExternalIPs provides any external addresses assigned to the instance
func (a *AWS) ExternalIPs() (addrs []net.IP, err error) {
	var (
		body []byte
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequest("GET", AWSExternalIPEndpoint, nil); err != nil {
		return
	}

	client := &http.Client{}
	if resp, err = client.Do(req); err != nil {
		return
	}

	// nolint: errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return addrs, fmt.Errorf("failed to retrieve external addresses for instance")
	}

	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	addrs = append(addrs, net.ParseIP(string(body)))

	return addrs, err
}
