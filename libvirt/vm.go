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
	subcollectors []vmSubcollector
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
	col.subcollectors = []vmSubcollector{
		NewVmGeneralCollector(col),
		NewMemoryCollector(col),
		NewCpuCollector(col),
		NewBlockCollector(col),
		NewInterfaceStatCollector(col),
	}
	collectors := make([]collector.Collector, len(col.subcollectors))
	for i, collector := range col.subcollectors {
		collectors[i] = collector
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
	for _, reader := range col.subcollectors {
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

type vmSubcollector interface {
	collector.Collector
	description(xmlDesc *xmlpath.Node)
}

type vmSubcollectorImpl struct {
	collector.AbstractCollector
	parent *vmCollector
}

func (col *vmSubcollectorImpl) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *vmSubcollectorImpl) description(xmlDesc *xmlpath.Node) {
}

func (parent *vmCollector) child(name string) vmSubcollectorImpl {
	return vmSubcollectorImpl{
		AbstractCollector: parent.Child(name),
		parent:            parent,
	}
}
