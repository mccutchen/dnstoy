package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/slog"

	"github.com/mccutchen/dnstoy"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging")
	timeout := flag.Duration("timeout", 5*time.Second, "Timeout for DNS queries")
	flag.Parse()

	var domains []string
	if flag.NArg() > 0 {
		domains = flag.Args()
	} else {
		// use a default set of domains to exercise DNS resolution
		domains = []string{
			"example.com",
			"facebook.com",
			"google.com",
			"twitter.com",
		}
		for _, domain := range domains {
			domains = append(domains, "www."+domain)
		}
	}

	logLevel := slog.LevelInfo
	if isDebugEnabled(*debug) {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	resolver := dnstoy.New(&dnstoy.Opts{
		Logger: logger,
		Dialer: &net.Dialer{
			Timeout: *timeout,
		},
		QueryTimeout: *timeout,
	})

	for _, domain := range domains {
		fmt.Printf("\nresolving %s ...\n", domain)
		ips, err := resolver.LookupIP(context.Background(), domain)
		if err != nil {
			fmt.Printf("error resolving %s: %s\n", domain, err)
			continue
		}
		fmt.Printf("%s resolves to: %s\n", domain, ips)
	}
}

func isDebugEnabled(debugFlag bool) bool {
	debugEnv := strings.ToLower(os.Getenv("DEBUG"))
	return debugFlag || (debugEnv != "" && debugEnv != "0" && debugEnv != "false")
}
