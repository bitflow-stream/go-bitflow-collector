package collector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ValueRingTestSuite struct {
	t *testing.T
	*require.Assertions
}

func (suite *ValueRingTestSuite) T() *testing.T {
	return suite.t
}

func (suite *ValueRingTestSuite) SetT(t *testing.T) {
	suite.t = t
	suite.Assertions = require.New(t)
}
func TestMarshallerTestSuite(t *testing.T) {
	suite.Run(t, new(ValueRingTestSuite))
}

func (suite *ValueRingTestSuite) testRing() {
	// TODO write tests
}
