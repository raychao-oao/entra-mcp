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

func registerReportTools(s *server.MCPServer, gc *graph.Client, engine *yamlengine.Engine) {
	s.AddTool(mcp.NewTool("entra.find_disabled_users",
		mcp.WithDescription("Find all accounts with accountEnabled = false. Max 200 results."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_disabled_users", mcppolicy.Resource{Type: "entra:report", ID: "disabled_users"}, ""); err != nil {
			return toolErr(err), nil
		}
		users, err := gc.FindDisabledUsers(ctx, 200)
		if err != nil {
			return toolErr(err), nil
		}
		return toolText(formatUserReport(users, "Disabled users", 200)), nil
	})

	s.AddTool(mcp.NewTool("entra.find_inactive_users",
		mcp.WithDescription("Find member accounts with no sign-in in the last N days. Requires AuditLog.Read.All + Entra ID P1/P2."),
		mcp.WithNumber("days", mcp.Description("Inactivity threshold in days (default: 90)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_inactive_users", mcppolicy.Resource{Type: "entra:report", ID: "inactive_users"}, ""); err != nil {
			return toolErr(err), nil
		}
		days := req.GetInt("days", 90)
		users, err := gc.FindInactiveUsers(ctx, days, 200)
		if err != nil {
			return toolErr(err), nil
		}
		title := fmt.Sprintf("Member accounts inactive for %d+ days", days)
		return toolText(formatUserReport(users, title, 200)), nil
	})

	s.AddTool(mcp.NewTool("entra.find_guest_users",
		mcp.WithDescription("Find all external guest accounts (userType = Guest). Max 200 results."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_guest_users", mcppolicy.Resource{Type: "entra:report", ID: "guest_users"}, ""); err != nil {
			return toolErr(err), nil
		}
		users, err := gc.FindGuestUsers(ctx, 200)
		if err != nil {
			return toolErr(err), nil
		}
		return toolText(formatUserReport(users, "External guest accounts", 200)), nil
	})

	s.AddTool(mcp.NewTool("entra.find_stale_guests",
		mcp.WithDescription("Find guest accounts with no sign-in in the last N days. Requires AuditLog.Read.All + Entra ID P1/P2."),
		mcp.WithNumber("days", mcp.Description("Inactivity threshold in days (default: 90)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_stale_guests", mcppolicy.Resource{Type: "entra:report", ID: "stale_guests"}, ""); err != nil {
			return toolErr(err), nil
		}
		days := req.GetInt("days", 90)
		users, err := gc.FindStaleGuests(ctx, days, 200)
		if err != nil {
			return toolErr(err), nil
		}
		title := fmt.Sprintf("Stale guest accounts (no sign-in in %d+ days)", days)
		return toolText(formatUserReport(users, title, 200)), nil
	})

	s.AddTool(mcp.NewTool("entra.find_password_never_expires",
		mcp.WithDescription("Find accounts with DisablePasswordExpiration policy. Max 200 results."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_password_never_expires", mcppolicy.Resource{Type: "entra:report", ID: "password_never_expires"}, ""); err != nil {
			return toolErr(err), nil
		}
		users, err := gc.FindPasswordNeverExpires(ctx, 200)
		if err != nil {
			return toolErr(err), nil
		}
		return toolText(formatUserReport(users, "Accounts with password never expires", 200)), nil
	})

	s.AddTool(mcp.NewTool("entra.find_synced_users",
		mcp.WithDescription("Find all accounts synced from on-premises AD (onPremisesSyncEnabled = true). Max 200 results."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_synced_users", mcppolicy.Resource{Type: "entra:report", ID: "synced_users"}, ""); err != nil {
			return toolErr(err), nil
		}
		users, err := gc.FindSyncedUsers(ctx, 200)
		if err != nil {
			return toolErr(err), nil
		}
		return toolText(formatUserReport(users, "Accounts synced from on-premises AD", 200)), nil
	})

	s.AddTool(mcp.NewTool("entra.find_privileged_role_members",
		mcp.WithDescription("Find members of high-privilege directory roles (Global Admin, etc.)."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.find_privileged_role_members", mcppolicy.Resource{Type: "entra:report", ID: "privileged_role_members"}, ""); err != nil {
			return toolErr(err), nil
		}
		roleMembers, err := gc.FindPrivilegedRoleMembers(ctx)
		if err != nil {
			return toolErr(err), nil
		}
		if len(roleMembers) == 0 {
			return toolText("No privileged role members found (or no high-privilege roles are activated)."), nil
		}
		var sb strings.Builder
		sb.WriteString("## Privileged Role Members\n\n")
		for role, members := range roleMembers {
			sb.WriteString(fmt.Sprintf("### %s (%d member(s))\n", role, len(members)))
			for _, u := range members {
				sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", u.DisplayName, u.UserPrincipalName))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("> ⚠️ Any write operations targeting these accounts require extra verification.\n")
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.list_subscribed_skus",
		mcp.WithDescription("List all M365 license SKUs with capacity and consumed counts."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.list_subscribed_skus", mcppolicy.Resource{Type: "entra:report", ID: "subscribed_skus"}, ""); err != nil {
			return toolErr(err), nil
		}
		skus, err := gc.ListSubscribedSKUs(ctx)
		if err != nil {
			return toolErr(err), nil
		}
		if len(skus) == 0 {
			return toolText("No license SKUs found."), nil
		}
		var sb strings.Builder
		sb.WriteString("## Subscribed License SKUs\n\n")
		sb.WriteString("| SKU | Status | Consumed | Available |\n")
		sb.WriteString("|-----|--------|----------|-----------|\n")
		for _, sku := range skus {
			available := sku.EnabledUnits - sku.ConsumedUnits
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d |\n",
				sku.SkuPartNumber, sku.Status, sku.ConsumedUnits, available))
		}
		sb.WriteString("\n_Use SKU part numbers to identify licenses. SKU IDs (GUIDs) are shown in entra.get_user output._")
		return toolText(sb.String()), nil
	})

	s.AddTool(mcp.NewTool("entra.generate_account_review_report",
		mcp.WithDescription("Generate a combined M365 account hygiene report: disabled, inactive, guests, password-never-expires, privileged roles."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := entrapolicy.Authorize(ctx, engine, "entra.generate_account_review_report", mcppolicy.Resource{Type: "entra:report", ID: "account_review"}, ""); err != nil {
			return toolErr(err), nil
		}

		var sb strings.Builder
		sb.WriteString("# M365 Account Review Report\n\n")

		disabled, err := gc.FindDisabledUsers(ctx, 200)
		if err != nil {
			sb.WriteString(fmt.Sprintf("## Disabled Accounts\n_Error: %v_\n\n", err))
		} else {
			sb.WriteString(formatUserSection("Disabled Accounts", disabled, 200))
		}

		inactive, err := gc.FindInactiveUsers(ctx, 90, 200)
		if err != nil {
			sb.WriteString(fmt.Sprintf("## Inactive Accounts (90+ days)\n_Error: %v_\n\n", err))
		} else {
			sb.WriteString(formatUserSection("Inactive Accounts (90+ days)", inactive, 200))
		}

		guests, err := gc.FindGuestUsers(ctx, 200)
		if err != nil {
			sb.WriteString(fmt.Sprintf("## Guest Accounts\n_Error: %v_\n\n", err))
		} else {
			sb.WriteString(formatUserSection("Guest Accounts", guests, 200))
		}

		pwdNeverExpires, err := gc.FindPasswordNeverExpires(ctx, 200)
		if err != nil {
			sb.WriteString(fmt.Sprintf("## Password Never Expires\n_Error: %v_\n\n", err))
		} else {
			sb.WriteString(formatUserSection("Password Never Expires", pwdNeverExpires, 200))
		}

		roleMembers, err := gc.FindPrivilegedRoleMembers(ctx)
		if err != nil {
			sb.WriteString(fmt.Sprintf("## Privileged Role Members\n_Error: %v_\n\n", err))
		} else {
			sb.WriteString("## Privileged Role Members\n\n")
			for role, members := range roleMembers {
				sb.WriteString(fmt.Sprintf("**%s:** %d member(s)\n", role, len(members)))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n_Report capped at 200 results per category. Sign-in data requires Entra ID P1/P2._\n")
		return toolText(sb.String()), nil
	})
}

func formatUserReport(users []graph.UserSummary, title string, cap int) string {
	if len(users) == 0 {
		return fmt.Sprintf("No results for: %s.", title)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d)\n\n", title, len(users)))
	sb.WriteString("| Display Name | UPN | Dept | Enabled | Synced | Last Sign-In |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")
	for _, u := range users {
		signIn := u.LastSuccessfulSignIn
		if signIn == "" {
			signIn = "n/a"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %v | %v | %s |\n",
			u.DisplayName, u.UserPrincipalName, u.Department, u.AccountEnabled, u.OnPremisesSyncEnabled, signIn))
	}
	if len(users) >= cap {
		sb.WriteString(fmt.Sprintf("\n_Results capped at %d. There may be additional entries._", cap))
	}
	return sb.String()
}

func formatUserSection(title string, users []graph.UserSummary, cap int) string {
	return formatUserReport(users, title, cap) + "\n\n"
}
