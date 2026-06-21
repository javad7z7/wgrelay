const $ = (id) => document.getElementById(id);

const fields = {
  configPath: $("configPath"),
  socks: $("socks"),
  http: $("http"),
  user: $("user"),
  pass: $("pass"),
  dns: $("dns"),
};

let running = false;

function setState(state, detail) {
  document.body.dataset.state = state;
  $("statusText").textContent = state;
  const on = state === "connected" || state === "connecting" || state === "disconnecting";
  running = state === "connected" || state === "connecting";
  $("toggleLabel").textContent = running ? "disconnect" : "connect";
  $("toggle").classList.toggle("is-on", running);
  // lock inputs while active
  Object.values(fields).forEach((el) => (el.disabled = on));
  $("browse").disabled = on;
  if (detail) appendLog("[error] " + detail, "err");
}

function appendLog(line, cls) {
  const log = $("log");
  const span = document.createElement("span");
  if (cls) span.className = cls;
  else if (/error|refused|fail/i.test(line)) span.className = "err";
  else if (/listening on|tunnel up/i.test(line)) span.className = "ok";
  span.textContent = line + "\n";
  log.appendChild(span);
  log.scrollTop = log.scrollHeight;
}

function currentOpts() {
  return {
    configPath: fields.configPath.value.trim(),
    socks: fields.socks.value.trim(),
    http: fields.http.value.trim(),
    user: fields.user.value,
    pass: fields.pass.value,
    dns: fields.dns.value.trim(),
  };
}

function persist() {
  window.api.saveSettings(currentOpts());
}

// ---- events -----------------------------------------------------------------
$("browse").addEventListener("click", async () => {
  const p = await window.api.pickConfig();
  if (p) {
    fields.configPath.value = p;
    persist();
  }
});

$("toggle").addEventListener("click", async () => {
  if (running) {
    await window.api.stop();
    return;
  }
  persist();
  const res = await window.api.start(currentOpts());
  if (!res.ok) {
    setState("error");
    appendLog("[error] " + res.error, "err");
  }
});

$("clearLog").addEventListener("click", () => ($("log").textContent = ""));

["socks", "http", "user", "pass", "dns"].forEach((k) =>
  fields[k].addEventListener("change", persist)
);

window.api.onStatus(({ state, detail }) => setState(state, detail));
window.api.onLog((line) => appendLog(line));

// ---- init -------------------------------------------------------------------
(async () => {
  const s = await window.api.getSettings();
  fields.configPath.value = s.configPath || "";
  fields.socks.value = s.socks || "127.0.0.1:1080";
  fields.http.value = s.http || "127.0.0.1:8080";
  fields.user.value = s.user || "";
  fields.pass.value = s.pass || "";
  fields.dns.value = s.dns || "";
  setState((await window.api.isRunning()) ? "connected" : "disconnected");
  $("appVersion").textContent = "v" + (await window.api.getVersion());
})();
