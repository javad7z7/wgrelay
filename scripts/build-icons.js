#!/usr/bin/env node
// Generates platform icon artifacts from build/icon-{16,32,64,128,256,512,1024}.png:
//   - build/icon.icns   (macOS, via `iconutil` — macOS-only, skipped elsewhere)
//   - build/icon.ico    (Windows, via `png-to-ico`)
//   - build/icon.png    (fallback / Linux, copy of icon-1024.png)
// Also copies icon-32.png, icon-64.png, icon-256.png into src/renderer/ so the
// HTML favicon and the BrowserWindow icon: option can reference them from
// inside the asar bundle.

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const pngToIco = require("png-to-ico");

const root = path.resolve(__dirname, "..");
const buildDir = path.join(root, "build");
const rendererDir = path.join(root, "src", "renderer");

const sizes = [16, 32, 64, 128, 256, 512, 1024];
const sources = Object.fromEntries(
  sizes.map((s) => [s, path.join(buildDir, `icon-${s}.png`)]),
);

function assertSources() {
  const missing = sizes.filter((s) => !fs.existsSync(sources[s]));
  if (missing.length) {
    console.error(
      `Missing source PNGs in build/: ${missing.map((s) => `icon-${s}.png`).join(", ")}`,
    );
    process.exit(1);
  }
}

function logWrote(file) {
  const { size } = fs.statSync(file);
  const kb = (size / 1024).toFixed(1);
  console.log(`  wrote ${path.relative(root, file)} (${kb} KB)`);
}

function buildPng() {
  const out = path.join(buildDir, "icon.png");
  fs.copyFileSync(sources[1024], out);
  logWrote(out);
}

function buildIcns() {
  if (process.platform !== "darwin") {
    console.log("  skip icon.icns (iconutil is macOS-only)");
    return;
  }
  const iconset = path.join(buildDir, "icon.iconset");
  fs.rmSync(iconset, { recursive: true, force: true });
  fs.mkdirSync(iconset);

  const map = [
    [16, "icon_16x16.png"],
    [32, "icon_16x16@2x.png"],
    [32, "icon_32x32.png"],
    [64, "icon_32x32@2x.png"],
    [128, "icon_128x128.png"],
    [256, "icon_128x128@2x.png"],
    [256, "icon_256x256.png"],
    [512, "icon_256x256@2x.png"],
    [512, "icon_512x512.png"],
    [1024, "icon_512x512@2x.png"],
  ];
  for (const [size, name] of map) {
    fs.copyFileSync(sources[size], path.join(iconset, name));
  }

  const out = path.join(buildDir, "icon.icns");
  execFileSync("iconutil", ["-c", "icns", iconset, "-o", out], {
    stdio: "inherit",
  });
  fs.rmSync(iconset, { recursive: true, force: true });
  logWrote(out);
}

async function buildIco() {
  const inputs = [16, 32, 64, 128, 256].map((s) => sources[s]);
  const buf = await pngToIco(inputs);
  const out = path.join(buildDir, "icon.ico");
  fs.writeFileSync(out, buf);
  logWrote(out);
}

function copyRendererAssets() {
  for (const size of [32, 64, 256]) {
    const dest = path.join(rendererDir, `icon-${size}.png`);
    fs.copyFileSync(sources[size], dest);
    logWrote(dest);
  }
}

(async () => {
  assertSources();
  console.log("Generating icons…");
  buildPng();
  buildIcns();
  await buildIco();
  copyRendererAssets();
  console.log("Done.");
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
