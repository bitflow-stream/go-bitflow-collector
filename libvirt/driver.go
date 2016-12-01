package libvirt

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	lib "github.com/rgbkrk/libvirt-go"
)

const (
	NO_FLAGS            = 0
	FETCH_DOMAINS_FLAGS = lib.VIR_CONNECT_LIST_DOMAINS_ACTIVE | lib.VIR_CONNECT_LIST_DOMAINS_RUNNING

	MAX_NUM_MEMORY_STATS = 8
	MAX_NUM_CPU_STATS    = 4
)

// ============================================ Interface ============================================

type Driver interface {
	Connect(uri string) error
	ListDomains() ([]Domain, error)
	Close() error
}

type Domain interface {
	GetName() (string, error)
	GetXML() (string, error)
	GetInfo() (DomainInfo, error)

	CpuStats() (lib.VirTypedParameters, error)
	BlockStats(dev string) (lib.VirDomainBlockStats, error)
	BlockInfo(dev string) (lib.VirDomainBlockInfo, error)
	MemoryStats() ([]lib.VirDomainMemoryStat, error)
	InterfaceStats(interfaceName string) (lib.VirDomainInterfaceStats, error)
}

type DomainInfo struct {
	CpuTime uint64
	MaxMem  uint64
	Mem     uint64
}

// ============================================ Real Implementation ============================================

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

func (driver *DriverImpl) connection() (*lib.VirConnection, error) {
	conn := driver.conn
	if conn != nil {
		if alive, err := conn.IsAlive(); err != nil || !alive {
			log.Warnln("Libvirt alive connection check failed:", err)
			driver.Close()
			conn = nil
		}
	}
	if conn == nil {
		if driver.uri == "" {
			return nil, errors.New("Drier.Connect() has not yet been called.")
		}
		newConn, err := lib.NewVirConnection(driver.uri)
		if err != nil {
			return nil, err
		}
		conn = &newConn
		driver.conn = conn
	}
	return conn, nil
}

func (col *DriverImpl) Close() (err error) {
	if col.conn != nil {
		_, err = col.conn.CloseConnection()
		col.conn = nil
	}
	return
}

type DomainImpl struct {
	domain lib.VirDomain
}

func (d *DomainImpl) GetName() (string, error) {
	return d.domain.GetName()
}

func (d *DomainImpl) CpuStats() (lib.VirTypedParameters, error) {
	stats := make(lib.VirTypedParameters, MAX_NUM_CPU_STATS)
	_, err := d.domain.GetCPUStats(&stats, len(stats), -1, 1, NO_FLAGS)
	return stats, err
}

func (d *DomainImpl) BlockStats(dev string) (lib.VirDomainBlockStats, error) {
	return d.domain.BlockStats(dev)
}

func (d *DomainImpl) BlockInfo(dev string) (lib.VirDomainBlockInfo, error) {
	return d.domain.GetBlockInfo(dev, NO_FLAGS)
}

func (d *DomainImpl) MemoryStats() ([]lib.VirDomainMemoryStat, error) {
	return d.domain.MemoryStats(MAX_NUM_MEMORY_STATS, NO_FLAGS)
}

