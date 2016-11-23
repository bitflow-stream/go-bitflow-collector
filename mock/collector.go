package mock

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
)

const _max_mock_val = 200

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

func NewMockCollector(factory *collector.ValueRingFactory) collector.Collector {
	col := &MockRootCollector{
		factory: factory,
	}
	col.Name = "mock-root"
	return col
}

type MockRootCollector struct {
	collector.AbstractCollector
	factory     *collector.ValueRingFactory
	externalVal int
	val         bitflow.Value
	startOnce   sync.Once
}

func (root *MockRootCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{
		newMockCollector(root, root.factory, 1),
		newMockCollector(root, root.factory, 2),
		newMockCollector(root, root.factory, 3),
	}, nil
}

func (root *MockRootCollector) Update() error {
	root.startOnce.Do(func() {
		millis := time.Millisecond * time.Duration(rand.Intn(500)+100) // 100..500
		log.Printf("Incrementing mock values %.4f times per second", float64(time.Second)/float64(millis))
		go func() {
			for {
				time.Sleep(millis)
				root.externalVal++
				if root.externalVal >= _max_mock_val {
					root.externalVal = 2
				}
			}
		}()
	})
	root.val = bitflow.Value(root.externalVal)
	return nil
}

func (root *MockRootCollector) Metrics() collector.MetricReaderMap {
	return nil
}

func (root *MockRootCollector) Depends() []collector.Collector {
	return nil
}

type MockCollector struct {
	collector.AbstractCollector
	root   *MockRootCollector
	ring   *collector.ValueRing
	factor int
}

func newMockCollector(root *MockRootCollector, factory *collector.ValueRingFactory, factor int) *MockCollector {
	col := &MockCollector{
		root:   root,
		factor: factor,
		ring:   factory.NewValueRing(),
	}
	col.Name = fmt.Sprintf("mock/%v", factor)
	col.Parent = &root.AbstractCollector
	return col
}

func (col *MockCollector) Init() ([]collector.Collector, error) {
	return nil, nil
}

func (col *MockCollector) Update() error {
	col.ring.Add(collector.StoredValue(col.root.val * bitflow.Value(col.factor)))
	return nil
}

func (col *MockCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		fmt.Sprintf("mock/%v", col.factor): col.ring.GetDiff,
	}
}

func (col *MockCollector) Depends() []collector.Collector {
	return []collector.Collector{col.root}
}
