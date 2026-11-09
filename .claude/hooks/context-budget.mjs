#!/usr/bin/env node
// UserPromptSubmit hook: warns when the session's context passes a budget,
// 100,000 tokens unless CLAUDE_CONTEXT_BUDGET says otherwise (rule 17).
//
// Past that size an agent follows its instructions less closely and decides
// worse, long before the window is full, and compaction carries forward a
// lossy summary rather than the session itself. So the hook tells Claude to
// suggest a handoff and a fresh session at the next natural break, and shows
// the user the same. It warns once on passing the budget and again every
// 50,000 tokens after; a compacted or cleared session starts again.
//
// The size is the last main-thread reply's usage in the transcript: what the
// model was sent, plus what it wrote. Never fails or blocks the prompt.

import {
  closeSync,
  openSync,
  readFileSync,
  readSync,
  fstatSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const DEFAULT_BUDGET = 100_000;
const STEP = 50_000;
const TAIL_BYTES = 4 * 1024 * 1024;

/** The context size from a transcript's JSONL text, or null if it has none. */
export function contextTokens(transcript) {
  const lines = transcript.split("\n");
  for (let i = lines.length - 1; i >= 0; i--) {
    let row;
    try {
      row = JSON.parse(lines[i]);
    } catch {
      continue;
    }
    const usage = row?.message?.usage;
    if (row?.type !== "assistant" || row.isSidechain || !usage) continue;
    return (
      (usage.input_tokens ?? 0) +
      (usage.cache_read_input_tokens ?? 0) +
      (usage.cache_creation_input_tokens ?? 0) +
      (usage.output_tokens ?? 0)
    );
  }
  return null;
}

const thousands = (n) => n.toLocaleString("en-GB");
const short = (n) => `${Math.round(n / 1000)}k`;

/** Whether to warn now, and the step to remember having warned at. */
export function checkBudget({ tokens, budget, warnedAt }) {
  if (tokens < budget) return { message: null, warnedAt: null };
  const step = budget + Math.floor((tokens - budget) / STEP) * STEP;
  if (warnedAt !== null && step <= warnedAt) return { message: null, warnedAt };
  return {
    message:
      `Context check: this session is using about ${thousands(Math.round(tokens / 1000) * 1000)} tokens of context, ` +
      `past the project's budget of ${thousands(budget)} (rule 17). Past that, agents follow instructions less closely. ` +
      "Don't stop in the middle of a step, but at the next natural break tell the user, and suggest writing a handoff " +
      "(/handoff, or update docs/handoff.md) and continuing in a fresh session, rather than carrying on or relying on compaction.",
    warnedAt: step,
  };
}

function tail(file) {
  const fd = openSync(file, "r");
  try {
    const size = fstatSync(fd).size;
    const length = Math.min(size, TAIL_BYTES);
    const buffer = Buffer.alloc(length);
    readSync(fd, buffer, 0, length, size - length);
    return buffer.toString("utf8");
  } finally {
    closeSync(fd);
  }
}

function main() {
  try {
    const input = JSON.parse(readFileSync(0, "utf8"));
    const tokens = contextTokens(tail(input.transcript_path));
    if (tokens === null) return;

    const configured = Number.parseInt(
      process.env.CLAUDE_CONTEXT_BUDGET ?? "",
      10,
    );
    const budget = configured > 0 ? configured : DEFAULT_BUDGET;
    const id = String(input.session_id ?? "session").replace(/[^\w-]/g, "_");
    const stateFile = path.join(
      process.env.CONTEXT_BUDGET_STATE_DIR || tmpdir(),
      `claude-context-budget-${id}`,
    );
    let warnedAt = null;
    try {
      warnedAt = Number.parseInt(readFileSync(stateFile, "utf8"), 10) || null;
    } catch {
      // Not warned yet in this session.
    }

    const result = checkBudget({ tokens, budget, warnedAt });
    writeFileSync(stateFile, String(result.warnedAt ?? ""));
    if (!result.message) return;
    process.stdout.write(
      JSON.stringify({
        systemMessage: `Context is about ${short(tokens)} tokens, past the ${short(budget)} budget. Consider a handoff and a fresh session at the next break.`,
        hookSpecificOutput: {
          hookEventName: "UserPromptSubmit",
          additionalContext: result.message,
        },
      }),
    );
  } catch {
    // A missing or unreadable transcript just means no warning.
  }
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === path.resolve(process.argv[1])
) {
  main();
  process.exit(0);
}
