/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	timeapi "github.com/talos-systems/talos/api/time"
	"github.com/talos-systems/talos/internal/app/ntpd/pkg/ntp"
	"github.com/talos-systems/talos/pkg/grpc/factory"
)

type NtpdSuite struct {
	suite.Suite
}

func TestNtpdSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NtpdSuite))
}

func (suite *NtpdSuite) TestTime() {
	testServer := "time.cloudflare.com"
	// Create ntp client
	n, err := ntp.NewNTPClient(ntp.WithServer(testServer))
	suite.Assert().NoError(err)

	// Create gRPC server
	api := NewRegistrator(n)
	server := factory.NewServer(api)
	listener, err := fakeNtpdRPC()
	suite.Assert().NoError(err)

	defer server.Stop()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(fmt.Sprintf("%s://%s", "unix", listener.Addr().String()), grpc.WithInsecure())
	suite.Assert().NoError(err)

	nClient := timeapi.NewTimeClient(conn)
	resp, err := nClient.Time(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Equal(resp.Server, testServer)
}

func (suite *NtpdSuite) TestTimeCheck() {
	testServer := "time.cloudflare.com"
	// Create ntp client with bogus server
	// so we can check that we explicitly check the time of the
	// specified server ( testserver )
	n, err := ntp.NewNTPClient(ntp.WithServer("127.0.0.1"))
	suite.Assert().NoError(err)

	// Create gRPC server
	api := NewRegistrator(n)
	server := factory.NewServer(api)
	listener, err := fakeNtpdRPC()
	suite.Assert().NoError(err)

	defer server.Stop()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(fmt.Sprintf("%s://%s", "unix", listener.Addr().String()), grpc.WithInsecure())
	suite.Assert().NoError(err)

	nClient := timeapi.NewTimeClient(conn)
	resp, err := nClient.TimeCheck(context.Background(), &timeapi.TimeRequest{Server: testServer})
	suite.Assert().NoError(err)
	suite.Assert().Equal(resp.Server, testServer)
}

func fakeNtpdRPC() (net.Listener, error) {
	tmpfile, err := ioutil.TempFile("", "ntpd")
	if err != nil {
		return nil, err
	}

	return factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(tmpfile.Name()),
	)
}
