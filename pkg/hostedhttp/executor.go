// Package hostedhttp executes explicitly authorized, destination-bound HTTP
// calls. It is shared by the hosted controller and the workflow platform. It
// grants no authority itself: callers must load an authorized Policy from their
// grant store, never construct one from untrusted execution parameters.
package hostedhttp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/reliant-labs/reliant/internal/connections"
	"github.com/reliant-labs/reliant/internal/integrations/catalog"
	"github.com/reliant-labs/reliant/internal/integrations/httpaction"
	"github.com/reliant-labs/reliant/internal/netguard"
)

// Endpoint authorizes one exact escaped path and method. Prefixes and globs
// are not supported. Query strings are not accepted by this capability.
type Endpoint struct {
	Method string
	Path   string
}

// Policy is the non-secret HTTP portion of a stored, owner-approved use grant.
// Origin must be canonical HTTPS, without a path, query, userinfo or fragment.
// This type does not replace workflow/org/environment authorization.
type Policy struct {
	Origin    string
	Endpoints []Endpoint
}

// Secret supplies the already-authorized secret only inside a callback. An
// implementation must not retain the callback or log its argument. It must
// resolve the latest live version and fail closed on deletion or revocation.
// Secret implementations must not be printable or serializable as plaintext.
type Secret interface {
	Use(context.Context, func([]byte) error) error
}

// Request contains only non-secret data. Authentication is always X-API-Key;
// callers cannot override headers, redirects, query strings or transport knobs.
type Request struct {
	URL    string `json:"url"`
	Method string `json:"method"`
	Body   any    `json:"body"`
}

// MaxRequestBytes limits the serialized request before a ticket is persisted
// or a credential is resolved. The HTTP runner separately bounds responses.
const MaxRequestBytes = 1 << 20

// RequestDigest binds a one-use action ticket to exactly the typed request.
// Field order and explicit null body are part of the internal wire contract.
func RequestDigest(req Request) (string, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return "", errors.New("hosted HTTP request is not JSON")
	}
	if len(raw) > MaxRequestBytes {
		return "", errors.New("hosted HTTP request exceeds size limit")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// Result contains only scrubbed output from the trusted HTTP runner.
type Result struct {
	Content    string         `json:"content"`
	Data       map[string]any `json:"data"`
	StatusCode int            `json:"status_code"`
	IsError    bool           `json:"is_error"`
	Retryable  bool           `json:"retryable"`
}

// Executor uses the same SSRF guard, TLS checks, response limits and scrub
// boundary as integration HTTP actions. There is no daemon transport.
type Executor struct{ runner *httpaction.Runner }

// New constructs a production executor with the standard public-network guard.
func New() *Executor { return &Executor{runner: httpaction.NewRunner(netguard.New())} }

// Validate checks both a policy's canonical form and the rendered request.
// Validate must succeed before any secret is read. Apply checks it again at
// credential attachment, so future redirect behavior cannot widen authority.
func (p Policy) Validate(r Request) error {
	origin, err := url.Parse(p.Origin)
	if err != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || origin.Opaque != "" || origin.RawFragment != "" || origin.Host != strings.ToLower(origin.Host) || origin.Port() != "" && origin.Port() != "443" {
		return errors.New("hosted HTTP grant requires a canonical HTTPS origin on port 443")
	}
	// Explicit :443 and trailing dots create alternate spellings of an origin.
	if origin.Port() != "" || strings.HasSuffix(origin.Hostname(), ".") {
		return errors.New("hosted HTTP grant origin is not canonical")
	}
	if ip := net.ParseIP(origin.Hostname()); ip != nil && netguard.IPBlocked(ip) {
		return errors.New("hosted HTTP grant destination is not public")
	}
	target, err := url.Parse(r.URL)
	if err != nil || target.User != nil || target.Scheme != origin.Scheme || target.Host != origin.Host || target.RawQuery != "" || target.ForceQuery || target.Fragment != "" || target.Opaque != "" || target.RawFragment != "" {
		return errors.New("request does not match hosted HTTP grant origin")
	}
	path := target.EscapedPath()
	// No alternate encodings, traversal or ambiguous separators. Endpoint grants
	// are exact API routes, not filesystem prefixes.
	if path == "" || target.RawPath != "" || strings.Contains(path, "%") || strings.Contains(path, "\\") || strings.Contains(path, "//") {
		return errors.New("hosted HTTP request path is not canonical")
	}
	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." {
			return errors.New("hosted HTTP request path is not canonical")
		}
	}
	switch r.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return errors.New("hosted HTTP method is not permitted")
	}
	for _, ep := range p.Endpoints {
		if ep.Method == r.Method && ep.Path == path {
			return nil
		}
	}
	return errors.New("request endpoint is not authorized by hosted HTTP grant")
}

