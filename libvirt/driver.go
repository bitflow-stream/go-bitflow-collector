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

	CpuStats() (VirTypedParameters, error)
	BlockStats(dev string) (VirDomainBlockStats, error)
	BlockInfo(dev string) (VirDomainBlockInfo, error)
	MemoryStats() (VirDomainMemoryStat, error)
	InterfaceStats(interfaceName string) (VirDomainInterfaceStats, error)
}

type DomainInfo struct {
	CpuTime uint64
	MaxMem  uint64
	Mem     uint64
}

type VirTypedParameters map[string]interface{}

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

type VirDomainMemoryStat map[int32]uint64

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
