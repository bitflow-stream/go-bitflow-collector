package psutil

import (
	"fmt"

	"github.com/bitflow-stream/go-bitflow"
	"github.com/bitflow-stream/go-bitflow-collector"
	psnet "github.com/shirou/gopsutil/net"
)

// TODO missing: metrics about individual NICs

var absoluteNetProtoValues = map[string]bool{
	// These values will not be aggregated through ValueRing
	"NoPorts":      true, // udp, udplite
	"CurrEstab":    true, // tcp
	"MaxConn":      true,
	"RtpAlgorithm": true,
	"RtoMax":       true,
	"RtpMin":       true,
	"DefaultTTL":   true, // ip
	"Forwarding":   true,
}

type NetProtoCollector struct {
	collector.AbstractCollector
	factory *collector.ValueRingFactory

	protocols    map[string]psnet.ProtoCountersStat
	protoReaders []*protoStatReader
}

func newNetProtoCollector(root *RootCollector) *NetProtoCollector {
	return &NetProtoCollector{
		AbstractCollector: root.Child("net-proto"),
		factory:           root.Factory,
	}
}

func (col *NetProtoCollector) Init() ([]collector.Collector, error) {
	col.protocols = make(map[string]psnet.ProtoCountersStat)
	col.protoReaders = nil

	if err := col.update(false); err != nil {
		return nil, err
	}
	for proto, counters := range col.protocols {
		for statName := range counters.Stats {
			var ring *collector.ValueRing
			if !absoluteNetProtoValues[statName] {
				ring = col.factory.NewValueRing()
			}
			protoReader := &protoStatReader{
				col:      col,
				protocol: proto,
				field:    statName,
				ring:     ring,
			}
			col.protoReaders = append(col.protoReaders, protoReader)
		}
	}
	return nil, nil
}

func (col *NetProtoCollector) Metrics() collector.MetricReaderMap {
	res := make(collector.MetricReaderMap)
	for _, reader := range col.protoReaders {
		name := "net-proto/" + reader.protocol + "/" + reader.field
		res[name] = reader.read
	}
	return res
}

func (col *NetProtoCollector) update(checkChange bool) error {
	counters, err := psnet.ProtoCounters(nil)
	if err != nil {
		return err
	}
	for _, counters := range counters {
		if checkChange {
			if _, ok := col.protocols[counters.Protocol]; !ok {
				return collector.MetricsChanged
			}
		}
		col.protocols[counters.Protocol] = counters
	}
	if checkChange && len(counters) != len(col.protocols) {
		// This means some previous metric is not available anymore
		return collector.MetricsChanged
	}
	return nil
}

func (col *NetProtoCollector) Update() (err error) {
	if err = col.update(true); err == nil {
		for _, protoReader := range col.protoReaders {
			if err := protoReader.update(); err != nil {
				return err
			}
		}
	}
	return
}

func (col *NetProtoCollector) MetricsChanged() error {
	return col.Update()
}

type protoStatReader struct {
	col      *NetProtoCollector
	protocol string
	field    string

	// Only one of the following 2 fields is used
	ring  *collector.ValueRing
	value bitflow.Value
}

func (reader *protoStatReader) update() error {
	if counters, ok := reader.col.protocols[reader.protocol]; ok {
		if val, ok := counters.Stats[reader.field]; ok {
			if reader.ring != nil {
				reader.ring.Add(collector.StoredValue(val))
			} else {
				reader.value = bitflow.Value(val)
			}
			return nil
		} else {
			return fmt.Errorf("Counter %v not found in protocol %v in NetProtoCollector", reader.field, reader.protocol)
		}
	} else {
		return fmt.Errorf("Protocol %v not found in NetProtoCollector", reader.protocol)
	}
}

func (reader *protoStatReader) read() bitflow.Value {
	if ring := reader.ring; ring != nil {
		return ring.GetDiff()
	} else {
		return reader.value
	}
}
