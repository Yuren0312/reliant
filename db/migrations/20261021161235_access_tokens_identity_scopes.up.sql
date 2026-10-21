-- forge:backward-compatible — the CHECK only widens: every row the previous release can write still passes, and it never writes the two new scopes.

-- access_tokens.scopes is pinned to forge's closed scope set
-- (accesstoken.AllScopes; TestAccessTokensScopesCheckMatchesForge). The forge
-- pin at 66ebe7fd added identity:read and identity:manage, so minting a token
-- with either failed this CHECK. Widen it to the set the pinned forge defines.
ALTER TABLE access_tokens DROP CONSTRAINT access_tokens_scopes;
ALTER TABLE access_tokens ADD CONSTRAINT access_tokens_scopes CHECK (
    cardinality(scopes) > 0
    AND scopes <@ ARRAY[
        'deploy:read', 'deploy:write', 'cluster:manage',
        'token:read', 'token:write',
        'reliant:api', 'daemon:connect', 'llm:invoke', 'proxy:port', 'mcp:connector', 'daemon:resume',
        'secret:read', 'secret:write',
        'domain:read', 'domain:write',
        'identity:read', 'identity:manage'
    ]::text[]
);
