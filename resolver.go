package dnstoy

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mccutchen/dnstoy/internal/byteview"
	"golang.org/x/exp/slog"
)

// authoritative root name servers
// https://www.iana.org/domains/root/servers
//
// may be overridden in tests
var defaultRootNameServers = []nameServerDef{
	newNameServerDef("a.root-servers.net", ".", net.ParseIP("198.41.0.4")),     // Verisign, Inc.
	newNameServerDef("b.root-servers.net", ".", net.ParseIP("199.9.14.201")),   // University of Southern California, Information Sciences Institute
	newNameServerDef("c.root-servers.net", ".", net.ParseIP("192.33.4.12")),    // Cogent Communications
	newNameServerDef("d.root-servers.net", ".", net.ParseIP("199.7.91.13")),    // University of Maryland
	newNameServerDef("e.root-servers.net", ".", net.ParseIP("192.203.230.10")), // NASA (Ames Research Center)
	newNameServerDef("f.root-servers.net", ".", net.ParseIP("192.5.5.241")),    // Internet Systems Consortium, Inc.
	newNameServerDef("g.root-servers.net", ".", net.ParseIP("192.112.36.4")),   // US Department of Defense (NIC)
	newNameServerDef("h.root-servers.net", ".", net.ParseIP("198.97.190.53")),  // US Army (Research Lab)
	newNameServerDef("i.root-servers.net", ".", net.ParseIP("192.36.148.17")),  // Netnod
	newNameServerDef("j.root-servers.net", ".", net.ParseIP("192.58.128.30")),  // Verisign, Inc.
	newNameServerDef("k.root-servers.net", ".", net.ParseIP("193.0.14.129")),   // RIPE NCC
	newNameServerDef("l.root-servers.net", ".", net.ParseIP("199.7.83.42")),    // ICANN
	newNameServerDef("m.root-servers.net", ".", net.ParseIP("202.12.27.33")),   // WIDE Project
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
	if opts.QueryTimeout == 0 {
		opts.QueryTimeout = 1 * time.Second
	}
	return &Resolver{
		rootNameServers: opts.RootNameServers,
		queryTimeout:    opts.QueryTimeout,
		dialer:          opts.Dialer,
		logger:          opts.Logger,
	}
}

// Opts defines the options used to configure a Resolver.
type Opts struct {
	RootNameServers []nameServerDef
	QueryTimeout    time.Duration
	Dialer          *net.Dialer
	Logger          *slog.Logger
}

// Resolver makes DNS queries.
type Resolver struct {
	rootNameServers []nameServerDef
	queryTimeout    time.Duration
	dialer          *net.Dialer
	logger          *slog.Logger
}

// LookupIP recursively resolves the given domain name, returning the resolved
// IP addresses.
func (r *Resolver) LookupIP(ctx context.Context, domainName string) ([]net.IP, error) {
	result, _, err := r.doLookupIP(ctx, r.chooseRootNameServer(), domainName, 0)
	return result, err
}

func (r *Resolver) doLookupIP(ctx context.Context, nameServer nameServerDef, domainName string, depth int) ([]net.IP, int, error) {
	msg, err := r.sendQuery(ctx, nameServer, domainName, RecordTypeA, depth)
	if err != nil {
		return nil, depth, err
	}

	r.logRecords("answer", msg.Answers)
	r.logRecords("authority", msg.Authorities)
	r.logRecords("additional", msg.Additionals)

	// if we successfully resolved IP address(es), we're done
	ips, err := ipAddrsFromRecords(msg.Answers)
	if err != nil {
		return nil, depth, err
	}
	if len(ips) > 0 {
		return ips, depth, nil
	}

	// if we find glue NS records, re-resolve again with a new name server
	if glue, err := getGlueNameServers(msg); err != nil {
		return nil, depth, fmt.Errorf("failed to get glue nameservers: %w", err)
	} else if len(glue) > 0 {
		nameServer = randomChoice(glue)
		r.logger.Debug(
			"recursively resolving with new name server from glue records",
			slog.String("query_name", domainName),
			slog.String("ns_name", nameServer.name),
			slog.String("ns_addr", nameServer.addr.String()),
			slog.String("ns_authority", nameServer.authority),
			slog.Int("depth", depth),
		)
		return r.doLookupIP(ctx, nameServer, domainName, depth+1)
	}

	// if we find NS records but no glue records, we must first resolve
	// nameserver domain to nameserver IP, then recurse with new nameserver IP
	if ns, found := matchRecord(msg.Authorities, RecordTypeNS); found {
		nsDomain := string(ns.Data)
		r.logger.Debug(
			"resolving NS domain",
			slog.String("ns_domain", nsDomain),
			slog.Int("depth", depth),
		)
		nextNSAddrs, newDepth, err := r.doLookupIP(ctx, r.chooseRootNameServer(), nsDomain, depth+1)
		if err != nil {
			return nil, newDepth, fmt.Errorf("error resolving nameserver: %w", err)
		}
		if len(nextNSAddrs) == 0 {
			return nil, newDepth, fmt.Errorf("no IP addresses found for nameserver %q", nsDomain)
		}
		for _, nsAddr := range nextNSAddrs {
			if nsAddr.IsPrivate() {
				r.logger.Debug("skipping private name server", slog.String("ns_addr", nsAddr.String()))
				continue
			}
			nameServer = newNameServerDef(nsDomain, string(ns.Name), nsAddr)
			r.logger.Debug(
				"recursively resolving with new name server",
				slog.String("query_domain", domainName),
				slog.String("ns_name", nameServer.name),
				slog.String("ns_addr", nameServer.addr.String()),
				slog.String("ns_authority", nameServer.authority),
				slog.Int("depth", depth),
			)
			return r.doLookupIP(ctx, nameServer, domainName, newDepth+1)
		}
	}

	// finally, if we find a CNAME, recursively resolve it instead of our
	// current query
	if cname, found := matchRecord(msg.Answers, RecordTypeCNAME); found {
		cnameDomain := string(cname.Data)
		r.logger.Debug(
			"recursively resolving CNAME",
			slog.String("cname", cnameDomain),
			slog.String("query_name", domainName),
			slog.Int("depth", depth),
		)
		return r.doLookupIP(ctx, nameServer, cnameDomain, depth+1)
	}

	r.logger.Debug(
		"no IP addresses found",
		slog.String("query_name", domainName),
		slog.String("msg", fmt.Sprintf("%#v", msg)),
	)
	return nil, depth, fmt.Errorf("failed to resolve %s to an IP", domainName)
}

