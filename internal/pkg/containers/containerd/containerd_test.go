/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containerd_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/containerd/containerd"
	containerdcntrs "github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	containerdrunner "github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	ctrd "github.com/talos-systems/talos/internal/pkg/containers/containerd"
	"github.com/talos-systems/talos/pkg/constants"
)

const (
	busyboxImage       = "docker.io/library/busybox:1.30.1"
	busyboxImageDigest = "sha256:4b6ad3a68d34da29bf7c8ccb5d355ba8b4babcad1f99798204e7abb43e54ee3d"
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
}

// nolint: maligned
type ContainerdSuite struct {
	suite.Suite

	tmpDir string

	containerdNamespace string
	containerdRunner    runner.Runner
	containerdWg        sync.WaitGroup
	containerdAddress   string

	client *containerd.Client
	image  containerd.Image

	containerRunners []runner.Runner
	containersWg     sync.WaitGroup
}

// WithAnnotations appends or replaces the annotations on the spec with the
// provided annotations
//
// TODO: taken from containerd/oci > 1.2.6
func WithAnnotations(annotations map[string]string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containerdcntrs.Container, s *oci.Spec) error {
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			s.Annotations[k] = v
		}
		return nil
	}
}

// nolint: dupl
func (suite *ContainerdSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	stateDir, rootDir := filepath.Join(suite.tmpDir, "state"), filepath.Join(suite.tmpDir, "root")
	suite.Require().NoError(os.Mkdir(stateDir, 0777))
	suite.Require().NoError(os.Mkdir(rootDir, 0777))

	suite.containerdAddress = filepath.Join(suite.tmpDir, "run.sock")

	args := &runner.Args{
		ID: "containerd",
		ProcessArgs: []string{
			"/bin/containerd",
			"--address", suite.containerdAddress,
			"--state", stateDir,
			"--root", rootDir,
		},
	}

	suite.containerdRunner = process.NewRunner(
		false,
		args,
		runner.WithLogPath(suite.tmpDir),
		runner.WithEnv([]string{"PATH=/bin:" + constants.PATH}),
	)
	suite.Require().NoError(suite.containerdRunner.Open(context.Background()))
	suite.containerdWg.Add(1)

	go func() {
		defer suite.containerdWg.Done()
		defer func() { suite.Require().NoError(suite.containerdRunner.Close()) }()
		suite.Require().NoError(suite.containerdRunner.Run(MockEventSink))
	}()

	suite.client, err = containerd.New(suite.containerdAddress)
	suite.Require().NoError(err)

	suite.containerdNamespace = fmt.Sprintf("talostest%d%d", rand.Int63(), time.Now().Unix())
	ctx := namespaces.WithNamespace(context.Background(), suite.containerdNamespace)

	suite.image, err = suite.client.Pull(ctx, busyboxImage, containerd.WithPullUnpack)
	suite.Require().NoError(err)
}

func (suite *ContainerdSuite) TearDownSuite() {
	suite.Require().NoError(suite.client.Close())

	suite.Require().NoError(suite.containerdRunner.Stop())
	suite.containerdWg.Wait()

	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *ContainerdSuite) SetupTest() {
	suite.containerRunners = nil
}

func (suite *ContainerdSuite) run(runners ...runner.Runner) {
	runningCh := make(chan struct{}, len(runners))

	for _, r := range runners {
		suite.Require().NoError(r.Open(context.Background()))

		suite.containerRunners = append(suite.containerRunners, r)

		suite.containersWg.Add(1)

		go func(r runner.Runner) {
			runningSink := func(state events.ServiceState, message string, args ...interface{}) {
				if state == events.StateRunning {
					runningCh <- struct{}{}
				}
			}

			defer suite.containersWg.Done()
			suite.Assert().NoError(r.Run(runningSink))
		}(r)
	}

	// wait for the containers to be started actually
	for range runners {
		<-runningCh
	}
}

func (suite *ContainerdSuite) TearDownTest() {
	for _, r := range suite.containerRunners {
		suite.Assert().NoError(r.Stop())
	}

	suite.containersWg.Wait()

	for _, r := range suite.containerRunners {
		suite.Assert().NoError(r.Close())
	}
}

