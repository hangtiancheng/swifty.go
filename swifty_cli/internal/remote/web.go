package remote

import _ "embed"

// indexHTML is the production build of the React frontend in ./fe.
// It is produced by `pnpm --dir fe build` (rsbuild with inlineScripts +
// inlineStyles), yielding a single self-contained index.html that we serve
// at "/" and that talks to the backend over "/ws".
//
//go:embed fe/dist/index.html
var indexHTML string
