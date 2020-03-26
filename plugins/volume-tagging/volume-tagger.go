package main

import (
	"fmt"
	collector "github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow-collector/libvirt"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/bitflow-stream/go-bitflow/script/reg"
	"github.com/bitflow-stream/go-bitflow/steps"
	log "github.com/sirupsen/logrus"
	"strings"
)

func RegisterLibvirtVolumeTagger(name string, b reg.ProcessorRegistry) {
	step := b.RegisterStep(name, func(p *bitflow.SamplePipeline, params map[string]interface{}) error {
		step := NewLibvirtVolumeTagger(params["uri"].(string), libvirt.NewDriver(), params["volumeKey"].(string),
			params["libvirtInstanceKey"].(string))
		p.Add(step)
		return nil
	}, "Append volume IDs to libvirt VM samples.").
		Optional("uri", reg.String(), libvirt.LocalUri).
		Optional("volumeKey", reg.String(), "volumes").
		Optional("libvirtInstanceKey", reg.String(), "vm")
	steps.AddTagChangeListenerParams(step)
}

func NewLibvirtVolumeTagger(uri string, driver libvirt.Driver, volumeKey string, libvirtInstanceKey string) *LibvirtVolumeTagger {
	return &LibvirtVolumeTagger{
		connectUri:         uri,
		driver:             driver,
		volumeKey:          volumeKey,
		libvirtInstanceKey: libvirtInstanceKey,
	}
}

type LibvirtVolumeTagger struct {
	bitflow.NoopProcessor
	connectUri string
	driver     libvirt.Driver
	domains    map[string]libvirt.Domain

	volumeKey          string
	libvirtInstanceKey string
}

func (l *LibvirtVolumeTagger) Init() error {
	l.domains = make(map[string]libvirt.Domain)
	if err := l.fetchDomains(false); err != nil {
		return err
	}
	return nil
}

func (l *LibvirtVolumeTagger) fetchDomains(checkChange bool) error {
	if err := l.driver.Connect(l.connectUri); err != nil {
		return err
	}
	domains, err := l.driver.ListDomains()
	if err != nil {
		return err
	}
	if checkChange && len(l.domains) != len(domains) {
		return collector.MetricsChanged
	}
	for _, domain := range domains {
		if name, err := domain.GetName(); err != nil {
			return err
		} else {
			if checkChange {
				if _, ok := l.domains[name]; !ok {
					return collector.MetricsChanged
				}
			}
			l.domains[name] = domain
		}
	}
	return nil
}

func (l *LibvirtVolumeTagger) String() string {
	return fmt.Sprintf("Libvirt volume tagger connected to libvirt via %v", l.connectUri)
}

func (l *LibvirtVolumeTagger) Sample(sample *bitflow.Sample, header *bitflow.Header) error {
	// Check sample tag for vm
	libvirtInstance := sample.Tag(l.libvirtInstanceKey)
	if libvirtInstance != "" {
		if _, ok := l.domains[libvirtInstance]; !ok { // Currently loaded domains do not contain sample's libvirt instance ID
			if err := l.fetchDomains(false); err != nil { // Update domains
				log.Warn("Error while updating libvirt domains.")
				return l.NoopProcessor.Sample(sample, header)
			}
		}
		if domain, ok := l.domains[libvirtInstance]; ok { // Check if they are containing it now
			var volumeIds []string
			if volumeInfo, err := domain.GetVolumeInfo(); err == nil { // If yes, get volume info
				for _, info := range volumeInfo { // Make a list out of the IDs
					volumeIds = append(volumeIds, info.Image)
				}
				sample.SetTag(l.volumeKey, strings.Join(volumeIds, ",")) // String list representation as tag
			}
		}
	}
	return l.NoopProcessor.Sample(sample, header)
}
