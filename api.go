package weekendns

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync/atomic"

	"github.com/mccutchen/weekendns/byteview"
)

var dialer = &net.Dialer{}

// authoritative root name servers
// https://www.iana.org/domains/root/servers
//
// may be overridden in tests
var rootNameServers = []string{
	"198.41.0.4",     // a.root-servers.net Verisign, Inc.
	"199.9.14.201",   // b.root-servers.net University of Southern California, Information Sciences Institute
	"192.33.4.12",    // c.root-servers.net Cogent Communications
	"199.7.91.13",    // d.root-servers.net University of Maryland
	"192.203.230.10", // e.root-servers.net NASA (Ames Research Center)
	"192.5.5.241",    // f.root-servers.net Internet Systems Consortium, Inc.
	"192.112.36.4",   // g.root-servers.net US Department of Defense (NIC)
	"198.97.190.53",  // h.root-servers.net US Army (Research Lab)
	"192.36.148.17",  // i.root-servers.net Netnod
	"192.58.128.30",  // j.root-servers.net Verisign, Inc.
	"193.0.14.129",   // k.root-servers.net RIPE NCC
	"199.7.83.42",    // l.root-servers.net ICANN
	"202.12.27.33",   // m.root-servers.net WIDE Project
}

// Resolve recursively resolves the given domain name, returning the resolved
// IP address, the parsed DNS Message, and an error.
func Resolve(ctx context.Context, domainName string) ([]net.IP, error) {
	nameserver := chooseNameServer()
	for {
		log.Printf("querying nameserver %q for domain %q", nameserver, domainName)
		msg, err := sendQuery(ctx, nameserver, domainName, ResourceTypeA)
		if err != nil {
			return nil, err
		}

		// successfully resolved IP address, we're done
		ips, err := ipAddrsFromRecords(msg.Answers)
		if err != nil {
			return nil, err
		}
		if len(ips) > 0 {
			return ips, nil
		}

		// resolve again with a new name server from the response
		nsIPs, err := ipAddrsFromRecords(msg.Additionals)
		if err != nil {
			return nil, err
		}
		if len(nsIPs) > 0 {
			nameserver = nsIPs[0].String()
			continue
		}

		// first resolve nameserver domain to nameserver IP, then recurse with
		// new nameserver IP
		if nsDomains := findNSDomains(msg); len(nsDomains) > 0 {
			nsDomain := nsDomains[0]
			nextNSAddrs, err := Resolve(ctx, nsDomain)
			if err != nil {
				return nil, fmt.Errorf("error resolving nameserver %q: %w", nsDomain, err)
			}
			if len(nextNSAddrs) > 0 {
				nameserver = nextNSAddrs[0].String()
				continue
			}
		}

		return nil, fmt.Errorf("failed to resolve %s to an IP", domainName)
	}
}

func sendQuery(ctx context.Context, dst string, domainName string, resourceType ResourceType) (Message, error) {
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(dst, "53"))
	if err != nil {
		return Message{}, err
	}

	query := NewQuery(domainName, resourceType)
	if _, err := conn.Write(query.Encode()); err != nil {
		return Message{}, err
	}

	buf := make([]byte, maxMessageSize)
	n, err := conn.Read(buf)
	if err != nil {
		return Message{}, err
	}

	msg, err := parseMessage(byteview.New(buf[:n]))
	if err != nil {
		return Message{}, err
	}

	return msg, nil
}

func ipAddrsFromRecords(records []Record) ([]net.IP, error) {
	results := make([]net.IP, 0, len(records))
	for _, r := range records {
		if r.Type == ResourceTypeA {
			ips, err := parseIPAddrs(r.Data)
			if err != nil {
				return nil, err
			}
			results = append(results, ips...)
		}
	}
	return results, nil
}

func findNSDomains(msg Message) []string {
	results := make([]string, 0, len(msg.Authorities))
	for _, a := range msg.Authorities {
		if a.Type == ResourceTypeNS {
			results = append(results, string(a.Data))
		}
	}
	return results
}

// currentNameServer is an index into rootNameServers, used to choose a name
// server in round-robin fashion.
var currentNameServer = &atomic.Int32{}

// chooseNameServer chooses an authoritative root name server in round-robin
// fashion.
func chooseNameServer() string {
	idx := currentNameServer.Add(1)
	return rootNameServers[int(idx)%len(rootNameServers)]
}
