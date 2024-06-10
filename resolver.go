package dnstoy

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"

	"github.com/mccutchen/dnstoy/internal/byteview"
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
		rootNameServers:   opts.RootNameServers,
		dialer:            opts.Dialer,
		logger:            opts.Logger,
		rootNameServerIdx: &atomic.Int32{},
	}
}

// Opts defines the options used to configure a Resolver.
type Opts struct {
	RootNameServers []string
	Dialer          *net.Dialer
	Logger          *slog.Logger
}

// Resolver makes DNS queries.
type Resolver struct {
	rootNameServers []string
	dialer          *net.Dialer
	logger          *slog.Logger

	// index into rootNameServers, used for round-robin choice
	rootNameServerIdx *atomic.Int32
}

// LookupIP recursively resolves the given domain name, returning the resolved
// IP addresses.
func (r *Resolver) LookupIP(ctx context.Context, domainName string) ([]net.IP, error) {
	result, _, err := r.doLookupIP(ctx, r.chooseRootNameServer(), domainName, 0)
	return result, err
}

func (r *Resolver) doLookupIP(ctx context.Context, nameServer string, domainName string, depth int) ([]net.IP, int, error) {
	msg, err := r.sendQuery(ctx, nameServer, domainName, RecordTypeA, depth)
	if err != nil {
		return nil, depth, err
	}

	r.logRecords("answer", msg.Answers, depth)
	r.logRecords("additional", msg.Additionals, depth)
	r.logRecords("authority", msg.Authorities, depth)

	// successfully resolved IP address, we're done
	ips, err := ipAddrsFromRecords(msg.Answers)
	if err != nil {
		return nil, depth, err
	}
	if len(ips) > 0 {
		return ips, depth, nil
	}

	// resolve again with a new name server from the response
	nsIPs, err := ipAddrsFromRecords(msg.Additionals)
	if err != nil {
		return nil, depth, err
	}
	if len(nsIPs) > 0 {
		nameServer = randomChoice(nsIPs).String()
		r.logger.Debug(
			"recursively resolving with new name server",
			slog.Int("depth", depth),
			slog.String("ns_addr", nameServer),
			slog.Int("ns_addr_count", len(nsIPs)),
			slog.String("target_domain", domainName),
		)
		return r.doLookupIP(ctx, nameServer, domainName, depth+1)
	}

	// first resolve nameserver domain to nameserver IP, then recurse with
	// new nameserver IP
	if nsDomains := domainsFromRecords(msg.Authorities, RecordTypeNS); len(nsDomains) > 0 {
		nsDomain := randomChoice(nsDomains)
		r.logger.Debug(
			"resolving NS domain",
			slog.Int("depth", depth),
			slog.String("ns_domain", nsDomain),
			slog.Int("ns_domain_count", len(nsDomains)),
			slog.String("target_domain", domainName),
		)
		nextNSAddrs, newDepth, err := r.doLookupIP(ctx, nameServer, nsDomain, depth+1)
		if err != nil {
			return nil, newDepth, fmt.Errorf("error resolving nameserver: %w", err)
		}
		if len(nextNSAddrs) > 0 {
			for _, nsAddr := range nextNSAddrs {
				if nsAddr.IsPrivate() {
					r.logger.Debug("skipping private name server", slog.String("ns_addr", nsAddr.String()))
					continue
				}
				nameServer = nsAddr.String()
				r.logger.Debug(
					"recursively resolving with new name server",
					slog.Int("depth", depth),
					slog.String("target_domain", domainName),
					slog.String("ns_addr", nameServer),
					slog.Int("ns_addr_count", len(nextNSAddrs)),
				)
				return r.doLookupIP(ctx, nameServer, domainName, newDepth+1)
			}
		}
	}

	if cnameDomains := domainsFromRecords(msg.Answers, RecordTypeCNAME); len(cnameDomains) > 0 {
		cnameDomain := randomChoice(cnameDomains)
		r.logger.Debug(
			"recursively resolving CNAME",
			slog.Int("depth", depth),
			slog.String("cname", cnameDomain),
			slog.Int("cname_count", len(cnameDomains)),
			slog.String("target_domain", domainName),
		)
		return r.doLookupIP(ctx, nameServer, cnameDomain, depth+1)
	}

	r.logger.Debug(
		"no IP addresses found",
		slog.String("target_domain", domainName),
		slog.String("msg", fmt.Sprintf("%#v", msg)),
	)
	return nil, depth, fmt.Errorf("failed to resolve %s to an IP", domainName)
}

