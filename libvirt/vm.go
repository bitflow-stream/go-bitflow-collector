package libvirt

import (
	"fmt"
	"strings"

	"github.com/antongulenko/go-bitflow-collector"
	"gopkg.in/xmlpath.v1"
)

type vmCollector struct {
	collector.AbstractCollector
	parent        *LibvirtCollector
	name          string
	domain        Domain
	subCollectors []vmSubCollector
}

func (parent *LibvirtCollector) newVmCollector(name string, domain Domain) *vmCollector {
	return &vmCollector{
		AbstractCollector: parent.Child(name),
		parent:            parent,
		name:              name,
		domain:            domain,
	}
}

func (col *vmCollector) Init() ([]collector.Collector, error) {
	col.subCollectors = []vmSubCollector{
		NewVmGeneralCollector(col),
		NewMemoryCollector(col),
		NewCpuCollector(col),
		NewBlockCollector(col),
		NewInterfaceStatCollector(col),
	}
	collectors := make([]collector.Collector, len(col.subCollectors))
	for i, subCollector := range col.subCollectors {
		collectors[i] = subCollector
	}
	return collectors, nil
}

func (col *vmCollector) Update() error {
	xmlData, err := col.domain.GetXML()
	if err != nil {
		return fmt.Errorf("Failed to retrieve XML domain description of %s: %v", col.name, err)
	}
	xmlDesc, err := xmlpath.Parse(strings.NewReader(xmlData))
	if err != nil {
		return fmt.Errorf("Failed to parse XML domain description of %s: %v", col.name, err)
	}
	for _, reader := range col.subCollectors {
		reader.description(xmlDesc)
	}
	return nil
}

func (col *vmCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *vmCollector) prefix() string {
	return "libvirt/" + col.Name + "/"
}

type vmSubCollector interface {
	collector.Collector
	description(xmlDesc *xmlpath.Node)
}

type vmSubCollectorImpl struct {
	collector.AbstractCollector
	parent *vmCollector
}

func (col *vmSubCollectorImpl) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *vmSubCollectorImpl) description(xmlDesc *xmlpath.Node) {
}

func (parent *vmCollector) child(name string) vmSubCollectorImpl {
	return vmSubCollectorImpl{
		AbstractCollector: parent.Child(name),
		parent:            parent,
	}
}
