package main

import (
	"flag"
	"log"

	"github.com/marcusz/monitoring-assistant/internal/mockserver"
)

func main() {
	fixtureDir := flag.String("fixtures", "tests/evals/fixtures", "fixture directory")
	serverType := flag.String("server-type", "", "optional MCP server type fixture filter (e.g. grafana)")
	flag.Parse()

	server, err := mockserver.NewForServerType(*fixtureDir, *serverType)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.ServeStdio(); err != nil {
		log.Fatal(err)
	}
}
