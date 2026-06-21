const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("api", {
  pickConfig: () => ipcRenderer.invoke("pick-config"),
  getSettings: () => ipcRenderer.invoke("get-settings"),
  saveSettings: (s) => ipcRenderer.invoke("save-settings", s),
  start: (opts) => ipcRenderer.invoke("start-proxy", opts),
  stop: () => ipcRenderer.invoke("stop-proxy"),
  isRunning: () => ipcRenderer.invoke("is-running"),
  getVersion: () => ipcRenderer.invoke("get-version"),
  onStatus: (cb) => ipcRenderer.on("status", (_e, s) => cb(s)),
  onLog: (cb) => ipcRenderer.on("log", (_e, line) => cb(line)),
});
