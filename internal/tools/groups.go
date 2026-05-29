package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/raychao-oao/entra-mcp/internal/graph"
	entrapolicy "github.com/raychao-oao/entra-mcp/internal/policy"
	mcppolicy "github.com/raychao-oao/mcp-policy/pkg/policy"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

func registerGroupTools(s *server.MCPServer, gc *graph.Client, engine *yamlengine.Engine) {
	s.AddTool(mcp.NewTool("entra.search_group",
		mcp.WithDescription("Search Entra groups by display name. Min 3 chars. Max 50 results."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Group name fragment (min 3 chars)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.search_group", mcppolicy.Resource{Type: "entra:group", ID: "*"}, ""); err != nil {
			return toolErr(err), nil
		}
		query := req.GetString("query", "")
		groups, err := gc.SearchGroups(ctx, query, 50)
		if err != nil {
			return toolErr(err), nil
		}
		if len(groups) == 0 {
			return toolText(fmt.Sprintf("No groups found matching %q.", query)), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d group(s) matching %q:\n\n", len(groups), query))
		for _, g := range groups {
			sb.WriteString(fmt.Sprintf("- **%s** [%s] — ID: `%s`\n", g.DisplayName, classifyGroup(g), g.ID))
			if g.Description != "" {
				sb.WriteString(fmt.Sprintf("  Description: %s\n", g.Description))
			}
			if g.OnPremisesSyncEnabled {
				sb.WriteString("  _(synced from on-prem AD)_\n")
			}
			if g.IsAssignableToRole {
				sb.WriteString("  _(role-assignable)_\n")
			}
		}
		if len(groups) == 50 {
			sb.WriteString("\n_Results capped at 50. Refine your query._")
		}
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.get_group",
		mcp.WithDescription("Get details of an Entra group by object ID."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Group object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.get_group", mcppolicy.Resource{Type: "entra:group", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		g, err := gc.GetGroup(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## %s\n\n", g.DisplayName))
		sb.WriteString(fmt.Sprintf("**Type:** %s\n", classifyGroup(*g)))
		sb.WriteString(fmt.Sprintf("**Security enabled:** %v\n", g.SecurityEnabled))
		sb.WriteString(fmt.Sprintf("**Mail enabled:** %v\n", g.MailEnabled))
		sb.WriteString(fmt.Sprintf("**On-prem synced:** %v\n", g.OnPremisesSyncEnabled))
		sb.WriteString(fmt.Sprintf("**Role-assignable:** %v\n", g.IsAssignableToRole))
		if g.Description != "" {
			sb.WriteString(fmt.Sprintf("**Description:** %s\n", g.Description))
		}
		if g.OnPremisesSyncEnabled {
			sb.WriteString("\n> ⚠️ This group is managed in on-premises AD. Membership changes must use ad-mcp.\n")
		}
		if g.IsAssignableToRole {
			sb.WriteString("\n> ⚠️ This is a role-assignable group. Membership changes require a privileged workflow.\n")
		}
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.list_group_members",
		mcp.WithDescription("List direct members of an Entra group. Max 200. Note: service principals may be excluded."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Group object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.list_group_members", mcppolicy.Resource{Type: "entra:group", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		members, err := gc.ListGroupMembers(ctx, id, 200)
		if err != nil {
			return toolErr(err), nil
		}
		if len(members) == 0 {
			return toolText(fmt.Sprintf("No members found in group %s.", id)), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%d member(s) in group %s:\n\n", len(members), id))
		for _, u := range members {
			sb.WriteString(fmt.Sprintf("- **%s** (%s) | %s | %s\n",
				u.DisplayName, u.UserPrincipalName, u.Department, u.JobTitle))
		}
		if len(members) == 200 {
			sb.WriteString("\n_Results capped at 200. There may be additional members._")
		}
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.list_group_owners",
		mcp.WithDescription("List owners of an Entra group. Returns empty for distribution groups, Exchange-created groups, and synced groups."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Group object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.list_group_owners", mcppolicy.Resource{Type: "entra:group", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		owners, err := gc.ListGroupOwners(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		if len(owners) == 0 {
			return toolText(fmt.Sprintf("No owners found for group %s. (Distribution groups, Exchange-created groups, and synced groups do not expose owners via Graph API.)", id)), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%d owner(s) of group %s:\n\n", len(owners), id))
		for _, u := range owners {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", u.DisplayName, u.UserPrincipalName))
		}
		return toolText(sb.String()), nil
	})
}
