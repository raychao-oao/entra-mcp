package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	msgraph "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/groups"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Client wraps the Microsoft Graph service client with helper methods for
// each entra-mcp tool. All methods return plain Go structs to keep the
// tool layer independent of the SDK's generated interfaces.
type Client struct {
	svc *msgraph.GraphServiceClient
}

// NewClient creates an authenticated Graph client using app-only
// (client credentials) flow.
func NewClient(tenantID, clientID, clientSecret string) (*Client, error) {
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("credential: %w", err)
	}
	svc, err := msgraph.NewGraphServiceClientWithCredentials(cred, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		return nil, fmt.Errorf("graph client: %w", err)
	}
	return &Client{svc: svc}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func timeVal(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func ptr[T any](v T) *T { return &v }

// ── User types ───────────────────────────────────────────────────────────────

type UserSummary struct {
	ID                     string
	DisplayName            string
	UserPrincipalName      string
	Mail                   string
	AccountEnabled         bool
	UserType               string
	Department             string
	JobTitle               string
	OnPremisesSyncEnabled  bool
	LastSuccessfulSignIn   string
}

type UserDetail struct {
	UserSummary
	AssignedLicenses []string // SKU IDs
	PasswordPolicies string
	ManagerUPN       string
}

// ── Group types ──────────────────────────────────────────────────────────────

type GroupSummary struct {
	ID                    string
	DisplayName           string
	Description           string
	GroupTypes            []string
	SecurityEnabled       bool
	MailEnabled           bool
	OnPremisesSyncEnabled bool
	IsAssignableToRole    bool
}

// ── SKU types ────────────────────────────────────────────────────────────────

type SKUSummary struct {
	SKUID        string
	SkuPartNumber string
	ConsumedUnits int32
	EnabledUnits  int32
	Status        string
}

// ── AuthMethod types ─────────────────────────────────────────────────────────

type AuthMethodSummary struct {
	MethodType string
	Count      int
}

// ── User methods ─────────────────────────────────────────────────────────────

// SearchUsers searches by displayName, UPN, or mail with a minimum query length of 3.
// Returns at most maxResults (cap: 50).
func (c *Client) SearchUsers(ctx context.Context, query string, maxResults int32) ([]UserSummary, error) {
	if len(query) < 3 {
		return nil, fmt.Errorf("query must be at least 3 characters")
	}
	if maxResults <= 0 || maxResults > 50 {
		maxResults = 50
	}

	filter := fmt.Sprintf(
		"startswith(displayName,'%s') or startswith(userPrincipalName,'%s') or startswith(mail,'%s')",
		escapeOData(query), escapeOData(query), escapeOData(query),
	)
	selectFields := []string{"id", "displayName", "userPrincipalName", "mail", "accountEnabled", "userType", "department", "jobTitle", "onPremisesSyncEnabled"}

	result, err := c.svc.Users().Get(ctx, &users.UsersRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.UsersRequestBuilderGetQueryParameters{
			Filter: &filter,
			Select: selectFields,
			Top:    &maxResults,
		},
	})
	if err != nil {
		return nil, wrapGraphErr(err)
	}

	return usersToSummary(result.GetValue()), nil
}

// GetUser retrieves a full user profile by UPN or object ID.
// Sign-in activity is included when AuditLog.Read.All is granted and the
// tenant has Entra ID P1/P2 — fields will be empty if unavailable.
func (c *Client) GetUser(ctx context.Context, idOrUPN string) (*UserDetail, error) {
	selectFields := []string{
		"id", "displayName", "userPrincipalName", "mail", "accountEnabled",
		"userType", "department", "jobTitle", "onPremisesSyncEnabled",
		"assignedLicenses", "passwordPolicies", "signInActivity",
	}

	result, err := c.svc.Users().ByUserId(idOrUPN).Get(ctx, &users.UserItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.UserItemRequestBuilderGetQueryParameters{
			Select: selectFields,
		},
	})
	if err != nil {
		return nil, wrapGraphErr(err)
	}

	return userToDetail(result), nil
}

// GetUserGroups returns the direct group memberships of a user.
func (c *Client) GetUserGroups(ctx context.Context, userID string) ([]GroupSummary, error) {
	result, err := c.svc.Users().ByUserId(userID).MemberOf().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	return directoryObjectsToGroups(result.GetValue()), nil
}

// GetTransitiveUserGroups returns all group memberships including nested and dynamic.
func (c *Client) GetTransitiveUserGroups(ctx context.Context, userID string) ([]GroupSummary, error) {
	result, err := c.svc.Users().ByUserId(userID).TransitiveMemberOf().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	return directoryObjectsToGroups(result.GetValue()), nil
}

