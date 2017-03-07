package libvirt

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow-collector"
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

type LibvirtCollector struct {
	collector.AbstractCollector
	connectUri string
	driver     Driver
	factory    *collector.ValueRingFactory
	domains    map[string]Domain
}

func NewLibvirtCollector(uri string, driver Driver, factory *collector.ValueRingFactory) *LibvirtCollector {
	return &LibvirtCollector{
		AbstractCollector: collector.RootCollector("libvirt"),
		driver:            driver,
		connectUri:        uri,
		factory:           factory,
	}
}

func (col *LibvirtCollector) Init() ([]collector.Collector, error) {
	col.Close()
	col.domains = make(map[string]Domain)
	if err := col.fetchDomains(false); err != nil {
		return nil, err
	}
	res := make([]collector.Collector, 0, len(col.domains))
	for name, domain := range col.domains {
		res = append(res, col.newVmCollector(name, domain))
	}
	return res, nil
}

func (col *LibvirtCollector) Update() error {
	return col.fetchDomains(true)
}

func (col *LibvirtCollector) MetricsChanged() error {
	return col.Update()
}

func (col *LibvirtCollector) fetchDomains(checkChange bool) error {
	if err := col.driver.Connect(col.connectUri); err != nil {
		return err
	}
	domains, err := col.driver.ListDomains()
	if err != nil {
		return err
	}
	if checkChange && len(col.domains) != len(domains) {
		return collector.MetricsChanged
	}
	for _, domain := range domains {
		if name, err := domain.GetName(); err != nil {
			return err
		} else {
			if checkChange {
				if _, ok := col.domains[name]; !ok {
					return collector.MetricsChanged
				}
			}
			col.domains[name] = domain
		}
	}
	return nil
}

func (col *LibvirtCollector) Close() {
	if err := col.driver.Close(); err != nil {
		log.Errorln("Error closing libvirt connection:", err)
	}
}
