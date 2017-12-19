package zgrab2

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/ajholland/zflags"
)

var parser *flags.Parser

func init() {
	parser = flags.NewParser(&config, flags.Default)
}

// NewIniParser creates and returns a ini parser initialized
// with the default parser
func NewIniParser() *flags.IniParser {
	return flags.NewIniParser(parser)
}

// AddCommand adds a module to the parser and returns a pointer to
// a flags.command object or an error
func AddCommand(command string, shortDescription string, longDescription string, port int, m ScanModule) (*flags.Command, error) {
	cmd, err := parser.AddCommand(command, shortDescription, longDescription, m)
	if err != nil {
		return nil, err
	}
	cmd.FindOptionByLongName("port").Default = []string{strconv.FormatUint(uint64(port), 10)}
	cmd.FindOptionByLongName("name").Default = []string{command}
	modules[command] = &m
	return cmd, nil
}

// ParseCommandLine parses the commands given on the command line
// and validates the framework configuration (global options)
// immediately after parsing
func ParseCommandLine(flags []string) ([]string, string, ScanFlags, error) {
	posArgs, moduleType, f, err := parser.ParseCommandLine(flags)
	if err == nil {
		validateFrameworkConfiguration()
	}
	sf, _ := f.(ScanFlags)
	return posArgs, moduleType, sf, err
}

// ParseTarget takes input as a string and parses it into either an IPNet
// (may have empty mask and just contain IP , domain name, or errors, may
// return both IPNet and domain name
func ParseTarget(s string) (*net.IPNet, string, error) {
	i := strings.IndexByte(s, ',')
	j := strings.IndexByte(s, '/')

	switch {
	case i == -1 && j == -1:
		// just ip or domain
		if ip := net.ParseIP(s); ip != nil {
			return &net.IPNet{IP: ip}, "", nil
		}
		ips, err := net.LookupIP(s)
		if err != nil {
			return nil, "", err
		}
		return &net.IPNet{IP: ips[0]}, s, nil // only return first IP after a lookup
	case i == -1:
		// cidr block
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			return nil, "", err
		}
		return ipnet, "", nil
	case j == -1:
		// ip,domain
		str := strings.Split(s, ",")
		if len(str) != 2 {
			return nil, "", errors.New("malformed input")
		}
		d := strings.TrimSpace(str[1])
		if ip := net.ParseIP(str[0]); ip != nil {
			return &net.IPNet{IP: ip}, d, nil
		}
		return nil, d, nil
	}
	return nil, "", nil
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func duplicateIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

// Given an error object thrown by a scan, attempt to get the appropriate ScanStatus enum value
func TryGetScanStatus(err error) ScanStatus {
	if err == nil {
		return SCAN_SUCCESS
	}
	switch e := err.(type) {
	case *net.OpError:
		switch e.Op {
		case "dial":
			// TODO: Distinguish connection timeout / connection refused
			// Windows examples:
			//	"dial tcp 192.168.30.3:22: connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond."
			//	"dial tcp 127.0.0.1:22: connectex: No connection could be made because the target machine actively refused it."
			return SCAN_CONNECTION_TIMEOUT
		case "read":
			// TODO: Distinguish connection reset vs timeout
			return SCAN_IO_TIMEOUT
		case "write":
			// TODO: Distinguish connection reset vs timeout
			return SCAN_IO_TIMEOUT
		default:
			// TODO: Do we need a generic network error?
			return SCAN_UNKNOWN_ERROR
		}
	// TODO: More error types
	default:
		return SCAN_UNKNOWN_ERROR
	}
}
