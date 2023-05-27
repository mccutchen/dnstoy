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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

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

	resolver := weekendns.New(&weekendns.Opts{
		Logger: logger,
	})

	for _, domain := range domains {
		fmt.Printf("\nresolving %s\n", domain)
		ips, err := resolver.Resolve(context.Background(), domain)
		if err != nil {
			logger.Error("error resolving domain", slog.String("domain", domain), slog.String("error", err.Error()))
			continue
		}
		fmt.Printf("%s resolves to: %s\n", domain, ips)
	}
}
