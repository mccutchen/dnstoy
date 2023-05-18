package main

import (
	"log"
	"net"

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
	header, err := weekendns.ParseHeader(view)
	if err != nil {
		log.Fatalf("error parsing header: %s", err)
	}
	log.Printf("header:   %#v", header)

	question, err := weekendns.ParseHeader(view)
	if err != nil {
		log.Fatalf("error parsing question: %s", err)
	}
	log.Printf("question: %#v", question)

	record, err := weekendns.ParseRecord(view)
	if err != nil {
		log.Fatalf("error parsing record: %s", err)
	}
	log.Printf("record:   %#v", record)
}
