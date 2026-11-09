// Stops the server global-setup.js started and drops its database.
const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const stateFile = path.join(__dirname, ".server.json");
const adminURL = process.env.TEST_DATABASE_URL || "postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_test";

module.exports = async () => {
  if (!fs.existsSync(stateFile)) return;
  const { pid, db } = JSON.parse(fs.readFileSync(stateFile, "utf8"));
  try { process.kill(pid, "SIGTERM"); } catch {}
  await new Promise((r) => setTimeout(r, 300));
  execFileSync("psql", ["-v", "ON_ERROR_STOP=1", "-q", adminURL, "-c", `DROP DATABASE IF EXISTS ${db} WITH (FORCE)`]);
  fs.unlinkSync(stateFile);
};
