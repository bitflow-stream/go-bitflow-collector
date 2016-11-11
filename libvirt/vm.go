package libvirt

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/golib"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
)

type vmMetricsCollector struct {
	col     *LibvirtCollector
	name    string
	readers []*activatedMetricsReader

	needXmlDesc   bool
	xmlDescParsed time.Time
}

var UpdateXmlDescription = errors.New("XML domain description must be updated")

func (collector *vmMetricsCollector) update() error {
	if domain, ok := collector.col.domains[collector.name]; !ok {
		return fmt.Errorf("Warning: libvirt domain %v not found", collector.name)
	} else {
		return collector.updateReaders(domain)
	}
}

func (collector *vmMetricsCollector) updateReaders(domain lib.VirDomain) error {
	updateDesc := false
	var res golib.MultiError
	for _, reader := range collector.readers {
		if reader.active {
			if err := reader.update(domain); err == UpdateXmlDescription {
				collector.needXmlDesc = true
				updateDesc = true
			} else if err != nil {
				res.Add(fmt.Errorf("Failed to update domain %s: %v", collector.name, err))
				updateDesc = true
				break
			}
		}
	}
	if !updateDesc && time.Now().Sub(collector.xmlDescParsed) >= DomainReparseInterval {
		updateDesc = true
	}
	if collector.needXmlDesc && updateDesc {
		res.Add(collector.updateXmlDesc(domain))
	}
	return res.NilOrError()
}

func (collector *vmMetricsCollector) updateXmlDesc(domain lib.VirDomain) error {
	xmlData, err := domain.GetXMLDesc(NO_FLAGS)
	if err != nil {
		return fmt.Errorf("Failed to retrieve XML domain description of %s: %v", collector.name, err)
	}
	xmlDesc, err := xmlpath.Parse(strings.NewReader(xmlData))
	if err != nil {
		return fmt.Errorf("Failed to parse XML domain description of %s: %v", collector.name, err)
	}
	collector.xmlDescParsed = time.Now()
	for _, reader := range collector.readers {
		reader.description(xmlDesc)
	}
	return nil
}

type activatedMetricsReader struct {
	vmMetricsReader
	active bool
}

func (reader *activatedMetricsReader) activate() {
	reader.active = true
}

type vmMetricsReader interface {
	register(domainName string) map[string]collector.MetricReader
	description(xmlDesc *xmlpath.Node)
	update(domain lib.VirDomain) error
}
