---
name: update-go-skills
description: >
  Regenerate the swifty.go framework skill documentation by reading the latest package source code and rewriting SKILL.md files for swifty_cache (swifty_cache → .github/skills/swifty-cache), swifty_http (swifty_http → .github/skills/swifty-http), swifty_orm (swifty_orm → .github/skills/swifty-orm), and swifty_rpc (swifty_rpc → .github/skills/swifty-rpc).
  Use this skill when the user asks to update, refresh, regenerate, or sync the skill docs with the current source, or when source code under swifty_cache or swifty_http or swifty_orm or swifty_rpc has changed and the skills need to reflect the new API surface. Also trigger when the user mentions "update skills", "refresh skill docs", "sync skills with source", or says the skill documentation is outdated.
  Do NOT use for creating entirely new skills unrelated to these four packages, or for editing the skill description/triggering metadata only.
---

# Update Swifty-Go Framework Skills

This skill orchestrates the regeneration of four framework skill documents by
reading the latest source code and producing comprehensive, professional English
documentation for each package.

## Target mapping

| Source directory | Skill file to update                   |
| ---------------- | -------------------------------------- |
| `swifty_cache/`  | `.github/skills/swifty-cache/SKILL.md` |
| `swifty_http/`   | `.github/skills/swifty-http/SKILL.md`  |
| `swifty_orm/`    | `.github/skills/swifty-orm/SKILL.md`   |
| `swifty_rpc/`    | `.github/skills/swifty-rpc/SKILL.md`   |

All paths are relative to the repository root.

## Procedure

For each of the four packages, execute the following steps in order:

1. Read every `.go` file in the source directory (and subdirectories where they
   exist, such as `swifty_rpc/pkg/`, `swifty_rpc/internal/`, `swifty_cache/pb/`).
   Do not skip test files -- they reveal usage patterns, edge cases, and
   internal helpers that downstream users may need to know about.

2. Read the `go.mod` file in the source directory to capture the module path,
   Go toolchain version, and dependency list.

3. Analyze the source to extract:
   - Package architecture and component relationships
   - All exported types, interfaces, functions, methods, and constants
   - Constructor patterns and option types
   - Method signatures with parameter and return types
   - Behavioral contracts, invariants, and non-obvious semantics
   - Concurrency safety characteristics
   - Error handling patterns and sentinel errors
   - Known limitations, caveats, and pitfalls
   - Internal implementation details that affect correctness (e.g., lock
     contention points, goroutine lifecycle, protocol details)
   - File-to-purpose mapping

4. Write the updated SKILL.md following the format specification below.

## Output format specification

Each SKILL.md must contain:

### YAML front matter

```yaml
---
name: <skill-name>
description: >
  <multi-line description optimized for skill triggering; include the module
  path, key type names, key function names, and explicit trigger/skip guidance>
---
```

The `description` field is the primary mechanism that determines whether Claude
invokes the skill. It must:

- Name the module import path
- List the most important exported symbols (types, functions, methods)
- Describe what the package does in one sentence
- Include explicit "Use when..." guidance with concrete trigger tokens
- Include explicit "Do NOT use for..." exclusions

### Body structure

The body must cover all of the following sections (adapt headings to fit the
package):

1. One-paragraph summary: what the package is, its design philosophy, and its
   module path.

2. Architecture overview: ASCII diagram showing the component graph and data
   flow. Name every major type and show ownership/composition relationships.

3. Core types: for each exported type, document:
   - Constructor(s) and option types
   - Full method table with signatures and behavioral notes
   - Field-level semantics where fields are public
   - Sentinel errors

4. Internal implementation details that affect correct usage (lock scope,
   goroutine lifecycle, protocol wire format, codec constraints, etc.).

5. Typical usage: realistic code examples covering the most common patterns.
   Examples must compile against the current API surface.

6. Pitfalls / known limitations: numbered list of non-obvious behaviors that
   bite users. Each item should state the behavior, why it happens, and how to
   avoid or work around it.

7. File map: table mapping each source file to its purpose.

8. Dependencies: external module dependencies.

9. Cross-references to sibling skills where integration points exist.

## Writing standards

- Use professional, precise English throughout.
- Prefer active voice and imperative mood for instructions.
- Document what the code actually does, not what it should do. If there is a
  gap between intent and implementation (e.g., an unenforced MaxBytes field),
  document the actual behavior and note the discrepancy.
- Include every exported symbol. Completeness is non-negotiable; a skill that
  omits an exported function forces the user to read source anyway.
- For behavioral contracts, state the invariant, then state what breaks if the
  contract is violated.
- Keep code examples minimal but compilable. Use `// ...` to elide irrelevant
  setup, never to hide API surface.
- Do not use bold text or emoji in the output.
- Do not invent features or capabilities that do not exist in the source code.

## Execution order

Process the four packages in any order. Each is independent. If one package
fails to update (e.g., source directory missing), report the failure and
continue with the remaining packages.

After updating all four, report what changed at a high level (new types added,
removed APIs, significant behavioral changes) so the user can verify the update
is correct.
