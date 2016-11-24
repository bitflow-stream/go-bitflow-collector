package psutil

import (
	"fmt"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	psnet "github.com/shirou/gopsutil/net"
)

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

type PsutilNetProtoCollector struct {
	collector.AbstractCollector
	Factory *collector.ValueRingFactory

	protocols    map[string]psnet.ProtoCountersStat
	protoReaders []*protoStatReader
}

func (col *PsutilNetProtoCollector) Init() error {
	col.Reset(col)
	col.protocols = make(map[string]psnet.ProtoCountersStat)
	col.protoReaders = nil

	// TODO missing: metrics about individual connections and NICs
	if err := col.update(false); err != nil {
		return err
	}
	col.Readers = make(map[string]collector.MetricReader)
	for proto, counters := range col.protocols {
		for statName, _ := range counters.Stats {
			var ring *collector.ValueRing
			if !absoluteNetProtoValues[statName] {
				ring = col.Factory.NewValueRing()
			}
			name := "net-proto/" + proto + "/" + statName
			protoReader := &protoStatReader{
				col:      col,
				protocol: proto,
				field:    statName,
				ring:     ring,
			}
			col.Readers[name] = protoReader.read
			col.protoReaders = append(col.protoReaders, protoReader)
		}
	}
	return nil
}

func (col *PsutilNetProtoCollector) update(checkChange bool) error {
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

func (col *PsutilNetProtoCollector) Update() (err error) {
	if err = col.update(true); err == nil {
		for _, protoReader := range col.protoReaders {
			if err := protoReader.update(); err != nil {
				return err
			}
		}
		col.UpdateMetrics()
	}
	return
}

type protoStatReader struct {
	col      *PsutilNetProtoCollector
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
			return fmt.Errorf("Counter %v not found in protocol %v in PsutilNetProtoCollector", reader.field, reader.protocol)
		}
	} else {
		return fmt.Errorf("Protocol %v not found in PsutilNetProtoCollector", reader.protocol)
	}
}

func (reader *protoStatReader) read() bitflow.Value {
	if ring := reader.ring; ring != nil {
		return ring.GetDiff()
	} else {
		return reader.value
	}
}
