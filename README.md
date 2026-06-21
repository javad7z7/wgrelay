# WgRelay

A cross-platform desktop app (macOS + Windows) that connects to a **WireGuard**
config and exposes it as a local **SOCKS5** and **HTTP** proxy — instead of
routing all your system traffic like the official WireGuard client does.

Only the apps you point at `127.0.0.1:<port>` go through the VPN. Everything
else on your machine is untouched. No admin/root, no `utun`/TUN driver, no
routing-table changes — the tunnel runs entirely in userspace
(`wireguard-go` + gVisor netstack).

## How it's built

Two independent pieces communicate via a spawned child process and CLI flags —
**not** a shared library or RPC:

- **Sidecar** (`sidecar/`, Go): the userspace WireGuard tunnel + SOCKS5/HTTP
  proxy. A single static binary driven entirely by flags (`-config`, `-socks`,
  `-http`, `-user`, `-pass`, `-dns`, `-verbose`). Knows nothing about Electron.
- **GUI** (`src/`, Electron): spawns/kills the sidecar, manages the config file
  and ports, renders status + a live log. Pure HTML/CSS/vanilla JS in the
  renderer — **no build step, no framework, no bundler**, easy to restyle.

```
src/main.js            Electron main: spawn/kill sidecar, IPC, settings
src/preload.js         contextBridge API surface (window.api)
src/renderer/          index.html · styles.css · renderer.js
sidecar/*.go           the Go proxy (wireguard.go, socks5.go, httpproxy.go, …)
scripts/build-sidecar  compiles the Go binary into resources/<plat>/
scripts/build-icons    generates icon.icns / icon.ico / icon.png in build/
```

**TCP only** (SOCKS5 CONNECT + HTTP/HTTPS CONNECT/forward). DNS for proxied
hostnames goes through the tunnel; if the WG config has no `DNS` line, the
sidecar falls back to `1.1.1.1`.

## Prerequisites

- [Node.js](https://nodejs.org) 18+
- [Go](https://go.dev/dl/) 1.23+ (only needed to build the sidecar)

## Develop / run locally

```sh
npm install
npm start        # builds the sidecar for your OS, then launches Electron
```

`npm start` always runs `scripts/build-sidecar.js` first, which cross-compiles
with `CGO_ENABLED=0` into `resources/<plat>/`:

- **macOS**: universal binary (arm64 + amd64 merged with `lipo`) → `resources/mac/wgrelay`
- **Windows**: `resources/win/wgrelay.exe`
- **Linux**: `resources/linux/wgrelay` (dev convenience only — not a release target)

### Working on the sidecar alone

It's a normal Go module. You can build and run it without Electron:

```sh
cd sidecar
go build -o wgrelay .
./wgrelay -config /path/to/wg.conf -socks 127.0.0.1:1080 -http 127.0.0.1:8080 -verbose
```

Verify with:

```sh
curl --socks5-hostname 127.0.0.1:1080 https://ifconfig.me
curl -x http://127.0.0.1:8080         https://ifconfig.me
```

## Package installers

```sh
npm run dist        # current OS
npm run dist:mac    # .dmg + .zip (arm64 + x64)
npm run dist:win    # NSIS installer + portable .exe (x64)
```

Output lands in `release/`. Note: you can only build macOS artifacts on macOS,
and Windows artifacts on Windows (or via the CI below).

## Publish GitHub Releases (CI)

`.github/workflows/release.yml` builds on both `macos-latest` and
`windows-latest` and uploads artifacts to a **draft** GitHub Release. It is
already wired to publish to `javad7z7/wgrelay` (see `build.publish` in
`package.json`). The built-in `GITHUB_TOKEN` is used automatically — no extra
secrets needed for plain (unsigned) releases.

To cut a release, bump the `version` in `package.json`, commit, then push a
matching tag:

```sh
git tag v0.2.0
git push origin v0.2.0
```

CI creates the draft with the `.dmg`, `.zip`, `.exe`, and the auto-update
metadata (`latest*.yml`). Review it on GitHub and hit Publish.

## Installing a release (unsigned)

These builds are **not code-signed**, so a freshly downloaded app shows a
security warning on first launch. Clear it once:

- **macOS**: right-click the app → **Open** → **Open**. If macOS says the app
  is *"damaged and can't be opened"*, the download was quarantined; clear it
  with:
  ```sh
  xattr -cr /Applications/WgRelay.app
  ```
- **Windows**: on the SmartScreen dialog, click **More info → Run anyway**.

Removing these warnings entirely requires paid certificates — see *Code
signing* below.

## Code signing (for clean distribution)

- **macOS**: set `CSC_LINK` / `CSC_KEY_PASSWORD` (Developer ID cert) and
  configure notarization (`APPLE_ID`, `APPLE_APP_SPECIFIC_PASSWORD`,
  `APPLE_TEAM_ID`) as CI secrets. The bundled Go binary is signed along with
  the app.
- **Windows**: set a code-signing cert via `CSC_LINK` / `CSC_KEY_PASSWORD` to
  avoid SmartScreen warnings.

## App icon

Platform artifacts live in `build/`:

- `build/icon.icns` — macOS bundle icon
- `build/icon.ico`  — Windows bundle icon
- `build/icon.png`  — generic fallback (also used for Linux dev builds)

To replace the icon, drop a fresh set of source PNGs into `build/`
(`icon-16.png`, `-32`, `-64`, `-128`, `-256`, `-512`, `-1024`) and run:

```sh
npm run build:icons
```

This regenerates `icon.icns` / `icon.ico` / `icon.png` and refreshes the
in-app favicon assets in `src/renderer/`. `electron-builder` auto-discovers the
generated files — no extra config needed.

## Usage

1. Click **browse** and select a standard WireGuard `.conf`.
2. Set the SOCKS5 / HTTP ports (defaults `1080` / `8080`).
3. **connect**. Point your apps at the proxy:

```sh
curl --socks5-hostname 127.0.0.1:1080 https://ifconfig.me
curl -x http://127.0.0.1:8080         https://ifconfig.me
```

Settings persist as JSON in Electron's `userData` directory
(`settings.json`). Stopping the proxy is a hard `child.kill()` of the sidecar —
safe precisely *because* the userspace tunnel makes no system changes.

## Limitations

- **TCP only** (SOCKS5 CONNECT + HTTP/HTTPS) — covers browsers, curl, most
  apps. No UDP-over-SOCKS.
- This is the same approach as [`wireproxy`](https://github.com/pufferffish/wireproxy);
  the sidecar is a compact implementation you fully own.

## License

MIT — see [`LICENSE`](./LICENSE).