// GetUserAuthMethodsSummary returns the count of each MFA method type registered.
// Returns method types only — no device identifiers or secret values.
func (c *Client) GetUserAuthMethodsSummary(ctx context.Context, userID string) ([]AuthMethodSummary, error) {
	result, err := c.svc.Users().ByUserId(userID).Authentication().Methods().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}

	counts := map[string]int{}
	for _, m := range result.GetValue() {
		odata := strVal(m.GetOdataType())
		methodType := odataTypeToMethodName(odata)
		counts[methodType]++
	}

	var out []AuthMethodSummary
	for t, n := range counts {
		out = append(out, AuthMethodSummary{MethodType: t, Count: n})
	}
	return out, nil
}

// GetUserManager returns the UPN and display name of the user's direct manager.
func (c *Client) GetUserManager(ctx context.Context, userID string) (*UserSummary, error) {
	result, err := c.svc.Users().ByUserId(userID).Manager().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	// Manager() returns a DirectoryObject; cast to User.
	if u, ok := result.(models.Userable); ok {
		s := userToSummary(u)
		return &s, nil
	}
	return nil, fmt.Errorf("manager is not a user object")
}

// ── Group methods ────────────────────────────────────────────────────────────

// SearchGroups searches groups by displayName or description.
func (c *Client) SearchGroups(ctx context.Context, query string, maxResults int32) ([]GroupSummary, error) {
	if len(query) < 3 {
		return nil, fmt.Errorf("query must be at least 3 characters")
	}
	if maxResults <= 0 || maxResults > 50 {
		maxResults = 50
	}

	filter := fmt.Sprintf("startswith(displayName,'%s')", escapeOData(query))
	selectFields := []string{"id", "displayName", "description", "groupTypes", "securityEnabled", "mailEnabled", "onPremisesSyncEnabled", "isAssignableToRole"}

	result, err := c.svc.Groups().Get(ctx, &groups.GroupsRequestBuilderGetRequestConfiguration{
		QueryParameters: &groups.GroupsRequestBuilderGetQueryParameters{
			Filter: &filter,
			Select: selectFields,
			Top:    &maxResults,
		},
	})
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	return groupsToSummary(result.GetValue()), nil
}

// GetGroup retrieves a group by object ID or display name.
func (c *Client) GetGroup(ctx context.Context, groupID string) (*GroupSummary, error) {
	selectFields := []string{"id", "displayName", "description", "groupTypes", "securityEnabled", "mailEnabled", "onPremisesSyncEnabled", "isAssignableToRole"}

	result, err := c.svc.Groups().ByGroupId(groupID).Get(ctx, &groups.GroupItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &groups.GroupItemRequestBuilderGetQueryParameters{
			Select: selectFields,
		},
	})
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	g := groupToSummary(result)
	return &g, nil
}

// ListGroupMembers returns up to maxResults direct members of a group.
// Note: service principals may be excluded in v1.0.
func (c *Client) ListGroupMembers(ctx context.Context, groupID string, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	result, err := c.svc.Groups().ByGroupId(groupID).Members().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	return directoryObjectsToUsers(result.GetValue()), nil
}

// ListGroupOwners returns the owners of a group.
// Returns empty for distribution groups, Exchange-created groups, and synced groups.
func (c *Client) ListGroupOwners(ctx context.Context, groupID string) ([]UserSummary, error) {
	result, err := c.svc.Groups().ByGroupId(groupID).Owners().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	return directoryObjectsToUsers(result.GetValue()), nil
}

// ── Report methods ───────────────────────────────────────────────────────────

// FindDisabledUsers returns accounts with accountEnabled = false.
func (c *Client) FindDisabledUsers(ctx context.Context, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	filter := "accountEnabled eq false"
	return c.listUsers(ctx, filter, maxResults)
}

// FindInactiveUsers returns member accounts with no sign-in in the last N days.
// Requires AuditLog.Read.All and Entra ID P1/P2 tenant license.
func (c *Client) FindInactiveUsers(ctx context.Context, days int, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	filter := fmt.Sprintf(
		"userType eq 'Member' and signInActivity/lastSuccessfulSignInDateTime le %s",
		cutoff,
	)
	selectFields := []string{"id", "displayName", "userPrincipalName", "mail", "accountEnabled", "userType", "department", "jobTitle", "onPremisesSyncEnabled", "signInActivity"}
	return c.listUsersWithSelect(ctx, filter, selectFields, maxResults)
}

