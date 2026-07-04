// @ts-check
"use strict";

/**
 * Go workspace management CLI.
 *
 * Commands:
 *   deps                List direct external dependencies for all go.work modules
 *   build               Run `go build ./...` for all go.work modules
 *   test                Run `go test ./...` for all go.work modules
 *   coverage            Run tests with coverage for all go.work modules
 *   coverage:standalone Run coverage for non-MongoDB modules only
 *   coverage:mongo      Run coverage for MongoDB-dependent modules only
 *   <module-name>       Run coverage for a specific module
 *   clean               Remove all build, test, and coverage artifacts
 *
 * CLI usage:
 *     node main.js deps
 *     node main.js build
 *     node main.js test
 *     node main.js coverage --timeout=120s
 *     node main.js swifty_http
 */

import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

/** @type {string} */
const ROOT_DIR = path.dirname(fileURLToPath(import.meta.url));

/** @type {string[]} */
const STANDALONE_MODULES = ["swifty_cache", "swifty_http", "swifty_rpc"];

/** @type {string[]} */
const MONGO_MODULES = ["swifty_orm", "swifty_ai"];

/** @type {string[]} */
const ALL_MODULES = [...STANDALONE_MODULES, ...MONGO_MODULES];

/** @type {string} */
const INTERNAL_MODULE_PREFIX = "github.com/hangtiancheng/";

/** @type {string} */
const DEFAULT_MONGO_URI = "mongodb://localhost:27017";

/**
 * Read an environment variable with a fallback.
 * @param {string} name
 * @param {string} fallback
 * @returns {string}
 */
function envValue(name, fallback) {
  return process.env[name] ?? fallback;
}

/**
 * Run a command synchronously, throwing on non-zero exit.
 * @param {string[]} command
 * @param {string} [cwd]
 * @param {NodeJS.ProcessEnv} [env]
 */
function run(command, cwd = ROOT_DIR, env = process.env) {
  const result = spawnSync(command[0], command.slice(1), {
    cwd,
    env,
    stdio: "inherit",
  });
  if (result.status !== 0) {
    const err = /** @type {Error & { exitCode: number }} */ (
      new Error(`Command failed: ${command.join(" ")} (exit ${result.status ?? "?"})`)
    );
    err.exitCode = result.status ?? 1;
    throw err;
  }
}

/**
 * @typedef {object} CommonArgs
 * @property {string} go           - Go binary path.
 * @property {string} gowork       - GOWORK env value ("off" disables workspace mode).
 * @property {string} coverprofile - Coverage output file name.
 * @property {string} timeout      - Go test timeout string (e.g. "90s").
 * @property {string} mongoUri     - MongoDB connection URI.
 */

/**
 * Build the environment dict for module-level commands.
 * @param {CommonArgs} args
 * @returns {NodeJS.ProcessEnv}
 */
function moduleEnv(args) {
  const env = { ...process.env };
  env.GOWORK = args.gowork;
  env.MONGO_URI = args.mongoUri;
  return env;
}

/**
 * Resolve the directory for a module, throwing if it does not exist.
 * @param {string} mod
 * @returns {string}
 */
function moduleDir(mod) {
  const dir = path.join(ROOT_DIR, mod);
  if (!fs.existsSync(dir) || !fs.statSync(dir).isDirectory()) {
    throw new Error(`Module directory does not exist: ${dir}`);
  }
  return dir;
}

// ---- build ----------------------------------------------------------------

/**
 * Run `go build ./...` for a single module.
 * @param {string} mod
 * @param {CommonArgs} args
 */
function buildModule(mod, args) {
  console.log(`==> Building ${mod}`);
  run([args.go, "build", "./..."], moduleDir(mod), moduleEnv(args));
}

/**
 * Run `go build ./...` for a list of modules.
 * @param {string[]} modules
 * @param {CommonArgs} args
 */
function buildModules(modules, args) {
  for (const mod of modules) {
    buildModule(mod, args);
  }
  console.log(`\nBuild passed for ${modules.length} module(s).`);
}

// ---- test -----------------------------------------------------------------

/**
 * Run `go test ./...` for a single module (no coverage).
 * @param {string} mod
 * @param {CommonArgs} args
 */
function testModule(mod, args) {
  console.log(`==> Testing ${mod}`);
  run(
    [args.go, "test", "./...", "-count=1", `-timeout=${args.timeout}`],
    moduleDir(mod),
    moduleEnv(args),
  );
}

/**
 * Run `go test ./...` for a list of modules (no coverage).
 * @param {string[]} modules
 * @param {CommonArgs} args
 */
function testModules(modules, args) {
  for (const mod of modules) {
    testModule(mod, args);
  }
  console.log(`\nTests passed for ${modules.length} module(s).`);
}

