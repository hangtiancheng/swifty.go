// @ts-check
"use strict";

/**
 * Tag and publish Go modules in a monorepo.
 *
 * Library usage:
 *     import { main } from './tag.js';
 *     main({ cache: 'v0.1.0', rpc: 'v0.2.0' });
 *
 * CLI usage:
 *     node tag.js --cache=v0.1.0 --rpc=v0.2.0
 */

import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

/** @type {Record<string, string>} */
const MODULES = {
  cache: "lark_cache",
  http: "lark_http",
  orm: "lark_orm",
  rpc: "lark_rpc",
};

/**
 * Run a command synchronously, throwing on non-zero exit.
 * @param {string[]} command
 */
function run(command) {
  console.log(`> ${command.join(" ")}`);
  const result = spawnSync(command[0], command.slice(1), { stdio: "inherit" });
  if (result.status !== 0) {
    throw new Error(
      `Command failed: ${command.join(" ")} (exit ${result.status ?? "?"})`,
    );
  }
}

/**
 * Validate that a version string matches semver (vX.Y.Z).
 * @param {string} ver
 * @returns {boolean}
 */
function isValidVersion(ver) {
  return /^v\d+\.\d+\.\d+/.test(ver);
}

/**
 * @typedef {object} Versions
 * @property {string} [cache] - lark_cache version tag (e.g. "v0.1.0").
 * @property {string} [http]  - lark_http version tag.
 * @property {string} [orm]   - lark_orm version tag.
 * @property {string} [rpc]   - lark_rpc version tag.
 */

/**
 * Tag and push specified module versions.
 * @param {Versions} versions
 */
function main(versions) {
  /** @type {string[]} */
  const tagged = [];

  for (const [key, mod] of Object.entries(MODULES)) {
    const ver = versions[/** @type {keyof Versions} */ (key)];
    if (!ver) continue;
    if (!isValidVersion(ver)) {
      throw new Error(`invalid version for ${key}: "${ver}" (expected vX.Y.Z)`);
    }
    const tag = `${mod}/${ver}`;
    run(["git", "tag", tag]);
    run(["git", "push", "origin", tag]);
    tagged.push(tag);
  }

  if (tagged.length === 0) {
    console.log("No modules specified.");
  } else {
    console.log(`\nTagged and pushed: ${tagged.join(", ")}`);
  }
}

/**
 * Parse CLI arguments into a Versions object.
 * @param {string[]} argv
 * @returns {Versions}
 */
function parseArgs(argv) {
  const args = argv.slice(2);
  if (args.length === 0) {
    const keys = Object.keys(MODULES)
      .map((k) => `--${k}=vX.Y.Z`)
      .join(" ");
    console.log(`Usage: node tag.js ${keys}`);
    process.exit(1);
  }

  /** @type {Versions} */
  const versions = {};
  for (const arg of args) {
    const m = arg.match(/^--(\w+)=(.+)$/);
    if (!m) {
      console.error(`invalid argument: ${arg}`);
      process.exit(1);
    }
    const [, key, ver] = m;
    if (!(key in MODULES)) {
      console.error(
        `unknown module: ${key} (expected: ${Object.keys(MODULES).join(", ")})`,
      );
      process.exit(1);
    }
    versions[/** @type {keyof Versions} */ (key)] = ver;
  }
  return versions;
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === path.resolve(process.argv[1])
) {
  main(parseArgs(process.argv));
}

export { main, parseArgs, isValidVersion, MODULES };