// Execute performs one authorized request. Policy authorization is the caller's
// responsibility; validation is deliberately performed before Secret.Use.
// Secrets and errors returned by their provider never enter Result or an error
// chain: a provider may be backed by a secret-bearing storage response.
func (e *Executor) Execute(ctx context.Context, p Policy, req Request, secret Secret) (*Result, error) {
	if err := p.Validate(req); err != nil {
		return nil, err
	}
	if e == nil || e.runner == nil || secret == nil {
		return nil, errors.New("hosted HTTP execution is unavailable")
	}
	// Reject unserializable/oversized requests before resolving the value.
	if _, err := RequestDigest(req); err != nil {
		return nil, err
	}
	action, err := catalog.MustBuiltin().Resolve("http/request@1")
	if err != nil {
		return nil, errors.New("hosted HTTP action is unavailable")
	}
	var out *Result
	var executionErr error
	err = secret.Use(ctx, func(value []byte) error {
		if len(value) < 16 {
			executionErr = errors.New("hosted HTTP credential is invalid")
			return nil
		}
		redactor := connections.NewRedactor()
		text := string(value)
		for _, ch := range value {
			if ch < 0x20 || ch == 0x7f {
				executionErr = errors.New("hosted HTTP credential is invalid")
				return nil
			}
		}
		redactor.Register(text)
		redactor.Register(url.QueryEscape(text))
		redactor.Register(url.PathEscape(text))
		if escaped, err := json.Marshal(text); err == nil {
			redactor.Register(string(escaped[1 : len(escaped)-1]))
		}
		cred := &credential{policy: p, value: text, redactor: redactor}
		result, runErr := e.runner.RunAuthenticated(ctx, action.Manifest, action.Spec, map[string]any{
			"url": req.URL, "method": req.Method, "body": req.Body,
			"follow_redirects": false, "response_format": "json", "connection": "hosted-use",
		}, source{cred}, httpaction.CallSite{})
		if runErr != nil {
			executionErr = errors.New("hosted HTTP request could not be completed")
			return nil
		}
		if result == nil {
			executionErr = errors.New("hosted HTTP request returned no result")
			return nil
		}
		// The runner scrubs raw content and all structured fields before returning.
		out = &Result{Content: result.Content, Data: result.Data, StatusCode: result.StatusCode, IsError: result.IsError, Retryable: result.Retryable}
		return nil
	})
	if err != nil {
		return nil, errors.New("hosted HTTP credential is unavailable")
	}
	return out, executionErr
}

type source struct{ cred *credential }

func (s source) Credential(context.Context, httpaction.CredentialRequest) (httpaction.Credential, error) {
	return s.cred, nil
}

type credential struct {
	policy   Policy
	value    string
	redactor *connections.Redactor
}

func (*credential) ConnectionID() string      { return "" }
func (*credential) Params() map[string]string { return nil }
func (c *credential) Scrub(s string) string   { return c.redactor.Scrub(s) }
func (c *credential) Apply(req *http.Request) error {
	if req == nil || req.URL == nil {
		return errors.New("hosted HTTP request is missing")
	}
	if err := c.policy.Validate(Request{URL: req.URL.String(), Method: req.Method}); err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.value)
	return nil
}
func (*credential) String() string               { return "hostedhttp.credential{redacted}" }
func (c *credential) GoString() string           { return c.String() }
func (c *credential) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(c.String())) }

var _ httpaction.CredentialSource = source{}
var _ httpaction.Credential = (*credential)(nil)
