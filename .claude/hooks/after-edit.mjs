#!/usr/bin/env node
// PostToolUse hook for Edit, Write and MultiEdit, adapted from the playbook's
// after-edit.mjs for Go: formats a changed .go file with gofmt and vets its
// package, so problems come back while the agent is still on that file.
//
// Exit 0: nothing to report. Exit 2: vet errors, shown to Claude. Files that
// aren't Go are left alone (no Prettier here: templates and SQL are hand-kept).

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

function run(command, args, cwd) {
  const r = spawnSync(command, args, { cwd, encoding: "utf8", timeout: 60_000 });
  return { status: r.status ?? 1, output: `${r.stdout ?? ""}${r.stderr ?? ""}` };
}

let input = {};
try {
  input = JSON.parse(readFileSync(0, "utf8") || "{}");
} catch {
  process.exit(0);
}
const projectDir = process.env.CLAUDE_PROJECT_DIR || input.cwd || process.cwd();
const filePath = input?.tool_input?.file_path;
if (typeof filePath !== "string" || !filePath.endsWith(".go")) process.exit(0);
const absolute = path.resolve(projectDir, filePath);
if (!absolute.startsWith(projectDir + path.sep) || !existsSync(absolute)) process.exit(0);

run("gofmt", ["-w", absolute], projectDir);
const pkg = "./" + path.relative(projectDir, path.dirname(absolute)).split(path.sep).join("/");
const vet = run("go", ["vet", pkg], projectDir);
if (vet.status !== 0) {
  process.stderr.write(`go vet found problems in ${pkg}. Fix them before moving on:\n${vet.output.slice(-4000)}`);
  process.exit(2);
}