// FindGuestUsers returns accounts with userType = Guest.
func (c *Client) FindGuestUsers(ctx context.Context, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	filter := "userType eq 'Guest'"
	return c.listUsers(ctx, filter, maxResults)
}

// FindStaleGuests returns guest accounts with no sign-in in the last N days.
// Requires AuditLog.Read.All and Entra ID P1/P2.
func (c *Client) FindStaleGuests(ctx context.Context, days int, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	filter := fmt.Sprintf(
		"userType eq 'Guest' and signInActivity/lastSuccessfulSignInDateTime le %s",
		cutoff,
	)
	selectFields := []string{"id", "displayName", "userPrincipalName", "mail", "accountEnabled", "userType", "department", "jobTitle", "onPremisesSyncEnabled", "signInActivity"}
	return c.listUsersWithSelect(ctx, filter, selectFields, maxResults)
}

// FindPasswordNeverExpires returns accounts with DisablePasswordExpiration policy.
func (c *Client) FindPasswordNeverExpires(ctx context.Context, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	filter := "passwordPolicies/any(p: p eq 'DisablePasswordExpiration')"
	return c.listUsers(ctx, filter, maxResults)
}

// FindSyncedUsers returns accounts synced from on-premises AD.
func (c *Client) FindSyncedUsers(ctx context.Context, maxResults int32) ([]UserSummary, error) {
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 200
	}
	filter := "onPremisesSyncEnabled eq true"
	return c.listUsers(ctx, filter, maxResults)
}

// FindPrivilegedRoleMembers returns members of high-privilege directory roles.
func (c *Client) FindPrivilegedRoleMembers(ctx context.Context) (map[string][]UserSummary, error) {
	privilegedRoles := map[string]bool{
		"Global Administrator":              true,
		"Privileged Role Administrator":     true,
		"Security Administrator":            true,
		"Exchange Administrator":            true,
		"SharePoint Administrator":          true,
		"User Administrator":                true,
		"Billing Administrator":             true,
		"Application Administrator":         true,
		"Cloud Application Administrator":   true,
		"Authentication Administrator":      true,
	}

	roles, err := c.svc.DirectoryRoles().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}

	result := map[string][]UserSummary{}
	for _, role := range roles.GetValue() {
		name := strVal(role.GetDisplayName())
		if !privilegedRoles[name] {
			continue
		}
		roleID := strVal(role.GetId())
		members, err := c.svc.DirectoryRoles().ByDirectoryRoleId(roleID).Members().Get(ctx, nil)
		if err != nil {
			// Log and continue — partial results are still useful.
			result[name] = nil
			continue
		}
		result[name] = directoryObjectsToUsers(members.GetValue())
	}
	return result, nil
}

// ListSubscribedSKUs returns all license SKUs with capacity and consumed counts.
func (c *Client) ListSubscribedSKUs(ctx context.Context) ([]SKUSummary, error) {
	result, err := c.svc.SubscribedSkus().Get(ctx, nil)
	if err != nil {
		return nil, wrapGraphErr(err)
	}

	var out []SKUSummary
	for _, sku := range result.GetValue() {
		var enabled int32
		if pp := sku.GetPrepaidUnits(); pp != nil {
			if e := pp.GetEnabled(); e != nil {
				enabled = *e
			}
		}
		var consumed int32
		if c := sku.GetConsumedUnits(); c != nil {
			consumed = *c
		}
		skuID := ""
		if id := sku.GetSkuId(); id != nil {
			skuID = id.String()
		}
		out = append(out, SKUSummary{
			SKUID:         skuID,
			SkuPartNumber: strVal(sku.GetSkuPartNumber()),
			ConsumedUnits: consumed,
			EnabledUnits:  enabled,
			Status:        strVal(sku.GetCapabilityStatus()),
		})
	}
	return out, nil
}

// ── internal helpers ─────────────────────────────────────────────────────────

func (c *Client) listUsers(ctx context.Context, filter string, maxResults int32) ([]UserSummary, error) {
	selectFields := []string{"id", "displayName", "userPrincipalName", "mail", "accountEnabled", "userType", "department", "jobTitle", "onPremisesSyncEnabled"}
	return c.listUsersWithSelect(ctx, filter, selectFields, maxResults)
}

