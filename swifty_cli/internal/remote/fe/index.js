// First: cd resolve(__filepath, ../../../../../swifty-cli)
// Second: run `pnpm install`, then `pnpm fe:build`
// Third: cp dist (/path/to/swifty-cli/apps/swifty/src/remote/fe/dist) to dirname(__filepath) (/path/to/swifty.go/swifty_cli/internal/remote/fe)

import path from 'node:path';
import fs from 'node:fs';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

/** @type {string} Directory containing this script. */
const __dirname = path.dirname(fileURLToPath(import.meta.url));

/** @type {string} Root of the swifty-cli repository (sibling of swifty.go). */
const swiftyCliRoot = path.resolve(__dirname, '../../../../..', 'swifty-cli');

/** @type {string} Package dir that owns the `fe:build` script. */
const swiftyAppDir = path.join(swiftyCliRoot, 'apps', 'swifty');

/** @type {string} Build output produced by `pnpm fe:build`. */
const distDir = path.join(swiftyAppDir, 'src', 'remote', 'fe', 'dist');

/** @type {string} Destination dist dir next to this script. */
const targetDir = path.join(__dirname, 'dist');

/**
 * Run a command synchronously, inheriting stdio; exit on failure.
 * @param {string} command - Executable to run.
 * @param {string[]} args - Command arguments.
 * @param {string} cwd - Working directory for the command.
 * @returns {void}
 */
function run(command, args, cwd) {
  console.log(`\n$ ${command} ${args.join(' ')}  (cwd: ${cwd})`);
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', shell: process.platform === 'win32' });
  if (result.error) {
    console.error(`Failed to run ${command}:`, result.error.message);
    process.exit(1);
  }
  if (result.status !== 0) {
    console.error(`${command} ${args.join(' ')} exited with code ${result.status}`);
    process.exit(result.status ?? 1);
  }
}

/**
 * Build the swifty-cli frontend and copy its dist into this directory.
 * @returns {void}
 */
function main() {
  if (!fs.existsSync(swiftyCliRoot)) {
    console.error(`swifty-cli repo not found at ${swiftyCliRoot}`);
    process.exit(1);
  }

  run('pnpm', ['install'], swiftyCliRoot);
  run('pnpm', ['fe:build'], swiftyAppDir);

  if (!fs.existsSync(distDir)) {
    console.error(`Build output not found at ${distDir}`);
    process.exit(1);
  }

  if (fs.existsSync(targetDir)) {
    console.log(`Removing old dist at ${targetDir}`);
    fs.rmSync(targetDir, { recursive: true, force: true });
  }
  fs.cpSync(distDir, targetDir, { recursive: true });
  console.log(`\nCopied ${distDir} -> ${targetDir}`);
}

main();