// sendQuery sends a query to a name server and parses the response.
func (r *Resolver) sendQuery(ctx context.Context, nameServer nameServerDef, targetDomain string, recordType RecordType, depth int) (Message, error) {
	conn, err := r.dialer.DialContext(ctx, "udp", net.JoinHostPort(nameServer.addr.String(), "53"))
	if err != nil {
		return Message{}, fmt.Errorf("failed to dial nameserver %s: %w", nameServer.name, err)
	}
	conn.SetDeadline(time.Now().Add(r.queryTimeout))

	r.logger.Debug(
		"sending DNS query",
		slog.String("query_name", targetDomain),
		slog.String("ns_name", nameServer.name),
		slog.String("ns_addr", nameServer.addr.String()),
		slog.String("ns_authority", nameServer.authority),
		slog.String("resource_type", recordType.String()),
		slog.Int("depth", depth),
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
	// r.logger.Debug("raw DNS response bytes", slog.String("resp_bytes", string(resp)))

	msg, err := parseMessage(byteview.New(resp))
	if err != nil {
		r.logger.Debug(
			"failed to parse DNS response",
			slog.String("err", err.Error()),
			slog.String("query_name", targetDomain),
			slog.String("ns_name", nameServer.name),
			slog.String("ns_addr", nameServer.addr.String()),
			slog.String("ns_authority", nameServer.authority),
			slog.String("resource_type", recordType.String()),
			slog.Int("depth", depth),
		)
		return Message{}, err
	}

	return msg, nil
}

// chooseRootNameServer chooses an authoritative root name server in round-robin
// fashion.
func (r *Resolver) chooseRootNameServer() nameServerDef {
	return randomChoice(r.rootNameServers)
}

func (r *Resolver) logRecords(section string, records []Record) {
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
			slog.String("section", section),
			slog.String("name", string(a.Name)),
			slog.String("type", a.Type.String()),
			slog.String("value", val),
		)
	}
}

// getGlueNameServers joins the glue records in the additional section with the
// name servers in the authority section.
func getGlueNameServers(msg Message) ([]nameServerDef, error) {
	if len(msg.Additionals) == 0 {
		return nil, nil
	}

	authorityIdx := make(map[string]int)
	for i, a := range msg.Authorities {
		authorityIdx[string(a.Data)] = i
	}

	results := make([]nameServerDef, 0, len(msg.Additionals))
	for _, a := range msg.Additionals {
		if a.Type != RecordTypeA && a.Type != RecordTypeAAAA {
			return nil, fmt.Errorf("unexpected record type %s (%v) in additional section", a.Type, a.Type)
		}

		idx, found := authorityIdx[string(a.Name)]
		if !found {
			return nil, fmt.Errorf("no authority found for %q", a.Name)
		}

		addrs, err := parseIPAddrs(a.Type, a.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse glue IP address: %w", err)
		}

		authority := msg.Authorities[idx]
		ns := newNameServerDef(string(authority.Data), string(authority.Name), addrs[0])
		results = append(results, ns)
	}
	return results, nil
}

// matchRecord returns the first record of the given type in the given slice.
func matchRecord(records []Record, recordType RecordType) (Record, bool) {
	for _, r := range records {
		if r.Type == recordType {
			return r, true
		}
	}
	return Record{}, false
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

// randomChoice returns a random element from the given slice.
func randomChoice[T any](choices []T) T {
	return choices[rand.Intn(len(choices))]
}

type nameServerDef struct {
	name      string
	addr      net.IP
	authority string
}

func newNameServerDef(name string, authority string, addr net.IP) nameServerDef {
	return nameServerDef{addr: addr, name: name, authority: authority}
}
