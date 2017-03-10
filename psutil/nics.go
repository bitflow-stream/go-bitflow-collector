package psutil

import (
	"github.com/antongulenko/go-bitflow-collector"
	psnet "github.com/shirou/gopsutil/net"
)

type NetPerNicCollector struct {
	collector.AbstractCollector

	interfaceCollectors map[string]*PsutilNetCollector
	root                *PsutilRootCollector
}

func NewNetPerNicCollector(Root *PsutilRootCollector) *NetPerNicCollector {
	return &NetPerNicCollector{
		AbstractCollector: collector.RootCollector("nics"),
		root:              Root,
	}
}

func (col *NetPerNicCollector) Init() ([]collector.Collector, error) {
	col.interfaceCollectors = make(map[string]*PsutilNetCollector)
	if err := col.Update(); err != nil {
		return nil, err
	}

	readers := make([]collector.Collector, 0, len(col.interfaceCollectors))
	for _, reader := range col.interfaceCollectors {
		readers = append(readers, reader)
	}
	return readers, nil
}

func (col *NetPerNicCollector) Update() error {
	io, err := psnet.IOCounters(true)
	if err == nil {
		for _, nic := range io {
			reader, ok := col.interfaceCollectors[nic.Name]
			if !ok {
				reader = newNetCollector(col.root, nic.Name)
				col.interfaceCollectors[nic.Name] = reader
			}

		}
	}

	return nil
}
