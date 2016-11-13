package pcap

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ProcNetDir         = "/proc/net"
	AllConnectionTypes = []string{"tcp", "udp", "tcp6", "udp6"}

	stateMap = map[string]string{
		"01": "established",
		"02": "syn_sent",
		"03": "syn_recv",
		"04": "fin_wait1",
		"05": "fin_wait2",
		"06": "time_wait",
		"07": "close",
		"08": "close_wait",
		"09": "last_ack",
		"0A": "listen",
		"0B": "closing",
	}
)

func ReadAllConnections() ([]*Connection, error) {
	var all []*Connection
	for _, typ := range AllConnectionTypes {
		cons, err := ReadConnections(typ)
		if err != nil {
			return nil, err
		}
		all = append(all, cons...)
	}
	return all, nil
}

func ReadConnections(typ string) ([]*Connection, error) {
	filename := filepath.Join(ProcNetDir, typ)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var cons []*Connection
	for _, line := range lines[1 : len(lines)-1] {
		con, err := parseConnectionLine(typ, line)
		if err != nil {
			return nil, err
		}
		cons = append(cons, con)
	}
	return cons, nil
}

func parseConnectionLine(typ string, line string) (*Connection, error) {
	parts := strings.Fields(line)

	ip_port := strings.Split(parts[1], ":")
	port_str := ip_port[1]
	port, err := parseHex(port_str)
	if err != nil {
		return nil, fmt.Errorf("Error parsing port %v: %v, line: %v", port_str, err, line)
	}
	remote_ip_port := strings.Split(parts[2], ":")
	remote_port_str := remote_ip_port[1]
	remote_port, err := parseHex(remote_port_str)
	if err != nil {
		return nil, fmt.Errorf("Error parsing remote port %v: %v, line: %v", remote_port_str, err, line)
	}

	ip_str := ip_port[0]
	ip, err := parseIp(ip_str)
	if err != nil {
		return nil, fmt.Errorf("Error parsing ip %v: %v, line: %v", ip_str, err, line)
	}
	remote_ip_str := remote_ip_port[0]
	remote_ip, err := parseIp(remote_ip_str)
	if err != nil {
		return nil, fmt.Errorf("Error parsing remote ip %v: %v, line: %v", remote_ip_str, err, line)
	}

	state := stateMap[parts[3]]
	inode := parts[9]

	return &Connection{
		State: state,
		Type:  typ,
		Inode: inode,
		Ip:    ip,
		Port:  port,
		Fip:   remote_ip,
		Fport: remote_port,
	}, nil
}

func parseHex(h string) (res int, err error) {
	var res64 int64
	res64, err = strconv.ParseInt(h, 16, 32)
	res = int(res64)
	return
}

func parseIp(ip string) (string, error) {
	switch len(ip) {
	case 32:
		i := []string{
			ip[30:32],
			ip[28:30],
			ip[26:28],
			ip[24:26],
			ip[22:24],
			ip[20:22],
			ip[18:20],
			ip[16:18],
			ip[14:16],
			ip[12:14],
			ip[10:12],
			ip[8:10],
			ip[6:8],
			ip[4:6],
			ip[2:4],
			ip[0:2],
		}
		return fmt.Sprintf("%v%v:%v%v:%v%v:%v%v:%v%v:%v%v:%v%v:%v%v",
				i[14], i[15], i[12], i[13],
				i[10], i[11], i[8], i[9],
				i[6], i[7], i[4], i[5],
				i[2], i[3], i[0], i[1]),
			nil
	case 8:
		parts := make([]int, 0, 4)
		for i := 0; i < 8; i += 2 {
			part, err := parseHex(ip[i : i+2])
			parts = append(parts, part)
			if err != nil {
				return "", fmt.Errorf("Wrong ipv4 address format in %v: %v", ip, err)
			}
		}
		return fmt.Sprintf("%v.%v.%v.%v", parts[3], parts[2], parts[1], parts[0]), nil
	}
	return "", fmt.Errorf("IP address illegal length: %v", ip)
}
