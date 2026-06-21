#!/usr/bin/env node
// Builds the Go sidecar for the CURRENT platform into resources/<plat>/.
// On macOS it produces a universal (arm64 + x64) binary via lipo.
// Run automatically by `npm start` / `npm run dist`.

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const root = path.join(__dirname, "..");
const sidecar = path.join(root, "sidecar");
const isWin = process.platform === "win32";
const isMac = process.platform === "darwin";

function sh(cmd, env) {
  console.log("> " + cmd);
  execSync(cmd, {
    cwd: sidecar,
    stdio: "inherit",
    env: { ...process.env, ...env },
  });
}

// Make sure module deps are present.
sh("go mod tidy");

if (isMac) {
  const outDir = path.join(root, "resources", "mac");
  fs.mkdirSync(outDir, { recursive: true });
  sh("go build -trimpath -o wgrelay-arm64 .", {
    GOOS: "darwin",
    GOARCH: "arm64",
    CGO_ENABLED: "0",
  });
  sh("go build -trimpath -o wgrelay-amd64 .", {
    GOOS: "darwin",
    GOARCH: "amd64",
    CGO_ENABLED: "0",
  });
  sh(
    `lipo -create -output "${path.join(outDir, "wgrelay")}" wgrelay-arm64 wgrelay-amd64`,
  );
  fs.rmSync(path.join(sidecar, "wgrelay-arm64"), { force: true });
  fs.rmSync(path.join(sidecar, "wgrelay-amd64"), { force: true });
  fs.chmodSync(path.join(outDir, "wgrelay"), 0o755);
  console.log("Built universal macOS sidecar -> resources/mac/wgrelay");
} else if (isWin) {
  const outDir = path.join(root, "resources", "win");
  fs.mkdirSync(outDir, { recursive: true });
  sh(`go build -trimpath -o "${path.join(outDir, "wgrelay.exe")}" .`, {
    GOOS: "windows",
    GOARCH: "amd64",
    CGO_ENABLED: "0",
  });
  console.log("Built Windows sidecar -> resources/win/wgrelay.exe");
} else {
  // Linux (handy for local dev); not a release target here.
  const outDir = path.join(root, "resources", "linux");
  fs.mkdirSync(outDir, { recursive: true });
  sh(`go build -trimpath -o "${path.join(outDir, "wgrelay")}" .`, {
    CGO_ENABLED: "0",
  });
  fs.chmodSync(path.join(outDir, "wgrelay"), 0o755);
  console.log("Built Linux sidecar -> resources/linux/wgrelay");
}
