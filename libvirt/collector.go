package libvirt

import (
	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/golib"
	lib "github.com/rgbkrk/libvirt-go"
)

type LibvirtCollector struct {
	collector.AbstractCollector
	ConnectUri string
	Factory    *collector.ValueRingFactory
	conn       *lib.VirConnection
	domains    map[string]lib.VirDomain
	vmReaders  []*vmMetricsCollector
}

func (col *LibvirtCollector) Init() error {
	col.Close()
	col.Reset(col)
	col.domains = make(map[string]lib.VirDomain)
	if err := col.fetchDomains(false); err != nil {
		return err
	}
	col.Readers = make(map[string]collector.MetricReader)
	col.vmReaders = make([]*vmMetricsCollector, 0, len(col.domains))
	for name, _ := range col.domains {
		vmReader := &vmMetricsCollector{
			col:  col,
			name: name,
			readers: []*activatedMetricsReader{
				&activatedMetricsReader{NewVmGeneralReader(col.Factory), false},
				&activatedMetricsReader{new(memoryStatReader), false},
				&activatedMetricsReader{NewCpuStatReader(col.Factory), false},
				&activatedMetricsReader{new(blockStatReader), false},
				&activatedMetricsReader{NewInterfaceStatReader(col.Factory), false},
			},
		}
		for _, reader := range vmReader.readers {
			for metric, registeredReader := range reader.register(name) {
				// The notify mechanism here is to avoid unnecessary libvirt
				// API-calls for metrics that are filtered out
				col.Readers[metric] = registeredReader
				col.Notify[metric] = reader.activate
			}
		}
		col.vmReaders = append(col.vmReaders, vmReader)
	}
	return nil
}

func (col *LibvirtCollector) Update() (err error) {
	if err = col.fetchDomains(true); err == nil {
		if err = col.updateVms(); err == nil {
			col.UpdateMetrics()
		}
	}
	return
}

func (col *LibvirtCollector) fetchDomains(checkChange bool) error {
	conn, err := col.connection()
	if err != nil {
		return err
	}
	domains, err := conn.ListAllDomains(FETCH_DOMAINS_FLAGS) // No flags: return all domains
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

func (col *LibvirtCollector) updateVms() error {
	var res golib.MultiError
	for _, vmReader := range col.vmReaders {
		res.Add(vmReader.update())
		if len(res) > tolerated_vm_update_errors {
			// Update other VMs, even if some of them fail
			break
		}
	}
	return res.NilOrError()
}

func (col *LibvirtCollector) connection() (*lib.VirConnection, error) {
	conn := col.conn
	if conn != nil {
		if alive, err := conn.IsAlive(); err != nil || !alive {
			log.Warnln("Libvirt alive connection check failed:", err)
			col.Close()
			conn = nil
		}
	}
	if conn == nil {
		newConn, err := lib.NewVirConnection(col.ConnectUri)
		if err != nil {
			return nil, err
		}
		conn = &newConn
		col.conn = conn
	}
	return conn, nil
}

func (col *LibvirtCollector) Close() {
	if col.conn != nil {
		if _, err := col.conn.CloseConnection(); err != nil {
			log.Errorln("Error closing libvirt connection:", err)
		}
		col.conn = nil
	}
}
