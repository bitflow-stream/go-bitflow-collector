package mock

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	log "github.com/sirupsen/logrus"
)

const _max_mock_val = 200

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

func NewMockCollector(factory *collector.ValueRingFactory) collector.Collector {
	return &RootCollector{
		AbstractCollector: collector.RootCollector("mock"),
		factory:           factory,
	}
}

type RootCollector struct {
	collector.AbstractCollector
	factory     *collector.ValueRingFactory
	externalVal int
	val         bitflow.Value
	startOnce   sync.Once
}

func (root *RootCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{
		newMockCollector(root, root.factory, 1),
		newMockCollector(root, root.factory, 2),
		newMockCollector(root, root.factory, 3),
	}, nil
}

func (root *RootCollector) Update() error {
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

func (root *RootCollector) Metrics() collector.MetricReaderMap {
	return nil
}

func (root *RootCollector) Depends() []collector.Collector {
	return nil
}

type Collector struct {
	collector.AbstractCollector
	root   *RootCollector
	ring   *collector.ValueRing
	factor int
}

func newMockCollector(root *RootCollector, factory *collector.ValueRingFactory, factor int) *Collector {
	return &Collector{
		AbstractCollector: root.Child(strconv.Itoa(factor)),
		root:              root,
		factor:            factor,
		ring:              factory.NewValueRing(),
	}
}

func (col *Collector) Init() ([]collector.Collector, error) {
	return nil, nil
}

func (col *Collector) Update() error {
	col.ring.Add(collector.StoredValue(col.root.val * bitflow.Value(col.factor)))
	return nil
}

func (col *Collector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		fmt.Sprintf("mock/%v", col.factor): col.ring.GetDiff,
	}
}

func (col *Collector) Depends() []collector.Collector {
	return []collector.Collector{col.root}
}
