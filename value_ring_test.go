package collector

import (
	"testing"

	"github.com/antongulenko/golib"
)

type ValueRingTestSuite struct {
	golib.AbstractTestSuite
}

func TestValueRing(t *testing.T) {
	new(ValueRingTestSuite).Run(t)
}

func (suite *ValueRingTestSuite) TestRing() {
	// TODO write tests
}
