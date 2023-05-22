package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/mccutchen/weekendns"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s DOMAIN", os.Args[0])
		os.Exit(1)
	}

	domainName := os.Args[1]

	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		log.Fatalf("error connecting to Google DNS: %s", err)
	}

	query := weekendns.NewQuery(domainName, weekendns.QueryTypeA)
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

	fmt.Println(weekendns.FormatIP(msg.Answers[0].Data))
}
