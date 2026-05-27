package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/raychao-oao/entra-mcp/internal/graph"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

// Register adds all MVP1 tools to the MCP server.
func Register(s *server.MCPServer, gc *graph.Client, engine *yamlengine.Engine) {
	registerUserTools(s, gc, engine)
	registerGroupTools(s, gc, engine)
	registerReportTools(s, gc, engine)
}

func toolText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: text},
		},
	}
}

func toolErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
		},
	}
}
