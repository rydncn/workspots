// Builds the binary, creates a fresh database with the migrations applied and
// seeded with known spots (seed.sql), and starts the server. Set BASE_URL to
// an already-running server (the release rehearsal) to skip all of this.
const { spawn, execFileSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const stateFile = path.join(__dirname, ".server.json");
const adminURL = process.env.TEST_DATABASE_URL || "postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_test";
const appURL = process.env.TEST_APP_DATABASE_URL || "postgres://workspots_app:dev-app@localhost:5432/workspots_test";

function psql(url, args) {
  return execFileSync("psql", ["-v", "ON_ERROR_STOP=1", "-qAt", url, ...args], { encoding: "utf8" });
}

module.exports = async () => {
  if (process.env.BASE_URL) return;
  const name = `e2e_${Date.now().toString(36)}`;
  const withDb = (url) => url.replace(/\/[^/?]+(\?|$)/, `/${name}$1`);

  execFileSync("go", ["build", "-o", path.join(__dirname, ".workspots"), "./cmd/workspots"], { cwd: root, stdio: "inherit" });
  psql(adminURL, ["-c", `CREATE DATABASE ${name}`]);
  execFileSync(path.join(__dirname, ".workspots"), ["migrate"], {
    env: { ...process.env, MIGRATE_DATABASE_URL: withDb(adminURL) }, stdio: "inherit",
  });
  psql(withDb(adminURL), ["-f", path.join(__dirname, "seed.sql")]);

  const port = 8090;
  const server = spawn(path.join(__dirname, ".workspots"), [], {
    env: { ...process.env, DATABASE_URL: withDb(appURL), ADDR: `127.0.0.1:${port}`, BASE_URL: `http://127.0.0.1:${port}` },
    stdio: ["ignore", fs.openSync(path.join(__dirname, ".server.log"), "w"), "inherit"],
    detached: true,
  });
  server.unref();
  fs.writeFileSync(stateFile, JSON.stringify({ pid: server.pid, db: name }));

  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/`);
      if (res.ok) return;
    } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error("server did not start; see e2e/.server.log");
};
