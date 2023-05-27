package weekendns

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/mccutchen/weekendns/byteview"
	"golang.org/x/exp/slog"
)

// authoritative root name servers
// https://www.iana.org/domains/root/servers
//
// may be overridden in tests
var defaultRootNameServers = []string{
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

// New returns a new Resolver.
func New(opts *Opts) *Resolver {
	if opts == nil {
		opts = &Opts{}
	}
	if len(opts.RootNameServers) == 0 {
		opts.RootNameServers = defaultRootNameServers
	}
	if opts.Dialer == nil {
		opts.Dialer = &net.Dialer{}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Resolver{
		rootNameServers: opts.RootNameServers,
		dialer:          opts.Dialer,
		logger:          opts.Logger,
		nameServerIdx:   &atomic.Int32{},
	}
}

type Opts struct {
	RootNameServers []string
	Dialer          *net.Dialer
	Logger          *slog.Logger
}

type Resolver struct {
	rootNameServers []string
	dialer          *net.Dialer
	logger          *slog.Logger

	// index into rootNameServers, used for round-robin choice
	nameServerIdx *atomic.Int32
}

// Resolve recursively resolves the given domain name, returning the resolved
// IP address, the parsed DNS Message, and an error.
func (r *Resolver) Resolve(ctx context.Context, domainName string) ([]net.IP, error) {
	nameserver := r.chooseNameServer()
	for {
		r.logger.Info("querying nameserver for domain", slog.String("ns", nameserver), slog.String("domain", domainName))
		msg, err := r.sendQuery(ctx, nameserver, domainName, ResourceTypeA)
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
			nextNSAddrs, err := r.Resolve(ctx, nsDomain)
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

func (r *Resolver) sendQuery(ctx context.Context, dst string, domainName string, resourceType ResourceType) (Message, error) {
	conn, err := r.dialer.DialContext(ctx, "udp", net.JoinHostPort(dst, "53"))
	if err != nil {
		return Message{}, err
	}

	r.logger.Debug(
		"starting DNS query",
		slog.String("server", dst),
		slog.String("domain", domainName),
		slog.String("resource_type", resourceType.String()),
	)

	query := NewQuery(domainName, resourceType)
	if _, err := conn.Write(query.Encode()); err != nil {
		return Message{}, err
	}

	buf := make([]byte, maxMessageSize)
	n, err := conn.Read(buf)
	if err != nil {
		return Message{}, err
	}

	resp := buf[:n]
	r.logger.Debug("received DNS response", slog.String("response_bytes", string(resp)))

	msg, err := parseMessage(byteview.New(resp))
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

// chooseNameServer chooses an authoritative root name server in round-robin
// fashion.
func (r *Resolver) chooseNameServer() string {
	idx := r.nameServerIdx.Add(1)
	return r.rootNameServers[int(idx)%len(r.rootNameServers)]
}
