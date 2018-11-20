package libvirt

import (
	"fmt"

	"github.com/bitflow-stream/go-bitflow-collector"
	log "github.com/sirupsen/logrus"
)

const LocalUri = "qemu:///system"

/*
	// TODO info about the node/hypervisor?

	// State
	v.GetState()
	v.IsActive()

	// Info
	v.GetID()
	v.GetMetadata()
	v.GetName()
	v.GetUUID()
	v.GetUUIDString()
	v.GetAutostart()
*/

func SshUri(host string, keyFile string) string {
	if keyFile != "" {
		keyFile = "&keyfile=" + keyFile
	}
	return fmt.Sprintf("qemu+ssh://%s/system?no_verify=1%s", host, keyFile)
}

type Collector struct {
	collector.AbstractCollector
	connectUri string
	driver     Driver
	factory    *collector.ValueRingFactory
	domains    map[string]Domain
}

func NewLibvirtCollector(uri string, driver Driver, factory *collector.ValueRingFactory) *Collector {
	return &Collector{
		AbstractCollector: collector.RootCollector("libvirt"),
		driver:            driver,
		connectUri:        uri,
		factory:           factory,
	}
}

func (parent *Collector) Init() ([]collector.Collector, error) {
	parent.Close()
	parent.domains = make(map[string]Domain)
	if err := parent.fetchDomains(false); err != nil {
		return nil, err
	}
	res := make([]collector.Collector, 0, len(parent.domains))
	for name, domain := range parent.domains {
		res = append(res, parent.newVmCollector(name, domain))
	}
	return res, nil
}

func (parent *Collector) Update() error {
	return parent.fetchDomains(true)
}

func (parent *Collector) MetricsChanged() error {
	return parent.Update()
}

func (parent *Collector) fetchDomains(checkChange bool) error {
	if err := parent.driver.Connect(parent.connectUri); err != nil {
		return err
	}
	domains, err := parent.driver.ListDomains()
	if err != nil {
		return err
	}
	if checkChange && len(parent.domains) != len(domains) {
		return collector.MetricsChanged
	}
	for _, domain := range domains {
		if name, err := domain.GetName(); err != nil {
			return err
		} else {
			if checkChange {
				if _, ok := parent.domains[name]; !ok {
					return collector.MetricsChanged
				}
			}
			parent.domains[name] = domain
		}
	}
	return nil
}

func (parent *Collector) Close() {
	if err := parent.driver.Close(); err != nil {
		log.Errorln("Error closing libvirt connection:", err)
	}
}
