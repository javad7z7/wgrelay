# WgRelay

A cross-platform desktop app (macOS + Windows) that connects to a **WireGuard**
config and exposes it as a local **SOCKS5** and **HTTP** proxy — instead of
routing all your system traffic like the official WireGuard client does.

Only the apps you point at `127.0.0.1:<port>` go through the VPN. Everything
else on your machine is untouched. No admin/root, no `utun`/TUN driver, no
routing-table changes — the tunnel runs entirely in userspace
(`wireguard-go` + gVisor netstack).

> 🇮🇷 برای راهنمای فارسی [اینجا را ببینید](#فارسی).

---

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

---

<div dir="rtl" lang="fa">

<a id="فارسی"></a>

# WgRelay — راهنمای فارسی

یک برنامهٔ دسکتاپ چندسکویی (macOS + Windows) که با گرفتن یک فایل پیکربندی
**WireGuard**، آن را به‌صورت یک پروکسی محلی **SOCKS5** و **HTTP** در دسترس قرار
می‌دهد — به‌جای اینکه مثل کلاینت رسمی WireGuard کل ترافیک سیستم را از تونل عبور
دهد.

فقط برنامه‌هایی که آن‌ها را به `127.0.0.1:<port>` وصل می‌کنید از VPN استفاده
می‌کنند و باقی سیستم بدون تغییر می‌ماند. نیازی به دسترسی ادمین/روت، درایور
`utun`/TUN، یا تغییر جدول مسیر (routing table) نیست — تونل کاملاً در فضای کاربر
(`wireguard-go` + gVisor netstack) اجرا می‌شود.

## معماری

پروژه از دو بخش مستقل تشکیل شده که فقط از طریق اجرای فرایند فرزند و آرگومان‌های
خط فرمان با هم در ارتباطند (نه کتابخانهٔ مشترک و نه RPC):

- **Sidecar** (`sidecar/`، Go): تونل WireGuard در فضای کاربر به‌علاوهٔ سرور
  SOCKS5/HTTP. یک باینری استاتیک که فقط با فلگ‌های CLI کنترل می‌شود
  (`-config`, `-socks`, `-http`, `-user`, `-pass`, `-dns`, `-verbose`).
- **GUI** (`src/`، Electron): مسئول اجرا و توقف sidecar، مدیریت فایل پیکربندی و
  پورت‌ها، و نمایش وضعیت و لاگ زنده. لایهٔ ظاهری با HTML/CSS/JavaScript خالص
  نوشته شده — **بدون مرحلهٔ بیلد، بدون فریم‌ورک، بدون باندلر**.

فقط **TCP** پشتیبانی می‌شود (SOCKS5 CONNECT + HTTP/HTTPS). DNS برای نام‌های
دامنه‌ای از داخل تونل پرس‌وجو می‌شود؛ اگر فایل کانفیگ خط `DNS` نداشته باشد،
به‌صورت پیش‌فرض از `1.1.1.1` استفاده می‌شود.

## پیش‌نیازها

- [Node.js](https://nodejs.org) نسخهٔ ۱۸ به بالا
- [Go](https://go.dev/dl/) نسخهٔ ۱.۲۳ به بالا (فقط برای بیلد sidecar)

## اجرای محلی

```sh
npm install
npm start
```

دستور `npm start` ابتدا `scripts/build-sidecar.js` را اجرا می‌کند تا باینری Go
برای سیستم‌عامل فعلی ساخته شود و سپس Electron اجرا می‌شود.

## ساخت نصاب (Installer)

```sh
npm run dist        # سیستم‌عامل فعلی
npm run dist:mac    # خروجی .dmg و .zip برای arm64 و x64
npm run dist:win    # نصاب NSIS و نسخهٔ portable برای x64
```

خروجی‌ها در پوشهٔ `release/` قرار می‌گیرند. توجه داشته باشید که خروجی macOS فقط
روی macOS و خروجی Windows فقط روی Windows ساخته می‌شود (یا از طریق CI).

## انتشار رسمی (GitHub Releases)

فایل `.github/workflows/release.yml` روی هر دو ranner ‏`macos-latest` و
`windows-latest` بیلد می‌گیرد و خروجی‌ها را به یک Release **پیش‌نویس** در
GitHub آپلود می‌کند. برای انتشار نسخهٔ جدید کافی است:

```sh
# پس از بالا بردن version در package.json و commit کردن:
git tag v0.2.0
git push origin v0.2.0
```

سپس در GitHub رفته و Release را Publish کنید.

## نصب نسخهٔ منتشرشده (بدون امضای دیجیتال)

چون این بیلدها به‌صورت پیش‌فرض **امضای کد ندارند**، هنگام اولین اجرا پیام
امنیتی ظاهر می‌شود. یک‌بار آن را پاک کنید:

- **macOS**: روی برنامه راست‌کلیک کنید → **Open** → **Open**. اگر پیام «damaged
  and can't be opened» دیدید یعنی فایل قرنطینه شده — با دستور زیر پاکش کنید:
  ```sh
  xattr -cr /Applications/WgRelay.app
  ```
- **Windows**: در دیالوگ SmartScreen روی **More info → Run anyway** بزنید.

## آیکون برنامه

برای تعویض آیکون، هفت فایل PNG با سایزهای ۱۶، ۳۲، ۶۴، ۱۲۸، ۲۵۶، ۵۱۲ و ۱۰۲۴ را
(با نام‌های `icon-16.png` تا `icon-1024.png`) داخل پوشهٔ `build/` قرار دهید و
سپس اجرا کنید:

```sh
npm run build:icons
```

این دستور به‌طور خودکار فایل‌های `icon.icns` (مک)، `icon.ico` (ویندوز) و
`icon.png` (Fallback) را تولید می‌کند و آیکون داخل برنامه را هم به‌روزرسانی
می‌کند.

## نحوهٔ استفاده

۱. دکمهٔ **browse** را بزنید و یک فایل استاندارد `.conf` مربوط به WireGuard
   انتخاب کنید.

۲. پورت‌های SOCKS5 و HTTP را تنظیم کنید (پیش‌فرض `1080` و `8080`).

۳. روی **connect** بزنید و برنامه‌های موردنظرتان را به پروکسی متصل کنید:

```sh
curl --socks5-hostname 127.0.0.1:1080 https://ifconfig.me
curl -x http://127.0.0.1:8080         https://ifconfig.me
```

تنظیمات به‌صورت JSON در پوشهٔ `userData` مربوط به Electron ذخیره می‌شوند
(`settings.json`). توقف پروکسی معادل `child.kill()` کردن sidecar است که چون
تونل هیچ تغییری در سیستم اعمال نکرده، کاملاً امن است.

## محدودیت‌ها

- فقط **TCP** پشتیبانی می‌شود (SOCKS5 CONNECT + HTTP/HTTPS) — برای مرورگرها،
  curl و بیشتر برنامه‌ها کافی است. UDP-over-SOCKS پشتیبانی نمی‌شود.
- این روش مشابه پروژهٔ [`wireproxy`](https://github.com/pufferffish/wireproxy)
  است؛ sidecar این پروژه یک پیاده‌سازی فشرده و کاملاً در اختیار خودتان است.

## مجوز

MIT — فایل [`LICENSE`](./LICENSE) را ببینید.

</div>
