package mock

import (
	"sync"
	"time"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
)

const _max_mock_val = 15

func RegisterMockCollector(factory *collector.ValueRingFactory) {
	collector.RegisterCollector(&MockCollector{
		ring: factory.NewValueRing(),
	})
}

type MockCollector struct {
	collector.AbstractCollector
	val       bitflow.Value
	ring      *collector.ValueRing
	startOnce sync.Once
}

func (col *MockCollector) Init() error {
	col.Reset(col)
	col.Readers = map[string]collector.MetricReader{
		"mock": col.ring.GetDiff,
	}
	col.startOnce.Do(func() {
		go func() {
			for {
				time.Sleep(333 * time.Millisecond)
				col.val++
				if col.val >= _max_mock_val {
					col.val = 2
				}
			}
		}()
	})
	return nil
}

func (col *MockCollector) Update() error {
	col.ring.Add(collector.StoredValue(col.val))
	col.UpdateMetrics()
	return nil
}
