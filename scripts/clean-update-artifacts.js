const fs = require("fs");
const path = require("path");

const releaseDir = path.join(__dirname, "..", "release");
if (!fs.existsSync(releaseDir)) process.exit(0);

const pattern = /\.blockmap$|^latest(-mac|-linux)?\.yml$/;

for (const entry of fs.readdirSync(releaseDir)) {
  if (pattern.test(entry)) {
    fs.unlinkSync(path.join(releaseDir, entry));
    console.log("removed", entry);
  }
}
