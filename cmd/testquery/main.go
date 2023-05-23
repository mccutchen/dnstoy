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
	msg, err := weekendns.SendQuery("8.8.8.8", domainName, weekendns.QueryTypeA)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(msg)
	fmt.Println(weekendns.FormatIP(msg.Answers[0].Data))
}