// ---- coverage -------------------------------------------------------------

/**
 * Run tests with coverage for a single module.
 * @param {string} mod
 * @param {CommonArgs} args
 */
function coverageModule(mod, args) {
  const dir = moduleDir(mod);
  console.log(`==> Coverage ${mod}`);
  run(
    [
      args.go,
      "test",
      "./...",
      "-count=1",
      `-timeout=${args.timeout}`,
      `-coverprofile=${args.coverprofile}`,
    ],
    dir,
    moduleEnv(args),
  );
  run([args.go, "tool", "cover", `-func=${args.coverprofile}`], dir);
}

/**
 * Run tests with coverage for a list of modules.
 * @param {string[]} modules
 * @param {CommonArgs} args
 */
function coverageModules(modules, args) {
  for (const mod of modules) {
    coverageModule(mod, args);
  }
}

/** @type {string[]} Directories created by air or go build that should be removed. */
const CLEAN_DIRS = ["tmp"];

/** @type {string[]} File extensions considered build/test artifacts. */
const CLEAN_EXTS = [".test", ".exe", ".out", ".dll", ".so", ".dylib"];

/**
 * Remove a file and log the path relative to the project root.
 * @param {string} filePath
 */
function removeFile(filePath) {
  fs.unlinkSync(filePath);
  console.log(`  rm ${path.relative(ROOT_DIR, filePath)}`);
}

/**
 * Recursively remove a directory and log the path relative to the project root.
 * @param {string} dirPath
 */
function removeDir(dirPath) {
  fs.rmSync(dirPath, { recursive: true, force: true });
  console.log(`  rm -rf ${path.relative(ROOT_DIR, dirPath)}`);
}

/**
 * Remove all build, test, and coverage artifacts from every module directory.
 * @param {CommonArgs} args
 */
function clean(args) {
  for (const mod of ALL_MODULES) {
    const modDir = path.join(ROOT_DIR, mod);
    if (!fs.existsSync(modDir)) continue;

    console.log(`==> Cleaning ${mod}`);

    const coveragePath = path.join(modDir, args.coverprofile);
    if (fs.existsSync(coveragePath)) {
      removeFile(coveragePath);
    }

    for (const dir of CLEAN_DIRS) {
      const dirPath = path.join(modDir, dir);
      if (fs.existsSync(dirPath) && fs.statSync(dirPath).isDirectory()) {
        removeDir(dirPath);
      }
    }

    const entries = fs.readdirSync(modDir, { withFileTypes: true });
    for (const entry of entries) {
      if (entry.isDirectory()) continue;
      const ext = path.extname(entry.name).toLowerCase();
      if (CLEAN_EXTS.includes(ext)) {
        removeFile(path.join(modDir, entry.name));
      }
    }
  }

  console.log("\nClean complete.");
}

// ---- deps -----------------------------------------------------------------

/**
 * Read the module list from go.work.
 * @param {string} [workFile]
 * @returns {string[]}
 */
function workspaceModules(workFile = path.join(ROOT_DIR, "go.work")) {
  /** @type {string[]} */
  const modules = [];
  let inUseBlock = false;
  const lines = fs.readFileSync(workFile, "utf-8").split(/\r?\n/);

  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (line.startsWith("use (")) {
      inUseBlock = true;
      continue;
    }
    if (inUseBlock && line === ")") {
      inUseBlock = false;
      continue;
    }
    if (inUseBlock && line.startsWith("./")) {
      modules.push(line.slice(2));
      continue;
    }
    if (line.startsWith("use ./")) {
      modules.push(line.slice(6));
    }
  }
  return modules;
}

/**
 * Parse a single `require` line and return the dependency path if it is direct.
 * @param {string} requireLine
 * @returns {string|null}
 */
function parseDependencyPath(requireLine) {
  if (requireLine.includes("// indirect")) return null;
  const match = requireLine.match(/^([^\s]+)\s+v[^\s]+/);
  if (match === null) return null;
  return match[1];
}

/**
 * Find direct, external (non-internal) dependencies of a Go module.
 * @param {string} mod
 * @returns {string[]}
 */
function directExternalDeps(mod) {
  const goMod = path.join(ROOT_DIR, mod, "go.mod");
  if (!fs.existsSync(goMod)) return [];

  /** @type {string[]} */
  const deps = [];
  let inRequireBlock = false;
  const lines = fs.readFileSync(goMod, "utf-8").split(/\r?\n/);

  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line || line.startsWith("//")) continue;
    if (line === "require (") {
      inRequireBlock = true;
      continue;
    }
    if (inRequireBlock && line === ")") {
      inRequireBlock = false;
      continue;
    }

    /** @type {string|null} */
    let dependencyPath = null;
    if (inRequireBlock) {
      dependencyPath = parseDependencyPath(line);
    } else if (line.startsWith("require ")) {
      dependencyPath = parseDependencyPath(line.slice(8).trim());
    }
    if (dependencyPath === null) continue;
    if (dependencyPath.startsWith(INTERNAL_MODULE_PREFIX)) continue;
    deps.push(dependencyPath);
  }
  return [...new Set(deps)].sort();
}