func (suite *ContainerdSuite) runK8sContainers() {
	suite.run(containerdrunner.NewRunner(false, &runner.Args{
		ID:          "test1",
		ProcessArgs: []string{"/bin/sh", "-c", "sleep 3600"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerOpts(containerd.WithContainerLabels(map[string]string{
			"io.kubernetes.pod.name":      "fun",
			"io.kubernetes.pod.namespace": "ns1",
		})),
		runner.WithOCISpecOpts(WithAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-log-directory": "sandbox",
			"io.kubernetes.cri.sandbox-id":            "c888d69b73b5b444c2b0bd70da28c3da102b0aeb327f3a297626e2558def327f",
		})),
		runner.WithContainerdAddress(suite.containerdAddress),
	), containerdrunner.NewRunner(false, &runner.Args{
		ID:          "test2",
		ProcessArgs: []string{"/bin/sh", "-c", "sleep 3600"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerOpts(containerd.WithContainerLabels(map[string]string{
			"io.kubernetes.pod.name":       "fun",
			"io.kubernetes.pod.namespace":  "ns1",
			"io.kubernetes.container.name": "run",
		})),
		runner.WithOCISpecOpts(WithAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-id": "c888d69b73b5b444c2b0bd70da28c3da102b0aeb327f3a297626e2558def327f",
		})),
		runner.WithContainerdAddress(suite.containerdAddress),
	))
}

