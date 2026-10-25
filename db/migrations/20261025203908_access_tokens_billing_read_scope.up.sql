-- forge:backward-compatible — widens the allowed scope vocabulary with billing:read; all previously valid tokens remain valid.
-- Only the vocabulary changes. No existing token gains a scope, and no member
-- gets implied access: live explicit organization grants remain authoritative.
ALTER TABLE access_tokens
    DROP CONSTRAINT access_tokens_scopes,
    ADD CONSTRAINT access_tokens_scopes CHECK (
        cardinality(scopes) > 0
        AND scopes <@ ARRAY[
            'deploy:read',
            'deploy:write',
            'cluster:manage',
            'token:read',
            'token:write',
            'reliant:api',
            'daemon:connect',
            'llm:invoke',
            'proxy:port',
            'mcp:connector',
            'daemon:resume',
            'secret:read',
            'secret:write',
            'domain:read',
            'domain:write',
            'identity:read',
            'identity:manage',
            'obs:read',
            'obs:triage',
            'obs:write',
            'db:query',
            'billing:read'
        ]::text[]
    );
COMMENT ON CONSTRAINT access_tokens_scopes ON access_tokens IS
    'forge:closed-set accesstoken.AllScopes';