/**
 * List direct external dependencies for all workspace modules.
 */
function listDeps() {
  /** @type {Set<string>} */
  const allDeps = new Set();
  for (const mod of workspaceModules()) {
    const deps = directExternalDeps(mod);
    console.log(`${mod}:`);
    if (deps.length === 0) {
      console.log("  (none)");
      continue;
    }
    for (const dep of deps) {
      allDeps.add(dep);
      console.log(`  ${dep}`);
    }
  }

  console.log("");
  console.log("All direct external dependencies:");
  if (allDeps.size === 0) {
    console.log("  (none)");
    return;
  }
  for (const dep of [...allDeps].sort()) {
    console.log(`  ${dep}`);
  }
}

// ---- CLI ------------------------------------------------------------------

const USAGE = `Usage: node main.js <command> [options]

Commands:
  deps                List direct external dependencies
  build               Run go build for all modules
  test                Run go test for all modules
  coverage            Run tests with coverage for all modules
  coverage:standalone Run coverage for non-MongoDB modules only
  coverage:mongo      Run coverage for MongoDB-dependent modules only
  <module-name>       Run coverage for a specific module (${ALL_MODULES.join(", ")})
  clean               Remove all build, test, and coverage artifacts

Options:
  --go <path>           Go binary (default: go)
  --gowork <value>      GOWORK env (default: off)
  --timeout <duration>  Test timeout (default: 90s)
  --mongo-uri <uri>     MongoDB URI (default: ${DEFAULT_MONGO_URI})
  --coverprofile <file> Coverage output file (default: coverage.out)`;

/**
 * Parse common CLI options.
 * @param {string[]} argv
 * @returns {CommonArgs}
 */
function parseCommonOptions(argv) {
  const args = argv.slice(2);
  /** @type {CommonArgs} */
  const result = {
    go: envValue("GO", "go"),
    gowork: envValue("GOWORK", "off"),
    coverprofile: envValue("COVERPROFILE", "coverage.out"),
    timeout: envValue("GO_TEST_TIMEOUT", "90s"),
    mongoUri: envValue("MONGO_URI", DEFAULT_MONGO_URI),
  };

  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--go") result.go = args[++i] ?? result.go;
    else if (args[i] === "--gowork") result.gowork = args[++i] ?? result.gowork;
    else if (args[i] === "--coverprofile") result.coverprofile = args[++i] ?? result.coverprofile;
    else if (args[i] === "--timeout") result.timeout = args[++i] ?? result.timeout;
    else if (args[i] === "--mongo-uri") result.mongoUri = args[++i] ?? result.mongoUri;
  }
  return result;
}

/**
 * Dispatch a subcommand.
 * @param {string[]} argv
 * @returns {number} Exit code.
 */
function main(argv) {
  const rawArgs = argv.slice(2);
  if (rawArgs.length === 0) {
    console.log(USAGE);
    return 1;
  }

  const command = rawArgs[0];
  const commonArgs = parseCommonOptions(argv);

  try {
    if (ALL_MODULES.includes(command)) {
      coverageModule(command, commonArgs);
    } else if (command === "deps") {
      listDeps();
    } else if (command === "build") {
      buildModules(ALL_MODULES, commonArgs);
    } else if (command === "test") {
      testModules(ALL_MODULES, commonArgs);
    } else if (command === "coverage") {
      coverageModules(ALL_MODULES, commonArgs);
    } else if (command === "coverage:standalone") {
      coverageModules(STANDALONE_MODULES, commonArgs);
    } else if (command === "coverage:mongo") {
      coverageModules(MONGO_MODULES, commonArgs);
    } else if (command === "clean") {
      clean(commonArgs);
    } else {
      console.error(`Unknown command: ${command}\n`);
      console.log(USAGE);
      return 1;
    }
  } catch (/** @type {any} */ err) {
    if (err.exitCode !== undefined) return err.exitCode;
    console.error(String(err));
    return 1;
  }
  return 0;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  process.exit(main(process.argv));
}

export {
  envValue,
  run,
  moduleEnv,
  buildModule,
  buildModules,
  testModule,
  testModules,
  coverageModule,
  coverageModules,
  clean,
  workspaceModules,
  parseDependencyPath,
  directExternalDeps,
  listDeps,
  parseCommonOptions,
  main,
  STANDALONE_MODULES,
  MONGO_MODULES,
  ALL_MODULES,
};
