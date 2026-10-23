// Copyright (c) 2025 Reliant Labs
package token

import (
	"context"
	"slices"
	"testing"

	"connectrpc.com/connect"
	fat "github.com/reliant-labs/forge/pkg/accesstoken"
	"github.com/reliant-labs/forge/pkg/svcerr"

	reliantv1 "github.com/reliant-labs/reliant/gen/reliant/v1"
	"github.com/reliant-labs/reliant/internal/accesstokenclient"
	"github.com/reliant-labs/reliant/internal/tokenauthority"
)

// cpAuthority is tokenauthority.Memory behind control-plane's two decisions
// that matter to a derived scope:
//
//   - MintForUser acts in the user's primary org, which exists only while they
//     are a member (members). A non-member's mint fails as control-plane's
//     "has no organization" does.
//   - UpdateForUser refuses an org permission the user's grants (grants) do not
//     include, as access_token_internal.requireUserHolds does.
//
// Both maps are read at call time, so a test can revoke a grant or remove a
// member between calls.
type cpAuthority struct {
	*tokenauthority.Memory
	members map[string]bool
	grants  map[string]fat.Set
	// asParent, when set, rewrites what Introspect says about the PARENT token
	// (not minted children), to model a caller shape Memory cannot mint.
	asParent func(*fat.Principal)
	parent   string
}

func (a *cpAuthority) MintForUser(ctx context.Context, req tokenauthority.MintRequest) (tokenauthority.Minted, error) {
	if !a.members[req.UserID] {
		return tokenauthority.Minted{}, &accesstokenclient.RPCError{Code: "failed_precondition", Message: "user " + req.UserID + " has no organization"}
	}
	return a.Memory.MintForUser(ctx, req)
}

func (a *cpAuthority) UpdateForUser(ctx context.Context, userID, tokenID string, name *string, scopes fat.Set) (tokenauthority.TokenInfo, error) {
	if scopes.Has(fat.ScopeDBQuery) && !a.grants[userID].Has(fat.ScopeDBQuery) {
		return tokenauthority.TokenInfo{}, &accesstokenclient.RPCError{Code: "permission_denied",
			Message: "your organization permissions do not include db:query"}
	}
	return a.Memory.UpdateForUser(ctx, userID, tokenID, name, scopes)
}

func (a *cpAuthority) Introspect(ctx context.Context, token string) (*fat.Principal, error) {
	p, err := a.Memory.Introspect(ctx, token)
	if err == nil && a.asParent != nil && token == a.parent {
		c := *p
		a.asParent(&c)
		return &c, nil
	}
	return p, err
}

func newCP(members []string, grants map[string]fat.Set) *cpAuthority {
	a := &cpAuthority{Memory: tokenauthority.NewMemory(), members: map[string]bool{}, grants: grants}
	for _, m := range members {
		a.members[m] = true
	}
	return a
}

