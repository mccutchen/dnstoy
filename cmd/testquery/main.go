package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"golang.org/x/exp/slog"

	"github.com/mccutchen/weekendns"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Parse()

	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

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

	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")

	for _, domain := range domains {
		fmt.Printf("\nresolving %s\n", domain)
		ips, err := weekendns.Resolve(context.Background(), domain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving %q: %s\n", domain, err)
			continue
		}
		fmt.Printf("%s resolves to: %s\n", domain, ips)
	}
}
