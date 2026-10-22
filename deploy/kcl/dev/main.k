"""
reliant's own `dev` environment: the whole product on THIS machine, as host
processes, with no control plane and no container runtime.

    forge env up dev      # migrate, then api + worker + gateway + web
    forge env down dev    # stops this checkout's stack only

WHAT RUNS. The three server roles (`reliant server api|worker|gateway`, the
same entrypoints prod runs) under air, so each one rebuilds and restarts when
a .go file changes; a `reliant db migrate up` job that gates them; and the Vite
dev server for web/. Underneath, forge runs the infrastructure as supervised
host processes (`forge.HostInfra`): postgres 16 (prod's major), NATS with
JetStream, and the Temporal dev server. Nothing here needs docker or k3d, so
it runs in a sandboxed cloud workspace and in a public clone that has no
control-plane checkout.

NO CONTROL PLANE. RELIANT_CONTROL_PLANE_URL is unset, so every server role
takes its self-hosted path (proto/reliant/config/v1/control_plane_config.proto):
reliant's own token store mints and validates daemon tokens, and LLM providers
are the per-user API keys and model endpoints entered in Settings. The API
authenticates with one shared bearer key (AUTH_MODE=apikey); forge generates it
into secrets/dev.yaml on first `forge env up dev`. Sign in to the web app with:

    grep AUTH_API_KEY secrets/dev.yaml

Then connect a daemon to this stack's gateway (the ports are the ones the
`forge env up` summary prints; ports.k derives them):

    KEY=$(sed -n 's/^AUTH_API_KEY: *//p' secrets/dev.yaml)
    curl -s localhost:<api>/reliant.v1.TokenService/CreateToken \
      -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
      -d '{"kind":"TOKEN_KIND_DAEMON","name":"dev"}'          # prints .token
    echo <token> | RELIANT_SERVER_URL=http://localhost:<api> \
      reliant daemon start --token --grpc-url http://localhost:<gateway>

and give the account a model: an API key under Settings, or a model endpoint
(any OpenAI-compatible server, Ollama included) — no provider key is needed.

control-plane's `forge env up dev` (../control-plane) remains the FULL-platform
stack — billing, the LLM gateway, managed workspaces — and needs k3d. This one
is the product alone. Until control-plane imports reliant's declarations (P8b,
research/forge-migration/12-p8-devtooling-plan.md), the two wire the same env
names by hand; keep them in step.

PARALLEL STACKS. Every port is `allocate_port(base, worktree)` (ports.k): the
primary checkout gets the bases, each linked worktree a disjoint 100-port
block, at most forge's default 8 stacks (`dev_stack.max_stacks`, the same cap
as control-plane's). Stack N owns 21N00-21N99. Each stack also keeps its own
data (postgres, NATS, Temporal under the checkout's .forge/hostinfra/), so two
worktrees never share a database, and `forge env down dev` stops only the
checkout it runs in. `forge env devstack list` shows the blocks; `forge env
devstack prune` reclaims a deleted worktree's.

LOGS. `.forge/logs/dev/<workload>.log` in this checkout, named as in
control-plane's stack: reliant-api-server.log, reliant-temporal-worker.log,
daemon-gateway.log, reliant-web.log. Host infra logs to its data dir
(.forge/hostinfra/<name>/<engine>.log).
"""

import forge
import forge.workloads as fw
import config_gen
import ..workloads as wl
import .config as appcfg
import .ports

# ── Runner ───────────────────────────────────────────────────────────────────
# air by default (hot reload). `-D host_runner=go-run` runs each role once,
# with no reload, on a machine without air.
_host_runner = option("host_runner", type="str", default="air", help="Host launch runner for the server roles: air (default, hot reload) or go-run")

_host = lambda air: str, listen: [int] -> forge.OnHost {
    forge.OnHost {
        runner = _host_runner
        if _host_runner == "air":
            air_config = air
        listen_ports = listen
    }
}

# ── Env ──────────────────────────────────────────────────────────────────────
# Each workload's env, layered last-wins: the typed config its root reaches
# (config.k, through config_gen.workloadEnvMap), then this machine's facts.
# The DSNs are credentials in the schema (a SecretRef by default); here they
# are composed from the ports above and override it, because a host process
# on this machine dials loopback, and a stale port in a secret store is
# exactly what connects an app to another stack's database.
_machine = {
    DATABASE_URL = "postgres://postgres:postgres@localhost:${ports.postgres}/reliant?sslmode=disable"
    NATS_URL = "nats://127.0.0.1:${ports.nats}"
}
_env = lambda w: fw.Workload -> {str:} {
    _cfg = config_gen.workloadEnvMap(appcfg.reliant_process, w)
    _own = w.env or {}
    _merged = {_k: _own[_k] if _k in _own else (_machine[_k] if _k in _machine else _v) for _k, _v in _cfg}
    {**_merged, **{_k: _v for _k, _v in _own if _k not in _merged}}
}

