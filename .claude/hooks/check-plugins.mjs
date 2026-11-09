#!/usr/bin/env node
// SessionStart hook: tells Claude, at the start of every session, which of
// the plugins the project enables in .claude/settings.json aren't installed.
//
// Enabling a plugin in the project's settings doesn't install it everywhere.
// A cloud session installs nothing from project settings, and the session
// that writes the settings never has the plugins it enabled. Without this
// check the playbook's flow quietly falls back to doing everything by hand,
// and nobody notices. It can't install them for the running session (skills
// load before hooks run), so it says what's missing and how to fix it.
//
// Prints nothing when everything is installed. Never fails the session.

import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

function defaultExec(command, args, cwd) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    timeout: 30_000,
    shell: process.platform === "win32",
  });
  return { status: result.status ?? 1, output: result.stdout ?? "" };
}

export function enabledPlugins(projectDir) {
  try {
    const settings = JSON.parse(
      readFileSync(path.join(projectDir, ".claude", "settings.json"), "utf8"),
    );
    return Object.entries(settings.enabledPlugins ?? {})
      .filter(([, on]) => on === true)
      .map(([id]) => id)
      .sort();
  } catch {
    return [];
  }
}

/** Returns the message for Claude, or null when there's nothing to say. */
export function checkPlugins({ projectDir, exec = defaultExec }) {
  const enabled = enabledPlugins(projectDir);
  if (enabled.length === 0) return null;

  const list = exec("claude", ["plugin", "list", "--json"], projectDir);
  let installed;
  try {
    if (list.status !== 0) throw new Error("claude plugin list failed");
    installed = new Set(JSON.parse(list.output).map((p) => p.id));
  } catch {
    return (
      "Couldn't check which plugins are installed (`claude plugin list --json` failed). " +
      `The project enables: ${enabled.join(", ")}. Check with \`claude plugin list\` before starting work.`
    );
  }

  const missing = enabled.filter((id) => !installed.has(id));
  if (missing.length === 0) return null;
  return (
    `These plugins are enabled in .claude/settings.json but not installed in this session: ${missing.join(", ")}. ` +
    "Tell the user before starting work, and don't quietly do their steps by hand instead. " +
    "Installing them now won't help this session, because skills load at start. " +
    "In a cloud environment, install them in the environment's setup script; locally, accept the plugin prompt or run " +
    "`claude plugin install <plugin>`. Then start a new session."
  );
}

function main() {
  try {
    readFileSync(0, "utf8");
  } catch {
    // No input is fine; this hook only needs the project directory.
  }
  const projectDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();
  const message = checkPlugins({ projectDir });
  if (message) process.stdout.write(message);
  process.exit(0);
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === path.resolve(process.argv[1])
)
  main();
