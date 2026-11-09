#!/usr/bin/env node
// Stop hook, adapted from the playbook's before-stop.mjs for Go: before Claude
// finishes a turn in which it changed files, run the fast checks. If one
// fails, Claude is sent back to fix it, or to say plainly that it couldn't.
// It sends Claude back once per stop (stop_hook_active), so it can't loop.
//
// The checks are the same ones CI runs first; keep them to what finishes in
// seconds. `go test` needs the development database (scripts/dev-db.sh).

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

export const CHECKS = [
  ["gofmt", ["-l", "."], (out) => out.trim() === "", "gofmt -l . lists unformatted files"],
  ["go", ["vet", "./..."], null, "go vet ./..."],
  ["go", ["test", "./..."], null, "go test ./..."],
];

function run(command, args, cwd) {
  const r = spawnSync(command, args, { cwd, encoding: "utf8", timeout: 300_000 });
  return { status: r.status ?? 1, output: `${r.stdout ?? ""}${r.stderr ?? ""}` };
}

let input = {};
try {
  input = JSON.parse(readFileSync(0, "utf8") || "{}");
} catch {
  process.exit(0);
}
if (input.stop_hook_active === true) process.exit(0);
const projectDir = process.env.CLAUDE_PROJECT_DIR || input.cwd || process.cwd();
if (!existsSync(path.join(projectDir, ".git")) || !existsSync(path.join(projectDir, "go.mod"))) process.exit(0);
const status = run("git", ["status", "--porcelain"], projectDir);
if (status.status !== 0 || status.output.trim() === "") process.exit(0);

for (const [command, args, ok, label] of CHECKS) {
  const r = run(command, args, projectDir);
  const passed = ok ? r.status === 0 && ok(r.output) : r.status === 0;
  if (!passed) {
    process.stdout.write(
      JSON.stringify({
        decision: "block",
        reason:
          `\`${label}\` fails after your changes. Fix it before finishing. If it can't be fixed in this session, ` +
          `stop and tell the user exactly what fails and why, rather than reporting the work as done.\n\n${r.output.slice(-4000)}`,
      }),
    );
    break;
  }
}
process.exit(0);