# ── The web app ──────────────────────────────────────────────────────────────
# The web app's directory, stated once: P9 moves it to frontends/web.
#
# It runs as a host PROCESS (`npm run dev`), not as a `forge.Frontend`. A
# forge.Frontend at an in-tree path is also a forge CODEGEN target, and
# `forge generate` would write forge's hooks, UI kit and buf template into
# web/ — the adoption P9 does deliberately (`git mv web frontends/web`, R0).
# Until then this stack only serves it. What that costs: forge's browser-
# console mirror and its npm-install freshness check, which the
# reliant-web-deps job below stands in for.
_web_path = "web"
_web_origin = "http://localhost:${ports.web}"

_web_deps = fw.Workload {
    name = "reliant-web-deps"
    kind = "job"
    command = ["npm", "install", "--no-audit", "--no-fund", "--loglevel=error"]
    before = ["reliant-web"]
    runtime = forge.OnHost {working_dir = _web_path}
}

_web = fw.Workload {
    name = "reliant-web"
    kind = "service"
    command = ["npm", "run", "dev", "--", "--strictPort"]
    runtime = forge.OnHost {working_dir = _web_path, listen_ports = [ports.web]}
    env = {
        # vite.config.ts reads PORT before FRONTEND_PORT.
        PORT = str(ports.web)
        # Vite proxies /reliant.v1.* and the terminal WebSocket here.
        VITE_API_URL = "http://localhost:${ports.api}"
        # What the onboarding's `reliant daemon start` command dials.
        VITE_GATEWAY_URL = "http://localhost:${ports.gateway}"
        VITE_DISABLE_TLS = "true"
        VITE_APP_URL = _web_origin
        # VITE_CONTROL_PLANE_API_URL is deliberately unset: the web app's one
        # control-plane gate (src/services/controlPlane/config.ts) then takes
        # its self-hosted path.
    }
}

# ── Bindings ─────────────────────────────────────────────────────────────────
_migrate = wl.migrate | {
    runtime = forge.OnHost {runner = "go-run"}
}
_migrate = _migrate | {env = _env(_migrate)}

_api = wl.api_server | {
    runtime = _host(".air.api.toml", [ports.api, ports.api_health])
    if _host_runner != "air":
        command = ["go", "run", "./cmd/reliant"]
    env = {PUBLIC_URL = "http://localhost:${ports.api}"}
}
_api = _api | {env = _env(_api)}

_worker = wl.temporal_worker | {
    runtime = _host(".air.worker.toml", [ports.worker_health])
    if _host_runner != "air":
        command = ["go", "run", "./cmd/reliant"]
}
_worker = _worker | {env = _env(_worker)}

_gateway = wl.daemon_gateway | {
    runtime = _host(".air.gateway.toml", [ports.gateway, ports.gateway_health])
    if _host_runner != "air":
        command = ["go", "run", "./cmd/reliant"]
}
_gateway = _gateway | {env = _env(_gateway)}

_bundle = forge.Bundle {
    project = "reliant"
    # This machine's own stack: forge creates everything it runs, so it may
    # hold plaintext dev secrets (FileSecrets).
    lifecycle = "local"
    workloads = [_migrate, _api, _worker, _gateway, _web_deps, _web]
    infra = [
        forge.HostInfra {
            name = "postgres"
            port = ports.postgres
            database = "reliant"
            # Prod's major (Cloud SQL POSTGRES_16).
            version = "16"
        }
        forge.HostInfra {
            name = "nats"
            engine = "nats"
            port = ports.nats
            nats_monitor_port = ports.nats_monitor
        }
        forge.HostInfra {
            name = "temporal"
            engine = "temporal"
            port = ports.temporal
            temporal_namespaces = ["reliant"]
            temporal_ui_port = ports.temporal_ui
        }
    ]
    secret_provider = forge.FileSecrets {
        path = "secrets/dev.yaml"
        generate = {
            "RELIANT_VAULT_KEY" = forge.GeneratedSecret {bytes = 32, encoding = "base64", prefix = "v1:"}
            "AUTH_API_KEY" = forge.GeneratedSecret {bytes = 24, encoding = "hex", prefix = "rdev_"}
        }
    }
}

output = forge.render(_bundle)
