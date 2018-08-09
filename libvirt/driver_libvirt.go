// +build !nolibvirt

package libvirt

import (
	"errors"

	lib "github.com/rgbkrk/libvirt-go"
	log "github.com/sirupsen/logrus"
)

const (
	NO_FLAGS            = 0
	FETCH_DOMAINS_FLAGS = lib.VIR_CONNECT_LIST_DOMAINS_ACTIVE | lib.VIR_CONNECT_LIST_DOMAINS_RUNNING

	MAX_NUM_MEMORY_STATS = 8
	MAX_NUM_CPU_STATS    = 4
)

func NewDriver() Driver {
	return new(DriverImpl)
}

type DriverImpl struct {
	uri  string
	conn *lib.VirConnection
}

func (d *DriverImpl) Connect(uri string) error {
	d.uri = uri
	return nil
}

func (d *DriverImpl) ListDomains() ([]Domain, error) {
	conn, err := d.connection()
	if err != nil {
		return nil, err
	}
	virDomains, err := conn.ListAllDomains(FETCH_DOMAINS_FLAGS)
	if err != nil {
		return nil, err
	}
	domains := make([]Domain, len(virDomains))
	for i, domain := range virDomains {
		domains[i] = &DomainImpl{domain}
	}
	return domains, nil
}

func (d *DriverImpl) connection() (*lib.VirConnection, error) {
	conn := d.conn
	if conn != nil {
		if alive, err := conn.IsAlive(); err != nil || !alive {
			log.Warnln("Libvirt alive connection check failed:", err)
			d.Close()
			conn = nil
		}
	}
	if conn == nil {
		if d.uri == "" {
			return nil, errors.New("Drier.Connect() has not yet been called.")
		}
		newConn, err := lib.NewVirConnection(d.uri)
		if err != nil {
			return nil, err
		}
		conn = &newConn
		d.conn = conn
	}
	return conn, nil
}

func (d *DriverImpl) Close() (err error) {
	if d.conn != nil {
		_, err = d.conn.CloseConnection()
		d.conn = nil
	}
	return
}

type DomainImpl struct {
	domain lib.VirDomain
}

func (d *DomainImpl) GetName() (string, error) {
	return d.domain.GetName()
}

func (d *DomainImpl) CpuStats() (VirTypedParameters, error) {
	stats := make(lib.VirTypedParameters, MAX_NUM_CPU_STATS)
	_, err := d.domain.GetCPUStats(&stats, len(stats), -1, 1, NO_FLAGS)
	if err != nil {
		return nil, err
	}
	result := make(VirTypedParameters, len(stats))
	for _, stat := range stats {
		result[stat.Name] = stat.Value
	}
	return result, nil
}

func (d *DomainImpl) BlockStats(dev string) (VirDomainBlockStats, error) {
	stats, err := d.domain.BlockStats(dev)
	return VirDomainBlockStats{
		RdReq:   stats.RdReq,
		WrReq:   stats.WrReq,
		RdBytes: stats.RdBytes,
		WrBytes: stats.WrBytes,
	}, err
}

func (d *DomainImpl) BlockInfo(dev string) (VirDomainBlockInfo, error) {
	stats, err := d.domain.GetBlockInfo(dev, NO_FLAGS)
	return VirDomainBlockInfo{
		Allocation: stats.Allocation(),
		Capacity:   stats.Capacity(),
		Physical:   stats.Physical(),
	}, err
}

func (d *DomainImpl) MemoryStats() (VirDomainMemoryStat, error) {
	stats, err := d.domain.MemoryStats(MAX_NUM_MEMORY_STATS, NO_FLAGS)
	if err != nil {
		return nil, err
	}
	results := make(VirDomainMemoryStat, len(stats))
	for _, stat := range stats {
		results[stat.Tag] = stat.Val
	}
	return results, nil
}

func (d *DomainImpl) InterfaceStats(interfaceName string) (VirDomainInterfaceStats, error) {
	stats, err := d.domain.InterfaceStats(interfaceName)
	return VirDomainInterfaceStats{
		RxBytes:   stats.RxBytes,
		RxPackets: stats.RxPackets,
		RxErrs:    stats.RxErrs,
		RxDrop:    stats.RxDrop,
		TxBytes:   stats.TxBytes,
		TxPackets: stats.TxPackets,
		TxErrs:    stats.TxErrs,
		TxDrop:    stats.TxDrop,
	}, err
}

func (d *DomainImpl) GetXML() (string, error) {
	return d.domain.GetXMLDesc(NO_FLAGS)
}

func (d *DomainImpl) GetInfo() (res DomainInfo, err error) {
	var info lib.VirDomainInfo
	info, err = d.domain.GetInfo()
	if err != nil {
		return
	}
	res.CpuTime = info.GetCpuTime()
	res.MaxMem = info.GetMaxMem()
	res.Mem = info.GetMemory()
	return
}
