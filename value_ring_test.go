package collector

import (
	"testing"

	"github.com/antongulenko/golib"
	"github.com/stretchr/testify/suite"
)

type ValueRingTestSuite struct {
	golib.AbstractTestSuite
}

func TestValueRing(t *testing.T) {
	suite.Run(t, new(ValueRingTestSuite))
}

func (suite *ValueRingTestSuite) TestRing() {
	// TODO write tests
}
