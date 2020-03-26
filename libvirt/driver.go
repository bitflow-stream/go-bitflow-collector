package libvirt

type Driver interface {
	Connect(uri string) error
	ListDomains() ([]Domain, error)
	Close() error
}

type Domain interface {
	GetName() (string, error)
	GetXML() (string, error)
	GetInfo() (DomainInfo, error)
	GetVolumeInfo() ([]VolumeInfo, error)

	CpuStats() (VirDomainCpuStats, error)
	BlockStats(dev string) (VirDomainBlockStats, error)
	BlockInfo(dev string) (VirDomainBlockInfo, error)
	InterfaceStats(interfaceName string) (VirDomainInterfaceStats, error)
	MemoryStats() (VirDomainMemoryStat, error)
}

type DomainInfo struct {
	CpuTime uint64
	MaxMem  uint64
	Mem     uint64
}

type VolumeInfo struct {
	Pool   string
	Image  string
	Driver string
	User   string
}

type VirDomainCpuStats struct {
	CpuTime    uint64
	UserTime   uint64
	SystemTime uint64
	VcpuTime   uint64
}

type VirDomainBlockStats struct {
	RdReq   int64
	WrReq   int64
	RdBytes int64
	WrBytes int64
}

type VirDomainBlockInfo struct {
	Allocation uint64
	Capacity   uint64
	Physical   uint64
}

type VirDomainMemoryStat struct {
	Available uint64
	Unused    uint64
}

type VirDomainInterfaceStats struct {
	RxBytes   int64
	RxPackets int64
	RxErrs    int64
	RxDrop    int64
	TxBytes   int64
	TxPackets int64
	TxErrs    int64
	TxDrop    int64
}
