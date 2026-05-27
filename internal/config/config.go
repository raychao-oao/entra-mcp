package config

import (
	"fmt"
	"os"
)

type Config struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	Addr         string
	PolicyFile   string
}

func Load() (*Config, error) {
	cfg := &Config{
		TenantID:     os.Getenv("ENTRA_MCP_TENANT_ID"),
		ClientID:     os.Getenv("ENTRA_MCP_CLIENT_ID"),
		ClientSecret: os.Getenv("ENTRA_MCP_CLIENT_SECRET"),
		Addr:         os.Getenv("ENTRA_MCP_ADDR"),
		PolicyFile:   os.Getenv("ENTRA_MCP_POLICY_FILE"),
	}
	if cfg.TenantID == "" {
		return nil, fmt.Errorf("ENTRA_MCP_TENANT_ID is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("ENTRA_MCP_CLIENT_ID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("ENTRA_MCP_CLIENT_SECRET is required")
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.PolicyFile == "" {
		cfg.PolicyFile = "/etc/entra-mcp/policy.yaml"
	}
	return cfg, nil
}
