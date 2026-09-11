# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`subsportal` (binary/rc name: `captiveportal`) is a Go HTTP captive-portal server for a prepaid WiFi service ("Arknet"). Guests connect to a router, get redirected here, enter a voucher code, and on success the app triggers a firewall (OpenBSD `pf`) rule reload so the client's IP is allowed through. It's designed to run on OpenBSD alongside a separate privileged companion daemon that actually executes `pfctl`/file moves on its behalf.

## Build & run

```sh
make build   # go mod tidy && go build -o captiveportal
make install # copies binary + templates/ into $HOME/captiveportal
make rc      # installs rc.captiveportal to /etc/rc.d (OpenBSD rc.d service)
make dist    # build + install + tarball for deployment
make clean   # removes built binary, install dir, and rc.d script
```

There are no Go tests in this repo (`go test ./...` currently has nothing to run).

The server reads config from a `.env` file (via `godotenv`) in the working directory at runtime. Required variables (see `auth.GetEnvVariable` call sites): `APP_IP`, `APP_PORT`, `API_URL`, `API_AUTH`, `RUN_DIR`, `ARKGATE_ADDR`, `ARKGATE_TLS_CERT`, `ARKGATE_TLS_KEY`, `ARKGATE_TLS_CA`, `ROUTER_ID`, `ROUTER_INDEX`. Missing `.env` is fatal (`log.Fatal`), not just a missing key.

`daemon_manager.ksh <daemon> <rundir>` is a watchdog loop (checks `ps aux | grep` count, restarts if not running) intended to be run separately, not part of the Go build.

## Architecture

**Request flow (`main.go`)**: a single `/` handler (`serveTemplate`) serves `templates/base.tmpl` + `templates/index.tmpl`. GET requests check for a `code` cookie and, if present, re-validate it against the upstream API before redirecting the client out (currently to `https://www.google.com`). POST requests take the `voucher` form field, validate it the same way, and on success set a 32-day `code` cookie and redirect; on failure they re-render the form with an error message.

**Voucher validation & pf reload (`main.go:validateCode`, `auth/auth.go`)**: validating a code calls the upstream API (`auth.ValidateCode`, hits `<API_URL>vouchers/value/<code>`) using a bearer JWT obtained via `auth.GetToken()` (`<API_URL>login` with Basic auth from `API_AUTH`). On success, the app acquires the shared lock and runs a fixed pf.conf swap sequence over an mTLS connection to a separate privileged daemon (`arkgated`): `check` (`pfctl -nf` on a staged file in `RUN_DIR`) → `backup` (`mv /etc/pf.conf /etc/pf.conf.prev`) → `move` (staged file → `/etc/pf.conf`) → `apply` (`pfctl -f /etc/pf.conf`), with `revert` (restore `.prev`) run if `apply` fails. This app never runs `pfctl` itself — it only sends JSON commands (`Arkcmd{Name, Cmd, Opts}`) over a TLS connection to `ARKGATE_ADDR`, authenticated with the client cert/key at `ARKGATE_TLS_CERT`/`ARKGATE_TLS_KEY` and verified against `ARKGATE_TLS_CA`, and expects an `"OK"` reply. Both call sites go through `auth.SendArkgateCmd` (`auth/auth.go`), which dials via `auth.GetArkgateConn` and returns an error instead of panicking if arkgated or its TLS config is unavailable — never call `cmd.SendCmd` with a raw connection directly.

**Background reloader (`auth.PfReloader`, started as a goroutine from `main.go`)**: polls `<API_URL>runtime/query/updatepf/<ROUTER_INDEX>` every 120s; on a 200 response it runs the same check/backup/move/apply/revert sequence, and on success calls `<API_URL>runtime/delete/<ROUTER_INDEX>` to acknowledge. It also refreshes the JWT in-loop via `refreshToken`, which checks the token's `exp` claim (`CheckExpirationWithoutVerify`, signature not verified — this app only reads the token it was issued, not the API's key).

**Locking (`locker/lock.go`)**: `locker.Lock` is a single shared `bool`, not a `sync.Mutex`. `GetLock`/`SetLock` just read/write the pointer with logging; callers busy-wait (`for GetLock(...) { time.Sleep(50ms) }`) before setting it. It coordinates the HTTP handler and `PfReloader` goroutine so they don't swap `pf.conf` concurrently — it is not a general-purpose concurrency primitive and isn't actually atomic.

**`cmd/cmd.go` is dead code**: it defines the same `Arkcmd` type and pf command builders as the external module `github.com/rbaylon/arkgatecmd` (imported in `main.go`/`auth.go` as `Acmd`). All actual usage goes through the external `Acmd` package, not this local `cmd` package — don't assume changes to `cmd/cmd.go` affect runtime behavior, and don't confuse the two nearly-identical APIs when reading call sites.

**Templates (`templates/`)**: plain `html/template`, loaded once at startup from `./templates/base.tmpl` + `./templates/index.tmpl` (paths are relative to the process's working directory, matching how `make install`/`make dist` lay out the deployed directory). `base.tmpl` defines the page shell and includes the `index` template; `index.tmpl` defines both `title` and `index`. Static assets would be served from `./static/` via `http.FileServer`, though no `static/` directory exists in the repo currently.

## Deployment shape

This service is meant to run unprivileged, delegating all privileged firewall operations to a separate daemon (`arkgated`) reachable over mutual-TLS at `ARKGATE_ADDR` — keep that separation when touching `auth.go`/`main.go`: this codebase should only ever *request* pf changes via the mTLS command protocol, never shell out to `pfctl` directly. The client cert/key (`ARKGATE_TLS_CERT`/`ARKGATE_TLS_KEY`) and the CA used to verify arkgated's server cert (`ARKGATE_TLS_CA`) must be provisioned on the host alongside `.env`; treat the key file with the same care as a credential (restrictive permissions, not committed to the repo).
