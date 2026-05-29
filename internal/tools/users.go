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

func registerUserTools(s *server.MCPServer, gc *graph.Client, engine *yamlengine.Engine) {
	s.AddTool(mcp.NewTool("entra.search_user",
		mcp.WithDescription("Search Entra users by name, email, or UPN. Min 3 chars. Max 50 results."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Name, email, or UPN fragment (min 3 chars)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.search_user", mcppolicy.Resource{Type: "entra:user", ID: "*"}, ""); err != nil {
			return toolErr(err), nil
		}
		query := req.GetString("query", "")
		users, err := gc.SearchUsers(ctx, query, 50)
		if err != nil {
			return toolErr(err), nil
		}
		if len(users) == 0 {
			return toolText(fmt.Sprintf("No users found matching %q.", query)), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d user(s) matching %q:\n\n", len(users), query))
		for _, u := range users {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", u.DisplayName, u.UserPrincipalName))
			sb.WriteString(fmt.Sprintf("  Department: %s | Title: %s | Type: %s | Enabled: %v | Synced: %v\n",
				u.Department, u.JobTitle, u.UserType, u.AccountEnabled, u.OnPremisesSyncEnabled))
		}
		if len(users) == 50 {
			sb.WriteString("\n_Results capped at 50. Refine your query for more specific results._")
		}
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.get_user",
		mcp.WithDescription("Get full Entra user profile by UPN or object ID. Sign-in timestamps require AuditLog.Read.All + Entra ID P1/P2."),
		mcp.WithString("id", mcp.Required(), mcp.Description("User UPN (alice@contoso.com) or object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.get_user", mcppolicy.Resource{Type: "entra:user", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		u, err := gc.GetUser(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## %s\n\n", u.DisplayName))
		sb.WriteString(fmt.Sprintf("**UPN:** %s\n", u.UserPrincipalName))
		sb.WriteString(fmt.Sprintf("**Mail:** %s\n", u.Mail))
		sb.WriteString(fmt.Sprintf("**Account enabled:** %v\n", u.AccountEnabled))
		sb.WriteString(fmt.Sprintf("**User type:** %s\n", u.UserType))
		sb.WriteString(fmt.Sprintf("**Department:** %s\n", u.Department))
		sb.WriteString(fmt.Sprintf("**Job title:** %s\n", u.JobTitle))
		sb.WriteString(fmt.Sprintf("**On-prem synced:** %v\n", u.OnPremisesSyncEnabled))
		if u.LastSuccessfulSignIn != "" {
			sb.WriteString(fmt.Sprintf("**Last successful sign-in:** %s\n", u.LastSuccessfulSignIn))
		} else {
			sb.WriteString("**Last successful sign-in:** not available (requires P1/P2)\n")
		}
		if u.PasswordPolicies != "" {
			sb.WriteString(fmt.Sprintf("**Password policies:** %s\n", u.PasswordPolicies))
		}
		if len(u.AssignedLicenses) > 0 {
			sb.WriteString(fmt.Sprintf("**Assigned license SKU IDs:** %s\n", strings.Join(u.AssignedLicenses, ", ")))
			sb.WriteString("_Use entra.list_subscribed_skus to resolve SKU IDs to friendly names._\n")
		} else {
			sb.WriteString("**Assigned licenses:** none\n")
		}
		if u.OnPremisesSyncEnabled {
			sb.WriteString("\n> ⚠️ This account is synced from on-premises AD. Write operations follow the source-of-authority model — most attribute changes must be made in AD (use ad-mcp). Licenses, sessions, and MFA methods are managed in Entra.\n")
		}
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.get_user_groups",
		mcp.WithDescription("Get direct group memberships for a user."),
		mcp.WithString("id", mcp.Required(), mcp.Description("User UPN or object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.get_user_groups", mcppolicy.Resource{Type: "entra:user", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		groups, err := gc.GetUserGroups(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		return toolText(formatGroupList(groups, id, "direct")), nil
	})

	s.AddTool(mcp.NewTool("entra.get_transitive_user_groups",
		mcp.WithDescription("Get all group memberships for a user including nested and dynamic groups."),
		mcp.WithString("id", mcp.Required(), mcp.Description("User UPN or object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.get_transitive_user_groups", mcppolicy.Resource{Type: "entra:user", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		groups, err := gc.GetTransitiveUserGroups(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		return toolText(formatGroupList(groups, id, "transitive (including nested/dynamic)")), nil
	})

	s.AddTool(mcp.NewTool("entra.get_user_auth_methods_summary",
		mcp.WithDescription("Get MFA method types registered for a user (counts only — no device details)."),
		mcp.WithString("id", mcp.Required(), mcp.Description("User UPN or object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.get_user_auth_methods_summary", mcppolicy.Resource{Type: "entra:user", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		methods, err := gc.GetUserAuthMethodsSummary(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		if len(methods) == 0 {
			return toolText(fmt.Sprintf("No MFA methods registered for user %s.\n\n> ⚠️ A user with no registered MFA methods may be blocked from signing in if MFA is enforced by Conditional Access.", id)), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("MFA methods registered for %s:\n\n", id))
		for _, m := range methods {
			sb.WriteString(fmt.Sprintf("- %s (%d registered)\n", m.MethodType, m.Count))
		}
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.get_user_manager",
		mcp.WithDescription("Get the direct manager of a user."),
		mcp.WithString("id", mcp.Required(), mcp.Description("User UPN or object ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		if err := entrapolicy.Authorize(ctx, engine, "entra.get_user_manager", mcppolicy.Resource{Type: "entra:user", ID: id}, ""); err != nil {
			return toolErr(err), nil
		}
		mgr, err := gc.GetUserManager(ctx, id)
		if err != nil {
			return toolErr(err), nil
		}
		if mgr == nil {
			return toolText(fmt.Sprintf("No manager assigned to %s.", id)), nil
		}
		return toolText(fmt.Sprintf("Manager of %s:\n\n**%s** (%s)\nDepartment: %s | Title: %s",
			id, mgr.DisplayName, mgr.UserPrincipalName, mgr.Department, mgr.JobTitle)), nil
	})
}

func formatGroupList(groups []graph.GroupSummary, userID, kind string) string {
	if len(groups) == 0 {
		return fmt.Sprintf("No %s group memberships found for user %s.", kind, userID)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d %s group membership(s) for %s:\n\n", len(groups), kind, userID))
	for _, g := range groups {
		groupType := classifyGroup(g)
		sb.WriteString(fmt.Sprintf("- **%s** [%s]\n", g.DisplayName, groupType))
		if g.OnPremisesSyncEnabled {
			sb.WriteString("  _(synced from on-prem AD — membership changes must use ad-mcp)_\n")
		}
		if g.IsAssignableToRole {
			sb.WriteString("  _(role-assignable — membership changes require privileged workflow)_\n")
		}
	}
	return sb.String()
}

func classifyGroup(g graph.GroupSummary) string {
	for _, t := range g.GroupTypes {
		if t == "Unified" {
			return "Microsoft 365 Group"
		}
	}
	if g.MailEnabled && !g.SecurityEnabled {
		return "Distribution Group"
	}
	if g.SecurityEnabled {
		return "Security Group"
	}
	return "Group"
}
