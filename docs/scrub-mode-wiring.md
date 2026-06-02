# Scrub-mode wiring (Settings dropdown → Hub)

How the Settings **Scrubbing mode** dropdown (`mask | redact | off`) actually
changes the Hub's behavior, rather than only persisting to `settings.json`.

## Investigation (scrubadubber-hub @ v0.1.3)

- **Control API (`:8384`) cannot set the mode.** It is the human-in-the-loop
  *review queue* only — `GET /healthz`, `GET /reviews`, `GET /reviews/{id}`,
  `POST /reviews/{id}/decision` (`allow|block|redact` for a single held request).
  See `api/control-plane.openapi.yaml` and `internal/review/api.go`. There is no
  endpoint for global configuration / scrubbing mode.
- **Mode is configuration.** `scrubbing.default_mode` (`mask|redact|off`) lives in
  `config.yaml`, validated at load (`internal/config/config.go`).
- **No live reload.** The Hub only handles `os.Interrupt` (shutdown); there is no
  SIGHUP/reload, so a config change requires a **Hub restart**.
- **Env override exists.** `internal/config/load.go` `applyEnvOverrides` honors
  **`SCRUB_DEFAULT_MODE`**, applied *after* the YAML is parsed — so the env value
  **wins over** `config.yaml`'s `default_mode`.

## Chosen design: `SCRUB_DEFAULT_MODE` env + restart-on-change

The tray already owns the Hub process and restarts it when settings change, so:

1. `buildHub` launches the Hub with `SCRUB_DEFAULT_MODE=<settings.Mode>`.
2. `applySettings` restarts the Hub when `Mode` changes (already wired via
   `reconfigureHub`), so the new mode takes effect.

Properties:

- **No YAML dependency** in this repo and **no Hub change** required.
- The **dropdown is the single source of truth** for mode (its env override beats
  `config.yaml`). All *other* scrubbing settings — detection layers, masking
  scope, rule packs — stay in `config.yaml`, editable via **Open config file**.
- Validation is safe: `settings.Mode` is always one of `mask|redact|off`, which
  the Hub also accepts.

### Alternatives considered

| Option | Why not |
|--------|---------|
| Edit `config.yaml` `default_mode` + restart | Needs a YAML dependency here, and the env override would shadow it anyway. |
| Control-API call | No mode endpoint exists in the control plane. |
| Live reload (SIGHUP) | The Hub doesn't support it; a restart is required regardless. |

## Implementation

`cmd/scrubadubber/main.go` `buildHub`:

```go
Env: []string{"SCRUB_DEFAULT_MODE=" + string(s.Mode)},
```

The existing `applySettings` → `reconfigureHub` path restarts the Hub on a mode
change, so the override is re-read on the next launch.

## Future

If the Hub later gains a control-API mode endpoint or live config reload, switch
to it to avoid the brief restart blip when the mode changes.
