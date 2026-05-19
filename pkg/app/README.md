# Configuration System (`pkg/app`)

## Overview

The shell-operator configuration system provides a **three-layer** approach
for setting every runtime parameter:

```
CLI flag (explicit)  >  environment variable  >  hardcoded default
```

The highest-priority source that supplies a value wins.
The implementation lives in three files:

| File | Responsibility |
|---|---|
| `app_config.go` | `Config` struct, `NewConfig()` (hardcoded defaults), `ParseEnv()` (env layer), `Validate()` |
| `flags.go` | `BindFlags()` — registers cobra/pflag CLI flags whose defaults are the *already-merged* `cfg` values |
| `debug.go` | Package-level `DebugUnixSocket` global used by debug sub-commands |

## How it works

### Initialization sequence

```
main()
  │
  ├─ 1. cfg := app.NewConfig()        // hardcoded defaults
  ├─ 2. app.ParseEnv(cfg)             // env vars overlay defaults
  │
  ├─ 3. app.BindFlags(cfg, root, cmd) // register CLI flags;
  │                                    //   flag default = current cfg value
  │                                    //   (already contains env override)
  │
  └─ 4. cobra.Execute()               // parse CLI args → explicit flags
         │                             //   overwrite cfg fields directly
         └─ applySliceFlags()          // fixup for []string fields
```

The key insight: **`BindFlags` never reads environment variables itself.**
It receives a `*Config` that already has env values merged in and uses those
values as flag defaults.  When a flag is *not* passed on the command line,
cobra keeps the default — which is the env value (or the hardcoded default if
the env var is unset).  When a flag *is* explicitly passed, cobra writes the
CLI value into `cfg`, so CLI always wins.

### Step-by-step

1. **`NewConfig()`** — creates a `Config` with all hardcoded defaults
   (e.g. `ListenPort = "9115"`, `ClientQPS = 5`).

