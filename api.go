package weekendns

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/mccutchen/weekendns/byteview"
)

var dialer = &net.Dialer{}

// Resolve recursively resolves the given domain name, returning the resolved
// IP address, the parsed DNS Message, and an error.
func Resolve(ctx context.Context, domainName string) (string, Message, error) {
	nameserver := "198.41.0.4"
	for {
		log.Printf("querying nameserver %q for domain %q", nameserver, domainName)
		msg, err := sendQuery(ctx, nameserver, domainName, ResourceTypeA)
		if err != nil {
			return "", Message{}, err
		}

		// successfully resolved IP address, we're done
		if ip := getAnswer(msg); ip != "" {
			return ip, msg, nil
		}

		// recurse with new nameserver IP from the response
		if nsIP := getNameserverIP(msg); nsIP != "" {
			nameserver = nsIP
			continue
		}

		// first resolve nameserver domain to nameserver IP, then recurse with
		// new nameserver IP
		if nsDomain := getNameserverDomain(msg); nsDomain != "" {
			nextNameserver, _, err := Resolve(ctx, nsDomain)
			if err != nil {
				return "", msg, err
			}
			nameserver = nextNameserver
			continue
		}

		return "", msg, fmt.Errorf("failed to resolve %s to an IP", domainName)
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
		log.Printf("error parsing message: %s", err)
		log.Printf("response: %q", string(buf[:n]))
		return Message{}, err
	}

	return msg, nil
}

func getAnswer(msg Message) string {
	for _, a := range msg.Answers {
		if a.Type == ResourceTypeA {
			return formatIP(a.Data)
		}
	}
	return ""
}

func getNameserverIP(msg Message) string {
	for _, a := range msg.Additionals {
		if a.Type == ResourceTypeA {
			return formatIP(a.Data)
		}
	}
	return ""
}

func getNameserverDomain(msg Message) string {
	for _, a := range msg.Authorities {
		if a.Type == ResourceTypeNS {
			return string(a.Data)
		}
	}
	return ""
}