func (d *DomainImpl) InterfaceStats(interfaceName string) (lib.VirDomainInterfaceStats, error) {
	return d.domain.InterfaceStats(interfaceName)
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

// ============================================ Mock Implementation ============================================

type MockDriver struct {
	uri         string
	injectedErr error
}

func (d *MockDriver) Connect(uri string) error {
	if d.injectedErr != nil {
		return d.injectedErr
	}
	d.uri = uri
	return nil
}

func (d *MockDriver) ListDomains() ([]Domain, error) {
	if err := d.err(); err != nil {
		return nil, err
	}
	return d.makeMockDomains(), nil
}

func (d *MockDriver) Close() error {
	d.uri = ""
	return d.injectedErr
}

func (d *MockDriver) err() error {
	if d.uri == "" {
		return errors.New("MockDomain: not connected")
	}
	return d.injectedErr
}

func (d *MockDriver) makeMockDomains() []Domain {
	return []Domain{
		&MockDomain{
			driver: d,
			name:   "domain1",
			cpu: lib.VirTypedParameters{
				lib.VirTypedParameter{},
				lib.VirTypedParameter{},
			},
			blockStats: map[string]lib.VirDomainBlockStats{
				"vda": lib.VirDomainBlockStats{},
			},
			blockInfo: map[string]lib.VirDomainBlockInfo{
				"vda": lib.VirDomainBlockInfo{},
			},
			mem: []lib.VirDomainMemoryStat{
				{},
				{},
			},
			interfaces: map[string]lib.VirDomainInterfaceStats{
				"eth0": lib.VirDomainInterfaceStats{},
			},
			xml: "",
			info: DomainInfo{
				CpuTime: 0,
				MaxMem:  0,
				Mem:     0,
			},
		},
		&MockDomain{
			driver: d,
			name:   "domain2",
			cpu: lib.VirTypedParameters{
				lib.VirTypedParameter{},
				lib.VirTypedParameter{},
			},
			blockStats: map[string]lib.VirDomainBlockStats{
				"vda": lib.VirDomainBlockStats{},
			},
			blockInfo: map[string]lib.VirDomainBlockInfo{
				"vda": lib.VirDomainBlockInfo{},
			},
			mem: []lib.VirDomainMemoryStat{
				{},
				{},
			},
			interfaces: map[string]lib.VirDomainInterfaceStats{
				"eth0": lib.VirDomainInterfaceStats{},
			},
			xml: "",
			info: DomainInfo{
				CpuTime: 0,
				MaxMem:  0,
				Mem:     0,
			},
		},
	}
}

type MockDomain struct {
	driver *MockDriver

	name       string
	cpu        lib.VirTypedParameters
	blockStats map[string]lib.VirDomainBlockStats
	blockInfo  map[string]lib.VirDomainBlockInfo
	mem        []lib.VirDomainMemoryStat
	interfaces map[string]lib.VirDomainInterfaceStats
	xml        string
	info       DomainInfo
}

func (d *MockDomain) err() error {
	return d.driver.err()
}

func (d *MockDomain) GetName() (string, error) {
	return d.name, d.err()
}

func (d *MockDomain) CpuStats() (lib.VirTypedParameters, error) {
	return d.cpu, d.err()
}

func (d *MockDomain) BlockStats(dev string) (lib.VirDomainBlockStats, error) {
	if err := d.err(); err != nil {
		return lib.VirDomainBlockStats{}, err
	}
	if res, ok := d.blockStats[dev]; !ok {
		return lib.VirDomainBlockStats{}, fmt.Errorf("Device %v for BlockStats not found in MockDomain %v", dev, d.name)
	} else {
		return res, nil
	}
}

func (d *MockDomain) BlockInfo(dev string) (lib.VirDomainBlockInfo, error) {
	if err := d.err(); err != nil {
		return lib.VirDomainBlockInfo{}, err
	}
	if res, ok := d.blockInfo[dev]; !ok {
		return lib.VirDomainBlockInfo{}, fmt.Errorf("Device %v for BlockInfo not found in MockDomain %v", dev, d.name)
	} else {
		return res, nil
	}
}

func (d *MockDomain) MemoryStats() ([]lib.VirDomainMemoryStat, error) {
	return d.mem, d.err()
}

func (d *MockDomain) InterfaceStats(interfaceName string) (lib.VirDomainInterfaceStats, error) {
	if err := d.err(); err != nil {
		return lib.VirDomainInterfaceStats{}, err
	}
	if res, ok := d.interfaces[interfaceName]; !ok {
		return lib.VirDomainInterfaceStats{}, fmt.Errorf("Interface %v not found in MockDomain %v", interfaceName, d.name)
	} else {
		return res, nil
	}
}

func (d *MockDomain) GetXML() (string, error) {
	return d.xml, d.err()
}

func (d *MockDomain) GetInfo() (DomainInfo, error) {
	return d.info, d.err()
}
