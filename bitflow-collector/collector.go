package main

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/cmd_helper"
	"github.com/antongulenko/go-bitflow-collector/libvirt"
	"github.com/antongulenko/go-bitflow-collector/mock"
	"github.com/antongulenko/go-bitflow-collector/ovsdb"
	"github.com/antongulenko/go-bitflow-collector/psutil"
	"github.com/antongulenko/golib"
)

var (
	collect_local_interval = 500 * time.Millisecond
	sink_interval          = 500 * time.Millisecond

	all_metrics           = false
	include_basic_metrics = false
	user_include_metrics  golib.StringSlice
	user_exclude_metrics  golib.StringSlice

	libvirt_uri = libvirt.LocalUri // libvirt.SshUri("host", "keyFile")
	ovsdb_host  = ""

	proc_update_pids time.Duration

	pcap_nics golib.StringSlice

	updateFrequencies = map[*regexp.Regexp]time.Duration{
		regexp.MustCompile("^psutil/pids$"):       1500 * time.Millisecond, // Changed processes
		regexp.MustCompile("^psutil/disk-usage$"): 5 * time.Second,         // Changed local partitions
		regexp.MustCompile("^libvirt$"):           10 * time.Second,        // New VMs
		regexp.MustCompile("^libvirt/[^/]+$"):     30 * time.Second,        // Changed VM configuration
	}

	ringFactory = collector.ValueRingFactory{
		// This is an important package-wide constant: time-window for all aggregated values
		Interval: 1000 * time.Millisecond,
	}
)

const (
	FailedCollectorCheckInterval   = 5 * time.Second
	FilteredCollectorCheckInterval = 3 * time.Second

	// Negative look-ahead is not supported, so explicitly encode the negation of the substring "all"
	negatedAll = "([^a]|a[^l]|al[^l])"
)

var (
	includeMetricsRegexes []*regexp.Regexp
	excludeMetricsRegexes = []*regexp.Regexp{
		regexp.MustCompile("^mock/.$"),
		regexp.MustCompile("^net-proto/(UdpLite|IcmpMsg)"),                         // Some extended protocol-metrics
		regexp.MustCompile("^disk-io/" + negatedAll),                               // Disk IO for specific partitions/disks
		regexp.MustCompile("^disk-usage/" + negatedAll),                            // Disk usage for specific partitions
		regexp.MustCompile("^net-proto/tcp/(MaxConn|RtpAlgorithm|RtpMin|RtoMax)$"), // Some irrelevant TCP/IP settings
		regexp.MustCompile("^net-proto/ip/(DefaultTTL|Forwarding)$"),
	}
	includeBasicMetricsRegexes = []*regexp.Regexp{
		regexp.MustCompile("^(cpu|mem/percent)$"),
		regexp.MustCompile("^disk-io/all/(io|ioTime|ioBytes)$"),
		regexp.MustCompile("^net-io/(bytes|packets|dropped|errors)$"),
		regexp.MustCompile("^proc/.+/(cpu|mem/rss|disk/(io|ioBytes)|net-io/(bytes|packets|dropped|errors))$"),
	}
)

func init() {
	flag.StringVar(&libvirt_uri, "libvirt", libvirt_uri, "Libvirt connection uri (default is local system)")
	flag.StringVar(&ovsdb_host, "ovsdb", ovsdb_host, "OVSDB host to connect to. Empty for localhost. Port is "+strconv.Itoa(ovsdb.DefaultOvsdbPort))
	flag.BoolVar(&all_metrics, "a", all_metrics, "Disable built-in filters on available metrics")
	flag.Var(&user_exclude_metrics, "exclude", "Metrics to exclude (substring match)")
	flag.Var(&user_include_metrics, "include", "Metrics to include exclusively (substring match)")
	flag.BoolVar(&include_basic_metrics, "basic", include_basic_metrics, "Include only a certain basic subset of metrics")

	flag.DurationVar(&proc_update_pids, "proc-interval", 1500*time.Millisecond, "Interval for updating list of observed pids")

	flag.DurationVar(&collect_local_interval, "ci", collect_local_interval, "Interval for collecting local samples")
	flag.DurationVar(&sink_interval, "si", sink_interval, "Interval for sinking (sending/printing/...) data when collecting local samples")

	flag.Var(&pcap_nics, "nic", "NICs to capture packets from for PCAP-based "+
		"monitoring of process network IO (/proc/.../net-pcap/...). Defaults to all physical NICs.")
}

func configurePcap() {
	psutil.PcapNics = pcap_nics
}

func createCollectorSource(cmd *cmd_helper.CmdDataCollector) *collector.CollectorSource {
	configurePcap()
	ringFactory.Length = int(ringFactory.Interval/collect_local_interval) * 10 // Make sure enough samples can be buffered
	var cols []collector.Collector

	cols = append(cols, mock.NewMockCollector(&ringFactory))
	cols = append(cols, createProcessCollectors(cmd)...)
	cols = append(cols, libvirt.NewLibvirtCollector(libvirt_uri, libvirt.NewDriver(), &ringFactory))
	cols = append(cols, ovsdb.NewOvsdbCollector(ovsdb_host, &ringFactory))

	if all_metrics {
		excludeMetricsRegexes = nil
	}
	if include_basic_metrics {
		includeMetricsRegexes = append(includeMetricsRegexes, includeBasicMetricsRegexes...)
	}
	for _, exclude := range user_exclude_metrics {
		regex, err := regexp.Compile(exclude)
		if err != nil {
			golib.Checkerr(fmt.Errorf("Error compiling exclude regex: %v", err))
		}
		excludeMetricsRegexes = append(excludeMetricsRegexes, regex)
	}
	for _, include := range user_include_metrics {
		regex, err := regexp.Compile(include)
		if err != nil {
			golib.Checkerr(fmt.Errorf("Error compiling include regex: %v", err))
		}
		includeMetricsRegexes = append(includeMetricsRegexes, regex)
	}

	return &collector.CollectorSource{
		RootCollectors:                 cols,
		UpdateFrequencies:              updateFrequencies,
		CollectInterval:                collect_local_interval,
		SinkInterval:                   sink_interval,
		ExcludeMetrics:                 excludeMetricsRegexes,
		IncludeMetrics:                 includeMetricsRegexes,
		FailedCollectorCheckInterval:   FailedCollectorCheckInterval,
		FilteredCollectorCheckInterval: FilteredCollectorCheckInterval,
	}
}
