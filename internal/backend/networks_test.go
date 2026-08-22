package backend

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const networksTestTimeout = 5 * time.Second

type networksSuite struct {
	suite.Suite
	client *Client
}

func (s *networksSuite) SetupTest() {
	s.client = NewClientWithBinary(fakeBinary(s.T()))
}

func (s *networksSuite) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), networksTestTimeout)
}

func (s *networksSuite) TestListNetworksReturnsTheDefaultNetwork() {
	ctx, cancel := s.context()
	defer cancel()

	got, err := s.client.ListNetworks(ctx)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal("default", got[0].Name)
	s.Equal("nat", got[0].Mode)
}

func (s *networksSuite) TestListNetworksMapsGatewayAndSubnetFromStatus() {
	ctx, cancel := s.context()
	defer cancel()

	got, err := s.client.ListNetworks(ctx)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal("192.168.64.1", got[0].Gateway)
	s.Equal("192.168.64.0/24", got[0].Subnet)
	s.False(got[0].Created.IsZero())
}

func (s *networksSuite) TestNetworkInspectReturnsTheNamedNetwork() {
	ctx, cancel := s.context()
	defer cancel()

	got, err := s.client.NetworkInspect(ctx, "default")

	s.Require().NoError(err)
	s.Equal("default", got.Name)
	s.Equal("192.168.64.1", got.Gateway)
}

func (s *networksSuite) TestNetworkInspectOnMissingNetworkReturnsErrNetworkNotFound() {
	ctx, cancel := s.context()
	defer cancel()

	_, err := s.client.NetworkInspect(ctx, "ghost")

	s.Require().ErrorIs(err, errNetworkNotFound)
}

func TestNetworksSuite(t *testing.T) {
	suite.Run(t, new(networksSuite))
}
