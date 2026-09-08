import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import process from "node:process";

const generatedDirectories = ["gen", "apps/web-console/src/generated"];
const present = generatedDirectories.filter((path) => existsSync(path));

if (present.length === 0) {
  console.log("No generated artifact directories exist yet.");
  process.exit(0);
}

console.log(`Generated directories present: ${present.join(", ")}`);

// Web Console OpenAPI types (WC-D02): the committed file must match what the
// contract produces today. A drifted file means the console is typed against a
// contract the Control Plane no longer serves.
const committed = "apps/web-console/src/generated/openapi.ts";
if (existsSync(committed)) {
  const scratch = mkdtempSync(join(tmpdir(), "guardian-generated-"));
  const candidate = join(scratch, "openapi.ts");
  try {
    // Run from the workspace so the root `redocly.yaml`, which declares a
    // named API without an openapi-ts output key, is not picked up.
    execFileSync(
      "npx",
      [
        "--yes",
        "openapi-typescript@7.13.0",
        "../../openapi/guardian.yaml",
        "--output",
        candidate,
      ],
      { stdio: "pipe", shell: process.platform === "win32", cwd: "apps/web-console" }
    );
    // Normalise line endings: a Windows checkout gets CRLF for the committed
    // file while the generator always emits LF, which would report every
    // Windows run as stale.
    const normalise = (path) => readFileSync(path, "utf8").replace(/\r\n/g, "\n");
    if (normalise(committed) !== normalise(candidate)) {
      console.error(
        `${committed} is stale. Run "npm run generate:api -w @guardianpot/web-console" and commit the result.`
      );
      process.exit(1);
    }
    // Types only: a runtime declaration here would ship bytes to the browser.
    const runtime = readFileSync(committed, "utf8").match(
      /^export (const|function|class|let|var) /m
    );
    if (runtime) {
      console.error(
        `${committed} declares runtime code (${runtime[0].trim()}); generated types must emit nothing.`
      );
      process.exit(1);
    }
    console.log("Web Console OpenAPI types are current and runtime-free.");
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}

// The runtime constraints the form validators derive from (WCX-11 section 8.2).
// A `maxLength` is not a type, so it cannot live in `openapi.ts`; it is
// generated separately and checked here for the same reason. A stale file means
// a form accepts input the Control Plane rejects, which is precisely the drift
// section 8.2 forbids hand-copying to avoid.
const constraints = "apps/web-console/src/generated/constraints.ts";
if (existsSync(constraints)) {
  const before = readFileSync(constraints, "utf8");
  execFileSync("npm", ["run", "generate:constraints", "-w", "@guardianpot/web-console"], {
    stdio: "pipe",
    shell: process.platform === "win32",
  });
  const after = readFileSync(constraints, "utf8");
  if (before.replace(/\r\n/g, "\n") !== after.replace(/\r\n/g, "\n")) {
    writeFileSync(constraints, before);
    console.error(
      `${constraints} is stale. Run "npm run generate:constraints -w @guardianpot/web-console" and commit the result.`
    );
    process.exit(1);
  }
  console.log("Web Console contract constraints are current.");
}
