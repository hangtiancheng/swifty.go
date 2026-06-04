// @ts-check
"use strict";

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

/**
 * @typedef {object} CliArgs
 * @property {string} root - Root directory to scan.
 * @property {boolean} dryRun - Show files without removing them.
 */

/**
 * Parse CLI arguments.
 * @param {string[]} argv - Full argv array (e.g. process.argv).
 * @returns {CliArgs}
 */
function parseArgs(argv) {
  /** @type {CliArgs} */
  const result = { root: ".", dryRun: false };
  const args = argv.slice(2);

  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--dry-run") {
      result.dryRun = true;
    } else if (!args[i].startsWith("--")) {
      result.root = args[i];
    }
  }
  return result;
}

/**
 * Recursively collect all .DS_Store file paths under root, sorted.
 * @param {string} root - Absolute directory path.
 * @returns {string[]}
 */
function iterDsStoreFiles(root) {
  /** @type {string[]} */
  const results = [];

  /**
   * @param {string} dir
   */
  function walk(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(fullPath);
      } else if (entry.name === ".DS_Store") {
        results.push(fullPath);
      }
    }
  }
  walk(root);
  return results.sort();
}

/**
 * Remove .DS_Store files (or simulate removal in dry-run mode).
 * @param {string[]} filePaths
 * @param {boolean} dryRun
 * @returns {[number, number]} [removed, failed]
 */
function removeDsStoreFiles(filePaths, dryRun) {
  let removed = 0;
  let failed = 0;

  for (const filePath of filePaths) {
    try {
      if (dryRun) {
        console.log(`Would remove: ${filePath}`);
      } else {
        fs.unlinkSync(filePath);
        console.log(`Removed: ${filePath}`);
      }
      removed++;
    } catch (/** @type {any} */ exc) {
      failed++;
      console.log(`Failed to remove ${filePath}: ${exc}`);
    }
  }
  return [removed, failed];
}

/**
 * Main entry point.
 * @param {string[]} argv
 * @returns {number} Exit code.
 */
function main(argv) {
  const { root: rootArg, dryRun } = parseArgs(argv);
  const root = path.resolve(rootArg);

  if (!fs.existsSync(root)) {
    console.log(`Target directory does not exist: ${root}`);
    return 1;
  }

  const stat = fs.statSync(root);
  if (!stat.isDirectory()) {
    console.log(`Target path is not a directory: ${root}`);
    return 1;
  }

  const dsStoreFiles = iterDsStoreFiles(root);
  if (dsStoreFiles.length === 0) {
    console.log(`No .DS_Store files found under: ${root}`);
    return 0;
  }

  const [removed, failed] = removeDsStoreFiles(dsStoreFiles, dryRun);
  const action = dryRun ? "matched" : "removed";
  console.log(`Done: ${action}=${removed}, failed=${failed}, root=${root}`);
  return failed > 0 ? 1 : 0;
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === path.resolve(process.argv[1])
) {
  process.exit(main(process.argv));
}

export { parseArgs, iterDsStoreFiles, removeDsStoreFiles, main };