func (c *Client) listUsersWithSelect(ctx context.Context, filter string, selectFields []string, maxResults int32) ([]UserSummary, error) {
	result, err := c.svc.Users().Get(ctx, &users.UsersRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.UsersRequestBuilderGetQueryParameters{
			Filter: &filter,
			Select: selectFields,
			Top:    &maxResults,
		},
	})
	if err != nil {
		return nil, wrapGraphErr(err)
	}
	return usersToSummary(result.GetValue()), nil
}

func usersToSummary(users []models.Userable) []UserSummary {
	out := make([]UserSummary, 0, len(users))
	for _, u := range users {
		out = append(out, userToSummary(u))
	}
	return out
}

func userToSummary(u models.Userable) UserSummary {
	s := UserSummary{
		ID:                    strVal(u.GetId()),
		DisplayName:           strVal(u.GetDisplayName()),
		UserPrincipalName:     strVal(u.GetUserPrincipalName()),
		Mail:                  strVal(u.GetMail()),
		AccountEnabled:        boolVal(u.GetAccountEnabled()),
		UserType:              strVal(u.GetUserType()),
		Department:            strVal(u.GetDepartment()),
		JobTitle:              strVal(u.GetJobTitle()),
		OnPremisesSyncEnabled: boolVal(u.GetOnPremisesSyncEnabled()),
	}
	if sa := u.GetSignInActivity(); sa != nil {
		s.LastSuccessfulSignIn = timeVal(sa.GetLastSuccessfulSignInDateTime())
	}
	return s
}

func userToDetail(u models.Userable) *UserDetail {
	d := &UserDetail{
		UserSummary:  userToSummary(u),
		PasswordPolicies: strVal(u.GetPasswordPolicies()),
	}
	for _, lic := range u.GetAssignedLicenses() {
		if id := lic.GetSkuId(); id != nil {
			d.AssignedLicenses = append(d.AssignedLicenses, id.String())
		}
	}
	return d
}

func groupsToSummary(groups []models.Groupable) []GroupSummary {
	out := make([]GroupSummary, 0, len(groups))
	for _, g := range groups {
		out = append(out, groupToSummary(g))
	}
	return out
}

func groupToSummary(g models.Groupable) GroupSummary {
	return GroupSummary{
		ID:                    strVal(g.GetId()),
		DisplayName:           strVal(g.GetDisplayName()),
		Description:           strVal(g.GetDescription()),
		GroupTypes:            g.GetGroupTypes(),
		SecurityEnabled:       boolVal(g.GetSecurityEnabled()),
		MailEnabled:           boolVal(g.GetMailEnabled()),
		OnPremisesSyncEnabled: boolVal(g.GetOnPremisesSyncEnabled()),
		IsAssignableToRole:    boolVal(g.GetIsAssignableToRole()),
	}
}

func directoryObjectsToGroups(objs []models.DirectoryObjectable) []GroupSummary {
	var out []GroupSummary
	for _, obj := range objs {
		if g, ok := obj.(models.Groupable); ok {
			out = append(out, groupToSummary(g))
		}
	}
	return out
}

func directoryObjectsToUsers(objs []models.DirectoryObjectable) []UserSummary {
	var out []UserSummary
	for _, obj := range objs {
		if u, ok := obj.(models.Userable); ok {
			out = append(out, userToSummary(u))
		}
	}
	return out
}

func odataTypeToMethodName(odata string) string {
	switch {
	case strings.Contains(odata, "microsoftAuthenticatorAuthenticationMethod"):
		return "MicrosoftAuthenticator"
	case strings.Contains(odata, "phoneAuthenticationMethod"):
		return "Phone"
	case strings.Contains(odata, "emailAuthenticationMethod"):
		return "Email"
	case strings.Contains(odata, "fido2AuthenticationMethod"):
		return "FIDO2"
	case strings.Contains(odata, "windowsHelloForBusinessAuthenticationMethod"):
		return "WindowsHello"
	case strings.Contains(odata, "passwordAuthenticationMethod"):
		return "Password"
	case strings.Contains(odata, "softwareOathAuthenticationMethod"):
		return "SoftwareOATH"
	case strings.Contains(odata, "temporaryAccessPassAuthenticationMethod"):
		return "TemporaryAccessPass"
	default:
		return odata
	}
}

func escapeOData(s string) string {
	// Escape single quotes for OData $filter strings.
	return strings.ReplaceAll(s, "'", "''")
}

// wrapGraphErr wraps Graph API errors, surfacing throttling (429) clearly.
func wrapGraphErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "429") || strings.Contains(strings.ToLower(msg), "throttl") {
		return fmt.Errorf("Microsoft Graph API rate limit reached — please wait and retry: %w", err)
	}
	return err
}