func exchange(t *testing.T, a tokenauthority.Authority, bearer string, scopes ...string) (*reliantv1.ExchangeTokenResponse, error) {
	t.Helper()
	svc := newTokenService(t, a, TokenControlPlane{Issuer: testControlPlane, ClipsGrants: true})
	resp, err := svc.ExchangeToken(context.Background(), exchangeReq(bearer, &reliantv1.ExchangeTokenRequest{
		Audience: testControlPlane, Scopes: scopes,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func liveTokens(t *testing.T, a tokenauthority.Authority, userID string) []tokenauthority.TokenInfo {
	t.Helper()
	infos, err := a.ListForUser(context.Background(), userID, "")
	if err != nil {
		t.Fatal(err)
	}
	return infos
}

// A deploy-only daemon credential (what exists today) is enough to ask for the
// derived scopes: they are derived, not inherited from the caller.
func TestExchangeToken_ObsScopesForMembers(t *testing.T) {
	a := newCP([]string{"user-1"}, nil)
	bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{
		Scopes: fullDaemon, Resource: &fat.Resource{Kind: fat.ResourceDaemon, ID: "daemon-1"},
	})
	resp, err := exchange(t, a, bearer, "obs:read", "obs:triage")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(resp.GetScopes(), []string{"obs:read", "obs:triage"}) {
		t.Fatalf("granted %v", resp.GetScopes())
	}
	p, err := a.Introspect(context.Background(), resp.GetToken())
	if err != nil {
		t.Fatal(err)
	}
	if !p.BoundTo(fat.ResourceDaemon, "daemon-1") || p.ActingUserID != "user-1" {
		t.Errorf("binding/actor not inherited: %+v", p)
	}
	if p.Scopes.Has(fat.ScopeDBQuery) || p.Scopes.Has(fat.ScopeDeployWrite) {
		t.Errorf("only the requested scopes: %s", p.Scopes)
	}
}

func TestExchangeToken_DBQueryNeedsTheGrant(t *testing.T) {
	for _, tc := range []struct {
		name   string
		grants map[string]fat.Set
		want   []string
	}{
		{"held", map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDBQuery)}, []string{"obs:read", "db:query"}},
		{"member without the grant: omitted, rest proceeds", map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDeployWrite)}, []string{"obs:read"}},
		{"no grant rows at all", nil, []string{"obs:read"}},
		{"another user's grant is no use", map[string]fat.Set{"user-2": fat.SetOf(fat.ScopeDBQuery)}, []string{"obs:read"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newCP([]string{"user-1", "user-2"}, tc.grants)
			// The calling token claims db:query itself: it must not matter.
			bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{
				Scopes: append([]fat.Scope{fat.ScopeDBQuery}, fullDaemon...),
			})
			resp, err := exchange(t, a, bearer, "obs:read", "db:query")
			if err != nil {
				t.Fatal(err)
			}
			if got := resp.GetScopes(); !slices.Equal(got, tc.want) {
				t.Fatalf("granted %v, want %v", got, tc.want)
			}
			p, err := a.Introspect(context.Background(), resp.GetToken())
			if err != nil {
				t.Fatal(err)
			}
			if p.Scopes.Has(fat.ScopeDBQuery) != slices.Contains(tc.want, "db:query") {
				t.Errorf("stored scopes %s disagree with the response %v", p.Scopes, tc.want)
			}
			// Exactly one exchanged token survives; any superseded one is revoked.
			if n := len(liveTokens(t, a, "user-1")); n != 2 {
				t.Errorf("live tokens = %d, want the parent plus one exchanged", n)
			}
		})
	}
}

func TestExchangeToken_DBQueryAloneWithoutGrantIsRefusedAndMintsNothing(t *testing.T) {
	a := newCP([]string{"user-1"}, map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDeployWrite)})
	bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
	_, err := exchange(t, a, bearer, "db:query")
	if svcerr.Code(err) != connect.CodePermissionDenied {
		t.Fatalf("err = %v, want PermissionDenied", err)
	}
	if n := len(liveTokens(t, a, "user-1")); n != 1 {
		t.Fatalf("a refused exchange left %d live tokens, want the parent alone", n)
	}
}

func TestExchangeToken_RevokedGrantIsHonouredAtTheNextExchange(t *testing.T) {
	a := newCP([]string{"user-1"}, map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDBQuery)})
	bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
	resp, err := exchange(t, a, bearer, "db:query")
	if err != nil || !slices.Equal(resp.GetScopes(), []string{"db:query"}) {
		t.Fatalf("with the grant: %v %v", resp.GetScopes(), err)
	}
	delete(a.grants, "user-1") // an admin revokes the grant; the session token is unchanged
	if _, err := exchange(t, a, bearer, "db:query"); svcerr.Code(err) != connect.CodePermissionDenied {
		t.Fatalf("after revocation: err = %v, want PermissionDenied", err)
	}
}

func TestExchangeToken_NonMemberGetsNoDerivedScope(t *testing.T) {
	// user-9 holds a (stale) session credential but is no longer in any org.
	a := newCP(nil, map[string]fat.Set{"user-9": fat.SetOf(fat.ScopeDBQuery)})
	bearer := mintSession(t, a.Memory, "user-9", tokenauthority.MintRequest{Scopes: fullDaemon})
	for _, scopes := range [][]string{{"obs:read"}, {"obs:triage"}, {"db:query"}, {"obs:read", "db:query"}} {
		if resp, err := exchange(t, a, bearer, scopes...); err == nil {
			t.Fatalf("%v: a non-member was granted %v", scopes, resp.GetScopes())
		}
	}
	if n := len(liveTokens(t, a, "user-9")); n != 1 {
		t.Fatalf("live tokens = %d, want the parent alone", n)
	}
}

