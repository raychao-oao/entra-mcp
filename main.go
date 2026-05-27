package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/raychao-oao/entra-mcp/internal/config"
	"github.com/raychao-oao/entra-mcp/internal/graph"
	"github.com/raychao-oao/entra-mcp/internal/tools"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

var Version = "0.1.0"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	gc, err := graph.NewClient(cfg.TenantID, cfg.ClientID, cfg.ClientSecret)
	if err != nil {
		log.Fatalf("graph client: %v", err)
	}

	var engine *yamlengine.Engine
	if cfg.PolicyFile != "" {
		if _, statErr := os.Stat(cfg.PolicyFile); statErr == nil {
			engine, err = yamlengine.LoadFile(cfg.PolicyFile)
			if err != nil {
				log.Fatalf("policy: %v", err)
			}
			log.Printf("policy loaded from %s", cfg.PolicyFile)
		} else {
			log.Printf("policy file not found (%s), running without policy enforcement", cfg.PolicyFile)
		}
	}

	s := server.NewMCPServer(
		"entra-mcp",
		Version,
		server.WithToolCapabilities(false),
	)

	tools.Register(s, gc, engine)

	log.Printf("entra-mcp %s listening on %s", Version, cfg.Addr)

	httpServer := server.NewStreamableHTTPServer(s)
	if err := httpServer.Start(cfg.Addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
