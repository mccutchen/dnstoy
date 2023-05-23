package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/mccutchen/weekendns"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s DOMAIN", os.Args[0])
		os.Exit(1)
	}

	domainName := os.Args[1]
	ip, msg, err := weekendns.Resolve(domainName, weekendns.ResourceTypeA)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(msg)
	fmt.Println(ip)
}