func TestExchangeToken_MemberOfAnotherOrgIsRefused(t *testing.T) {
	// The caller's token is for org A; the user's primary org is now B.
	mem := tokenauthority.NewMemory()
	bearer := mintSession(t, mem, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
	a := orgShiftAuthority{Memory: mem, parentToken: bearer}
	svc := newTokenService(t, a, TokenControlPlane{Issuer: testControlPlane, ClipsGrants: true})
	_, err := svc.ExchangeToken(context.Background(), exchangeReq(bearer, &reliantv1.ExchangeTokenRequest{
		Audience: testControlPlane, Scopes: []string{"obs:read", "obs:triage"},
	}))
	if svcerr.Code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("err = %v, want FailedPrecondition", err)
	}
	if n := len(liveTokens(t, mem, "user-1")); n != 1 {
		t.Fatalf("live tokens = %d, want the parent alone", n)
	}
}

func TestExchangeToken_NonDaemonResourceBoundCallerStillRefused(t *testing.T) {
	for _, kind := range []fat.ResourceKind{fat.ResourcePort, fat.ResourceConnector} {
		a := newCP([]string{"user-1"}, map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDBQuery)})
		bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
		a.parent = bearer
		a.asParent = func(p *fat.Principal) { p.Resource = &fat.Resource{Kind: kind, ID: "x"} }
		for _, scopes := range [][]string{{"obs:read"}, {"db:query"}, {"deploy:read"}} {
			if _, err := exchange(t, a, bearer, scopes...); svcerr.Code(err) != connect.CodePermissionDenied {
				t.Errorf("%s-bound, %v: err = %v, want PermissionDenied", kind, scopes, err)
			}
		}
		if n := len(liveTokens(t, a, "user-1")); n != 1 {
			t.Errorf("%s-bound: refused exchanges left %d live tokens", kind, n)
		}
	}
}

func TestExchangeToken_DerivedScopesOnlyWhereControlPlaneClips(t *testing.T) {
	a := newCP([]string{"user-1"}, map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDBQuery)})
	bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
	svc := newTokenService(t, a, TokenControlPlane{Issuer: testControlPlane}) // ClipsGrants false
	_, err := svc.ExchangeToken(context.Background(), exchangeReq(bearer, &reliantv1.ExchangeTokenRequest{
		Audience: testControlPlane, Scopes: []string{"obs:read", "db:query"},
	}))
	if svcerr.Code(err) != connect.CodePermissionDenied {
		t.Fatalf("err = %v, want PermissionDenied", err)
	}
}

func TestExchangeToken_UnavailableGrantCheckNeverGrantsDBQuery(t *testing.T) {
	a := &flakyUpdate{cpAuthority: newCP([]string{"user-1"}, map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDBQuery)})}
	bearer := mintSession(t, a.cpAuthority, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
	if resp, err := exchange(t, a, bearer, "obs:read", "db:query"); err == nil {
		t.Fatalf("granted %v although the grant could not be checked", resp.GetScopes())
	}
	if n := len(liveTokens(t, a, "user-1")); n != 1 {
		t.Fatalf("live tokens = %d, want the parent alone (the unverified token revoked)", n)
	}
}

type flakyUpdate struct{ *cpAuthority }

func (f *flakyUpdate) UpdateForUser(context.Context, string, string, *string, fat.Set) (tokenauthority.TokenInfo, error) {
	return tokenauthority.TokenInfo{}, &accesstokenclient.RPCError{Code: "unavailable", Message: "down"}
}

func TestExchangeToken_DefaultRequestStaysDeployOnly(t *testing.T) {
	a := newCP([]string{"user-1"}, map[string]fat.Set{"user-1": fat.SetOf(fat.ScopeDBQuery)})
	bearer := mintSession(t, a, "user-1", tokenauthority.MintRequest{Scopes: fullDaemon})
	resp, err := exchange(t, a, bearer)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range resp.GetScopes() {
		if s == "obs:read" || s == "obs:triage" || s == "db:query" {
			t.Errorf("an unqualified exchange granted %s", s)
		}
	}
}