2. **`ParseEnv(cfg)`** — uses [`caarlos0/env`](https://github.com/caarlos0/env)
   to overlay environment variables on top of the existing `cfg` values.
   The library respects `envPrefix` tags on nested structs
   (`SHELL_OPERATOR_`, `KUBE_`, `DEBUG_`, etc.) and field-level `env` tags.
   A field whose env var is not set keeps its `NewConfig` default.

3. **`BindFlags(cfg, rootCmd, startCmd)`** — registers every flag on the
   `start` subcommand with `pflag.*Var(...)`. The *default* argument for each
   flag is `cfg.<Field>` — the value that already includes env overrides.
   Returns a post-parse fixup function for `[]string` slice flags.

4. **`cobra.Execute()`** → parses CLI arguments. Only *explicitly* passed
   flags modify `cfg`. Untouched flags retain the default, which is the env
   value.

5. **`applySliceFlags()`** — special handling for `ClientCA` (which is
   `[]string`). If the CLI provided values, they win; otherwise the env value
   is kept.

## Config struct

```go
type Config struct {
    App           AppSettings           `envPrefix:"SHELL_OPERATOR_"`
    Kube          KubeSettings          `envPrefix:"KUBE_"`
    ObjectPatcher ObjectPatcherSettings `envPrefix:"OBJECT_PATCHER_"`
    Admission     AdmissionSettings     `envPrefix:"VALIDATING_WEBHOOK_"`
    Conversion    ConversionSettings    `envPrefix:"CONVERSION_WEBHOOK_"`
    Debug         DebugSettings         `envPrefix:"DEBUG_"`
    Log           LogSettings           `envPrefix:"LOG_"`
}
```

Each nested struct uses its own `envPrefix`, and each field inside uses a short
`env:"..."` tag.  The final environment variable name is
**`<prefix><field_tag>`**, for example:

| Struct | Prefix | Field tag | Env variable |
|---|---|---|---|
| `AppSettings` | `SHELL_OPERATOR_` | `HOOKS_DIR` | `SHELL_OPERATOR_HOOKS_DIR` |
| `KubeSettings` | `KUBE_` | `CLIENT_QPS` | `KUBE_CLIENT_QPS` |
| `LogSettings` | `LOG_` | `LEVEL` | `LOG_LEVEL` |
| `DebugSettings` | `DEBUG_` | `KEEP_TMP_FILES` | `DEBUG_KEEP_TMP_FILES` |

## Full reference: flags ↔ env vars ↔ defaults

### Application

| Flag | Env variable | Default |
|---|---|---|
| `--hooks-dir` | `SHELL_OPERATOR_HOOKS_DIR` | `hooks` |
| `--tmp-dir` | `SHELL_OPERATOR_TMP_DIR` | `/tmp/shell-operator` |
| `--listen-address` | `SHELL_OPERATOR_LISTEN_ADDRESS` | `0.0.0.0` |
| `--listen-port` | `SHELL_OPERATOR_LISTEN_PORT` | `9115` |
| `--prometheus-metrics-prefix` | `SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX` | `shell_operator_` |
| `--namespace` | `SHELL_OPERATOR_NAMESPACE` | *(empty)* |

### Kubernetes

| Flag | Env variable | Default |
|---|---|---|
| `--kube-context` | `KUBE_CONTEXT` | *(empty)* |
| `--kube-config` | `KUBE_CONFIG` | *(empty)* |
| `--kube-server` | `KUBE_SERVER` | *(empty)* |
| `--kube-client-qps` | `KUBE_CLIENT_QPS` | `5` |
| `--kube-client-burst` | `KUBE_CLIENT_BURST` | `10` |

### Object Patcher

| Flag | Env variable | Default |
|---|---|---|
| `--object-patcher-kube-client-qps` | `OBJECT_PATCHER_KUBE_CLIENT_QPS` | `5` |
| `--object-patcher-kube-client-burst` | `OBJECT_PATCHER_KUBE_CLIENT_BURST` | `10` |
| `--object-patcher-kube-client-timeout` | `OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT` | `10s` |

### Validating Webhook

| Flag | Env variable | Default |
|---|---|---|
| `--validating-webhook-configuration-name` | `VALIDATING_WEBHOOK_CONFIGURATION_NAME` | `shell-operator-hooks` |
| `--validating-webhook-service-name` | `VALIDATING_WEBHOOK_SERVICE_NAME` | `shell-operator-validating-svc` |
| `--validating-webhook-server-cert` | `VALIDATING_WEBHOOK_SERVER_CERT` | `/validating-certs/tls.crt` |
| `--validating-webhook-server-key` | `VALIDATING_WEBHOOK_SERVER_KEY` | `/validating-certs/tls.key` |
| `--validating-webhook-ca` | `VALIDATING_WEBHOOK_CA` | `/validating-certs/ca.crt` |
| `--validating-webhook-client-ca` | `VALIDATING_WEBHOOK_CLIENT_CA` | *(empty)* |
| `--validating-webhook-failure-policy` | `VALIDATING_WEBHOOK_FAILURE_POLICY` | `Fail` |
| `--validating-webhook-listen-port` | `VALIDATING_WEBHOOK_LISTEN_PORT` | `9680` |
| `--validating-webhook-listen-address` | `VALIDATING_WEBHOOK_LISTEN_ADDRESS` | `0.0.0.0` |

### Conversion Webhook

| Flag | Env variable | Default |
|---|---|---|
| `--conversion-webhook-service-name` | `CONVERSION_WEBHOOK_SERVICE_NAME` | `shell-operator-conversion-svc` |
| `--conversion-webhook-server-cert` | `CONVERSION_WEBHOOK_SERVER_CERT` | `/conversion-certs/tls.crt` |
| `--conversion-webhook-server-key` | `CONVERSION_WEBHOOK_SERVER_KEY` | `/conversion-certs/tls.key` |
| `--conversion-webhook-ca` | `CONVERSION_WEBHOOK_CA` | `/conversion-certs/ca.crt` |
| `--conversion-webhook-client-ca` | `CONVERSION_WEBHOOK_CLIENT_CA` | *(empty)* |
| `--conversion-webhook-listen-port` | `CONVERSION_WEBHOOK_LISTEN_PORT` | `9681` |
| `--conversion-webhook-listen-address` | `CONVERSION_WEBHOOK_LISTEN_ADDRESS` | `0.0.0.0` |

### Logging

| Flag | Env variable | Default |
|---|---|---|
| `--log-level` | `LOG_LEVEL` | `info` |
| `--log-type` | `LOG_TYPE` | `text` |
| `--log-no-time` | `LOG_NO_TIME` | `false` |
| `--log-proxy-hook-json` | `LOG_PROXY_HOOK_JSON` | `false` |

### Debug (hidden flags)

| Flag | Env variable | Default |
|---|---|---|
| `--debug-unix-socket` | `DEBUG_UNIX_SOCKET` | `/var/run/shell-operator/debug.socket` |
| `--debug-http-addr` | `DEBUG_HTTP_SERVER_ADDR` | *(empty)* |
| `--debug-keep-tmp-files` | `DEBUG_KEEP_TMP_FILES` | `false` |
| `--debug-kubernetes-api` | `DEBUG_KUBERNETES_API` | `false` |

Use `shell-operator debug-options` to see all hidden debug flags.

## Priority behavior

### Scenario 1: Only env is set

```bash
export SHELL_OPERATOR_LISTEN_PORT=8080
shell-operator start
# → cfg.App.ListenPort = "8080"
```

`ParseEnv` writes `"8080"` into `cfg`.  `BindFlags` uses `"8080"` as the
flag default.  No CLI flag is passed, so the default (`"8080"`) is kept.

### Scenario 2: Only flag is passed

```bash
shell-operator start --listen-port=3000
# → cfg.App.ListenPort = "3000"
```

`NewConfig` sets `"9115"`.  `ParseEnv` has nothing to override.  `BindFlags`
registers the flag with default `"9115"`.  The explicit `--listen-port=3000`
overwrites `cfg.App.ListenPort`.

### Scenario 3: Both env and flag are set

```bash
export SHELL_OPERATOR_LISTEN_PORT=8080
shell-operator start --listen-port=3000
# → cfg.App.ListenPort = "3000"   ← CLI wins
```

`ParseEnv` writes `"8080"`.  `BindFlags` registers with default `"8080"`.
The explicit flag writes `"3000"`.  **CLI flag always wins.**

### Scenario 4: Neither env nor flag

```bash
shell-operator start
# → cfg.App.ListenPort = "9115"   ← hardcoded default
```

## Changing priority (env > flag vs. flag > env)

The current implementation enforces **flag > env > default**.  This is the
standard behavior expected by most CLI tools (12-factor apps still work because
in containers there are usually no CLI overrides, so env is effectively the
highest source).

If you need to **invert** the priority so that **env > flag** for specific
parameters, modify the `BindFlags` flow:

```go
// After cobra.Execute(), override cfg field from env if the env var is set,
// regardless of whether the flag was passed:
func overrideFromEnv(cfg *Config) {
    if v := os.Getenv("SHELL_OPERATOR_LISTEN_PORT"); v != "" {
        cfg.App.ListenPort = v
    }
}
```

Call this function after `cobra.Execute()` for the parameters where env must
have higher priority.

Alternatively, you can check whether a flag was explicitly changed by the user:

```go
if !cmd.Flags().Changed("listen-port") {
    // flag was not passed — env value (already in cfg) is in effect
}
```

`pflag.Changed()` returns `true` only if the flag was explicitly set on the
command line.  This is exactly how the current system works: if `Changed` is
false, the default (env value) remains.

## Handling `[]string` (slice) fields

Slice fields like `ClientCA` require special treatment because
`pflag.StringArrayVar` does not support setting a non-nil default that can be
cleanly overridden by CLI.  The approach used:

1. Before `BindFlags`, the env value is captured in a local variable.
2. `StringArrayVar` binds to a **separate** CLI-only slice with a `nil` default.
3. After `cobra.Execute()`, a fixup function checks: if the CLI slice is
   non-empty, use it; otherwise fall back to the captured env value.

```go
envClientCA := cfg.Admission.ClientCA   // save env value
var cliClientCA []string
f.StringArrayVar(&cliClientCA, "validating-webhook-client-ca", nil, "...")

// post-parse fixup:
if len(cliClientCA) > 0 {
    cfg.Admission.ClientCA = cliClientCA
} else {
    cfg.Admission.ClientCA = envClientCA
}
```

## Adding a new parameter

1. **Add a field** to the appropriate settings struct in `app_config.go`
   with an `env:"TAG_NAME"` tag.

2. **Set the default** in `NewConfig()` if a non-zero default is needed.

3. **Register the flag** in the corresponding `bind*Flags` function in
   `flags.go`:
   ```go
   f.StringVar(&cfg.App.MyParam, "my-param", cfg.App.MyParam, "Description. Can be set with $SHELL_OPERATOR_MY_PARAM.")
   ```
   The default argument must be `cfg.<field>` — the value already merged with
   env.

4. **Add a test** in `app_config_test.go` (update the "all flags" and
   "all envs" tests, and add a priority test if the behavior is non-trivial).

5. **For `[]string` fields** — follow the `ClientCA` pattern: capture the
   env value before binding, use `StringArrayVar` with `nil` default, add a
   fixup in the returned closure.

## Advantages

- **Single `Config` struct** — all configuration is in one place, easy to
  pass around, test, and inspect.
- **Declarative env mapping** — `caarlos0/env` with struct tags means no
  hand-written `os.Getenv` calls for each field; adding a new env var is one
  struct tag.
- **Deterministic priority** — the three-layer model (flag > env > default)
  is enforced structurally, not by convention. `BindFlags` receives an
  already-merged config as flag defaults, so the priority emerges from the
  initialization order.
- **No double-read of env** — environment is read once in `ParseEnv`.
  `BindFlags` does not read env at all, eliminating a class of bugs where
  env parsing is duplicated or inconsistent.
- **Testable** — `Config` is a plain struct. Tests create a `NewConfig()`,
  optionally call `ParseEnv`, then `bindAndParse` with arbitrary argv.
  No global state is mutated (except the legacy `DebugUnixSocket` global
  which is kept for backward compatibility with debug sub-commands).
- **Cobra-native** — the system uses standard cobra/pflag machinery. Flag
  help strings, type checking, and completion work out of the box.
- **Validation** — `Validate(cfg)` checks the final merged config for
  consistency (required fields, allowed values) in one place.

## Disadvantages

- **Slice fields need manual fixups** — `pflag` does not cleanly support
  "use this default unless the flag is explicitly passed" for `[]string`.
  The capture-and-fixup pattern works but adds boilerplate and cognitive
  overhead.
- **`DebugUnixSocket` global** — debug sub-commands (queue, hook, config)
  bind their `--debug-unix-socket` flag to a package-level variable instead
  of `cfg`, because they don't go through the `start` command path.  This
  is a pragmatic compromise but means there are two sources for this one value.
- **No built-in config file support** — if a `.yaml` / `.toml` config file
  is ever needed, an additional layer would have to be inserted between
  `ParseEnv` and `BindFlags`.
- **Env tag duplication** — the env variable name appears in both the struct
  tag (for `caarlos0/env`) and the flag help string (for the user).  They
  can drift if updated independently.
- **`caarlos0/env` dependency** — introduces a third-party library for env
  parsing.  A purely stdlib approach (`os.Getenv` + manual assignment) would
  have zero dependencies but more boilerplate.
- **Hidden debug flags** — debug flags are registered but marked hidden.
  Users need to know about `debug-options` subcommand or read the source
  to discover them.

## Testing

Run the configuration tests:

```bash
go test ./pkg/app/ -v -run 'TestBindFlags|TestParseEnv|TestCLIFlag|TestEnvOverrides'
```

The test suite covers:
- All flags correctly populate `Config` fields.
- All env vars correctly populate `Config` fields.
- Env vars override hardcoded defaults when no flag is passed.
- Explicit CLI flags override env vars.
