package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strings"

	"learning-go/aggregator/internal/aggregator"
	"learning-go/aggregator/internal/apisim"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	baseURL := flag.String("base-url", "", "base URL for downstream APIs (defaults to self)")
	flag.Parse()

	derivedBaseURL := *baseURL
	if derivedBaseURL == "" {
		host, port, err := splitAddr(*addr)
		if err != nil {
			log.Fatalf("invalid -addr %q: %v", *addr, err)
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		derivedBaseURL = "http://" + net.JoinHostPort(host, port)
	}

	mux := http.NewServeMux()
	apisim.Register(mux, apisim.DefaultConfig())

	client := aggregator.NewClient(derivedBaseURL)
	mux.Handle("/aggregate", aggregator.NewHandler(client))

	log.Printf("listening on %s", *addr)
	log.Printf("downstreams at %s", derivedBaseURL)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func splitAddr(addr string) (host string, port string, err error) {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1", strings.TrimPrefix(addr, ":"), nil
	}
	return net.SplitHostPort(addr)
}
