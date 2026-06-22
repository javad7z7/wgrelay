const {
  app,
  BrowserWindow,
  ipcMain,
  dialog,
  Menu,
  nativeImage,
  Tray,
} = require("electron");
const { spawn } = require("child_process");
const path = require("path");
const fs = require("fs");

let win = null;
let child = null; // the running wgrelay sidecar process
let tray = null;
let proxyState = "disconnected";

// ---- sidecar binary location -------------------------------------------------
function platformDir() {
  if (process.platform === "win32") return "win";
  if (process.platform === "darwin") return "mac";
  return "linux";
}
function binaryName() {
  return process.platform === "win32" ? "wgrelay.exe" : "wgrelay";
}
function sidecarPath() {
  const name = binaryName();
  return app.isPackaged
    ? path.join(process.resourcesPath, "bin", name)
    : path.join(app.getAppPath(), "resources", platformDir(), name);
}

// ---- tiny JSON settings store ------------------------------------------------
function settingsFile() {
  return path.join(app.getPath("userData"), "settings.json");
}
function loadSettings() {
  try {
    return JSON.parse(fs.readFileSync(settingsFile(), "utf8"));
  } catch {
    return {
      configPath: "",
      socks: "127.0.0.1:1080",
      http: "127.0.0.1:8080",
      user: "",
      pass: "",
      dns: "",
    };
  }
}
function saveSettings(s) {
  try {
    fs.writeFileSync(settingsFile(), JSON.stringify(s, null, 2));
  } catch (e) {
    console.error("saveSettings:", e);
  }
  return true;
}

// ---- status helpers ----------------------------------------------------------
function send(channel, payload) {
  if (win && !win.isDestroyed()) win.webContents.send(channel, payload);
}
function setStatus(state, detail) {
  proxyState = state;
  send("status", { state, detail: detail || "" });
  updateTray();
}

// ---- macOS menu bar ---------------------------------------------------------
function showWindow() {
  if (!win || win.isDestroyed()) createWindow();
  if (win.isMinimized()) win.restore();
  win.show();
  win.focus();
}

function trayIcon(active) {
  // Electron can return an empty nativeImage for inline SVGs on macOS.
  // Use real PNG assets so the status item is always visible, swapping between
  // the active (connected) and idle (disconnected) artwork.
  const file = active ? "menubar-active.png" : "menubar-idle.png";
  return nativeImage
    .createFromPath(path.join(__dirname, "renderer", "menubar", file))
    .resize({ width: 18, height: 18 });
}

function toggleProxyFromTray() {
  if (child) {
    stopProxy();
    return;
  }

  const result = startProxy(loadSettings());
  if (!result.ok) {
    setStatus("error", result.error);
    dialog.showErrorBox("WgRelay", result.error);
  }
}

function updateTray() {
  if (!tray) return;

  const active = proxyState === "connected" || proxyState === "connecting";
  const busy = proxyState === "disconnecting";
  const status = proxyState[0].toUpperCase() + proxyState.slice(1);
  tray.setImage(trayIcon(active));
  tray.setToolTip(`WgRelay - ${status}`);
  tray.setContextMenu(
    Menu.buildFromTemplate([
      { label: `Status: ${status}`, enabled: false },
      { type: "separator" },
      {
        label: active ? "Disconnect" : "Connect",
        enabled: !busy,
        click: toggleProxyFromTray,
      },
      { label: "Show WgRelay", click: showWindow },
      { type: "separator" },
      { label: "Quit WgRelay", click: () => app.quit() },
    ]),
  );
}

function createTray() {
  if (process.platform !== "darwin" || tray) return;
  tray = new Tray(trayIcon(false));
  updateTray();
}

// ---- process control ---------------------------------------------------------
function startProxy(opts) {
  if (child) return { ok: false, error: "Already running" };

  const bin = sidecarPath();
  if (!fs.existsSync(bin)) {
    return {
      ok: false,
      error:
        "Sidecar binary not found at " + bin + ". Run `npm run build:sidecar`.",
    };
  }
  if (!opts.configPath || !fs.existsSync(opts.configPath)) {
    return { ok: false, error: "Pick a valid WireGuard .conf file first." };
  }

  const args = ["-config", opts.configPath];
  args.push("-socks", opts.socks || "");
  args.push("-http", opts.http || "");
  if (opts.user) args.push("-user", opts.user);
  if (opts.pass) args.push("-pass", opts.pass);
  if (opts.dns) args.push("-dns", opts.dns);

  setStatus("connecting");
  try {
    child = spawn(bin, args, { windowsHide: true });
  } catch (e) {
    child = null;
    setStatus("error", String(e));
    return { ok: false, error: String(e) };
  }

  const onData = (buf) => {
    const text = buf.toString();
    text
      .split(/\r?\n/)
      .filter(Boolean)
      .forEach((line) => {
        send("log", line);
        if (/listening on/i.test(line)) setStatus("connected");
      });
  };
  child.stdout.on("data", onData);
  child.stderr.on("data", onData);

  child.on("exit", (code, signal) => {
    send("log", `[sidecar exited] code=${code} signal=${signal || "-"}`);
    child = null;
    setStatus("disconnected");
  });
  child.on("error", (err) => {
    send("log", "[sidecar error] " + err.message);
    child = null;
    setStatus("error", err.message);
  });

  return { ok: true };
}

function stopProxy() {
  if (!child) return { ok: true };
  setStatus("disconnecting");
  // Userspace tunnel makes no system changes, so a hard kill is safe.
  child.kill();
  return { ok: true };
}

// ---- IPC ---------------------------------------------------------------------
ipcMain.handle("pick-config", async () => {
  const res = await dialog.showOpenDialog(win, {
    title: "Select a WireGuard config",
    properties: ["openFile"],
    filters: [
      { name: "WireGuard config", extensions: ["conf"] },
      { name: "All files", extensions: ["*"] },
    ],
  });
  if (res.canceled || res.filePaths.length === 0) return null;
  return res.filePaths[0];
});
ipcMain.handle("get-settings", () => loadSettings());
ipcMain.handle("save-settings", (_e, s) => saveSettings(s));
ipcMain.handle("start-proxy", (_e, opts) => startProxy(opts));
ipcMain.handle("stop-proxy", () => stopProxy());
ipcMain.handle("is-running", () => !!child);
ipcMain.handle("get-version", () => app.getVersion());

// ---- window ------------------------------------------------------------------
function createWindow() {
  win = new BrowserWindow({
    width: 520,
    height: 720,
    minWidth: 460,
    minHeight: 600,
    title: "WgRelay",
    show: false,
    icon: path.join(__dirname, "renderer", "icon-256.png"),
    backgroundColor: "#0c0f0d",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  win.setMenuBarVisibility(false);
  win.once("ready-to-show", () => win.show());
  win.loadFile(path.join(__dirname, "renderer", "index.html"));
}

app.whenReady().then(() => {
  if (process.platform === "darwin" && app.dock) {
    app.dock.setIcon(path.join(__dirname, "renderer", "icon-256.png"));
  }
  createWindow();
  createTray();
});

app.on("activate", () => {
  if (BrowserWindow.getAllWindows().length === 0) createWindow();
});
app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
app.on("before-quit", () => {
  if (child) child.kill();
});
