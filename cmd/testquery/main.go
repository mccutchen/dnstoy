package main

import (
	"encoding/json"
	"log"
	"net"
	"os"

	"github.com/mccutchen/weekendns"
)

func main() {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		log.Fatalf("error connecting to Google DNS: %s", err)
	}

	query := weekendns.NewQuery("example.com", weekendns.QueryTypeA)
	if _, err := conn.Write(query.Encode()); err != nil {
		log.Fatalf("error writing query: %s", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("error reading DNS response: %s", err)
	}

	view := weekendns.NewByteView(buf[:n])
	msg, err := weekendns.ParseMessage(view)
	if err != nil {
		log.Fatalf("error parsing DNS message: %s", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(msg); err != nil {
		log.Fatalf("error encoding DNS message as JSON: %s", err)
	}
}
