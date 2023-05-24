package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mccutchen/weekendns"
)

func main() {
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")

	domains := []string{
		"example.com",
		"facebook.com",
		"google.com",
		"twitter.com",
	}
	for _, domain := range domains {
		domains = append(domains, "www."+domain)
	}
	for _, domain := range domains {
		fmt.Printf("\nresolving %s\n", domain)
		ip, msg, err := weekendns.Resolve(context.Background(), domain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving %q: %s\n", domain, err)
			continue
		}
		enc.Encode(msg)
		fmt.Printf("%s resolves to: %s\n", domain, ip)
	}
}
