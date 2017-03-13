package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const magic = "SIP/2.0"

type SipPacket struct {
	HeaderLine string

	RequestVerb  string
	ResponseCode int
}

func (sip *SipPacket) String() string {
	if sip.RequestVerb == "" {
		return fmt.Sprintf("Response %v", sip.ResponseCode)
	} else {
		return fmt.Sprintf("Request %v", sip.RequestVerb)
	}
}

func ParseSipPacket(payload []byte) (*SipPacket, error) {
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Scan()
	line := scanner.Text()
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, nil
	}
	if parts[0] == magic {
		if len(parts) < 2 {
			return nil, fmt.Errorf("SIP header line too short: %v", line)
		}
		code, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Failed to parse response code: %v (Header: %v)", err, line)
		}
		return &SipPacket{
			HeaderLine:   line,
			ResponseCode: code,
		}, nil
	} else if parts[len(parts)-1] == magic {
		if len(parts) < 2 {
			return nil, fmt.Errorf("SIP header line too short: %v", line)
		}
		return &SipPacket{
			HeaderLine:  line,
			RequestVerb: parts[0],
		}, nil
	} else {
		return nil, nil
	}
}