func (suite *ContainerdSuite) TestPodsNonK8s() {
	suite.run(containerdrunner.NewRunner(false, &runner.Args{
		ID:          "test",
		ProcessArgs: []string{"/bin/sh", "-c", "sleep 3600"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	))

	i, err := ctrd.NewInspector(context.Background(), suite.containerdNamespace, ctrd.WithContainerdAddress(suite.containerdAddress))
	suite.Assert().NoError(err)

	pods, err := i.Pods()
	suite.Require().NoError(err)
	suite.Require().Len(pods, 1)
	suite.Assert().Equal("test", pods[0].Name)
	suite.Assert().Equal("", pods[0].Sandbox)
	suite.Require().Len(pods[0].Containers, 1)
	suite.Assert().Equal("test", pods[0].Containers[0].Display)
	suite.Assert().Equal("test", pods[0].Containers[0].Name)
	suite.Assert().Equal("test", pods[0].Containers[0].ID)
	suite.Assert().Equal(busyboxImage, pods[0].Containers[0].Image)
	suite.Assert().Equal("RUNNING", pods[0].Containers[0].Status)
	suite.Assert().NotNil(pods[0].Containers[0].Metrics)

	suite.Assert().NoError(i.Close())
}

func (suite *ContainerdSuite) TestPodsK8s() {
	suite.runK8sContainers()

	i, err := ctrd.NewInspector(context.Background(), suite.containerdNamespace, ctrd.WithContainerdAddress(suite.containerdAddress))
	suite.Assert().NoError(err)

	pods, err := i.Pods()
	suite.Require().NoError(err)
	suite.Require().Len(pods, 1)
	suite.Assert().Equal("ns1/fun", pods[0].Name)
	suite.Assert().Equal("sandbox", pods[0].Sandbox)
	suite.Require().Len(pods[0].Containers, 2)

	suite.Assert().Equal("ns1/fun", pods[0].Containers[0].Display)
	suite.Assert().Equal("test1", pods[0].Containers[0].Name)
	suite.Assert().Equal("test1", pods[0].Containers[0].ID)
	suite.Assert().Equal("sandbox", pods[0].Containers[0].Sandbox)
	suite.Assert().Equal("0", pods[0].Containers[0].RestartCount)
	suite.Assert().Equal("", pods[0].Containers[0].GetLogFile())
	suite.Assert().Equal(busyboxImage, pods[0].Containers[0].Image)
	suite.Assert().Equal("RUNNING", pods[0].Containers[0].Status)
	suite.Assert().NotNil(pods[0].Containers[0].Metrics)

	suite.Assert().Equal("ns1/fun:run", pods[0].Containers[1].Display)
	suite.Assert().Equal("run", pods[0].Containers[1].Name)
	suite.Assert().Equal("test2", pods[0].Containers[1].ID)
	suite.Assert().Equal("sandbox", pods[0].Containers[1].Sandbox)
	suite.Assert().Equal("sandbox/run/0.log", pods[0].Containers[1].GetLogFile())
	suite.Assert().Equal(busyboxImage, pods[0].Containers[1].Image)
	suite.Assert().Equal("RUNNING", pods[0].Containers[1].Status)
	suite.Assert().NotNil(pods[0].Containers[1].Metrics)

	suite.Assert().NoError(i.Close())
}

func (suite *ContainerdSuite) TestContainerNonK8s() {
	suite.run(containerdrunner.NewRunner(false, &runner.Args{
		ID:          "shelltest",
		ProcessArgs: []string{"/bin/sh", "-c", "sleep 3600"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	))

	i, err := ctrd.NewInspector(context.Background(), suite.containerdNamespace, ctrd.WithContainerdAddress(suite.containerdAddress))
	suite.Assert().NoError(err)

	cntr, err := i.Container("shelltest")
	suite.Require().NoError(err)
	suite.Require().NotNil(cntr)
	suite.Assert().Equal("shelltest", cntr.Name)
	suite.Assert().Equal("shelltest", cntr.Display)
	suite.Assert().Equal("shelltest", cntr.ID)
	suite.Assert().Equal(busyboxImageDigest, cntr.Image) // image is not resolved
	suite.Assert().Equal("RUNNING", cntr.Status)

	cntr, err = i.Container("nosuchcontainer")
	suite.Require().NoError(err)
	suite.Require().Nil(cntr)

	suite.Assert().NoError(i.Close())
}

func (suite *ContainerdSuite) TestContainerK8s() {
	suite.runK8sContainers()

	i, err := ctrd.NewInspector(context.Background(), suite.containerdNamespace, ctrd.WithContainerdAddress(suite.containerdAddress))
	suite.Assert().NoError(err)

	cntr, err := i.Container("ns1/fun")
	suite.Require().NoError(err)
	suite.Require().NotNil(cntr)
	suite.Assert().Equal("test1", cntr.Name)
	suite.Assert().Equal("ns1/fun", cntr.Display)
	suite.Assert().Equal("test1", cntr.ID)
	suite.Assert().Equal("sandbox", cntr.Sandbox)
	suite.Assert().Equal("", cntr.GetLogFile())
	suite.Assert().Equal(busyboxImageDigest, cntr.Image) // image is not resolved
	suite.Assert().Equal("RUNNING", cntr.Status)

	cntr, err = i.Container("ns1/fun:run")
	suite.Require().NoError(err)
	suite.Require().NotNil(cntr)
	suite.Assert().Equal("run", cntr.Name)
	suite.Assert().Equal("ns1/fun:run", cntr.Display)
	suite.Assert().Equal("test2", cntr.ID)
	suite.Assert().Equal("sandbox", cntr.Sandbox)
	suite.Assert().Equal("sandbox/run/0.log", cntr.GetLogFile())
	suite.Assert().Equal(busyboxImageDigest, cntr.Image) // image is not resolved
	suite.Assert().Equal("RUNNING", cntr.Status)

	cntr, err = i.Container("ns2/fun:run")
	suite.Require().NoError(err)
	suite.Require().Nil(cntr)

	cntr, err = i.Container("ns1/run:run")
	suite.Require().NoError(err)
	suite.Require().Nil(cntr)

	cntr, err = i.Container("ns1/fun:go")
	suite.Require().NoError(err)
	suite.Require().Nil(cntr)

	suite.Assert().NoError(i.Close())
}

func TestContainerdSuite(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("can't run the test as non-root")
	}

	_, err := os.Stat("/bin/containerd")
	if err != nil {
		t.Skip("containerd binary is not available, skipping the test")
	}

	suite.Run(t, new(ContainerdSuite))
}