// sendQuery queries a DNS server for a domain and parses the response.
func (r *Resolver) sendQuery(ctx context.Context, nameserver string, targetDomain string, recordType RecordType, depth int) (Message, error) {
	nsAddr := net.JoinHostPort(nameserver, "53")
	conn, err := r.dialer.DialContext(ctx, "udp", nsAddr)
	if err != nil {
		return Message{}, fmt.Errorf("failed to dial nameserver %q: %w", nsAddr, err)
	}

	r.logger.Debug(
		"sending DNS query to nameserver",
		slog.Int("depth", depth),
		slog.String("nameserver", nameserver),
		slog.String("target_domain", targetDomain),
		slog.String("resource_type", recordType.String()),
	)

	query := NewQuery(targetDomain, recordType)
	if _, err := conn.Write(query.Encode()); err != nil {
		return Message{}, err
	}

	buf := make([]byte, maxMessageSize)
	n, err := conn.Read(buf)
	if err != nil {
		return Message{}, err
	}

	resp := buf[:n]
	r.logger.Debug("raw DNS response bytes", slog.String("resp_bytes", string(resp)))

	msg, err := parseMessage(byteview.New(resp))
	if err != nil {
		r.logger.Debug(
			"failed to parse DNS response",
			slog.Int("depth", depth),
			slog.String("err", err.Error()),
			slog.String("target_domain", targetDomain),
			slog.String("server", nameserver),
			slog.String("resource_type", recordType.String()),
		)
		return Message{}, err
	}

	return msg, nil
}

// chooseRootNameServer chooses an authoritative root name server in round-robin
// fashion.
func (r *Resolver) chooseRootNameServer() string {
	idx := r.rootNameServerIdx.Add(1)
	return r.rootNameServers[int(idx)%len(r.rootNameServers)]
}

func (r *Resolver) logRecords(section string, records []Record, depth int) {
	for _, a := range records {
		// try to log human-readable data instead of raw bytes where we know
		// what to expect for a given record type
		val := string(a.Data)
		if a.Type == RecordTypeA || a.Type == RecordTypeAAAA {
			ips, err := parseIPAddrs(a.Type, a.Data)
			if err == nil {
				val = fmt.Sprintf("%v", ips)
			}
		}

		r.logger.Debug(
			"resource record",
			slog.Int("depth", depth),
			slog.String("section", section),
			slog.String("name", string(a.Name)),
			slog.String("type", a.Type.String()),
			slog.String("value", val),
		)
	}
}

func ipAddrsFromRecords(records []Record) ([]net.IP, error) {
	results := make([]net.IP, 0, len(records))
	for _, r := range records {
		if r.Type == RecordTypeA || r.Type == RecordTypeAAAA {
			ips, err := parseIPAddrs(r.Type, r.Data)
			if err != nil {
				return nil, err
			}
			results = append(results, ips...)
		}
	}
	return results, nil
}

func domainsFromRecords(records []Record, targetRecordType RecordType) []string {
	results := make([]string, 0, len(records))
	for _, a := range records {
		if a.Type == targetRecordType {
			results = append(results, string(a.Data))
		}
	}
	return results
}

// randomChoice returns a random element from the given slice.
func randomChoice[T any](choices []T) T {
	return choices[rand.Intn(len(choices))]
}
