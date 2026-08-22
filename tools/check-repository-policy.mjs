import { existsSync, readFileSync } from "node:fs";

const required = [
  "AGENTS.md",
  "SECURITY.md",
  "CONTRIBUTING.md",
  ".github/CODEOWNERS",
  ".github/PULL_REQUEST_TEMPLATE.md",
  ".github/workflows/quality.yml",
  "docs/adr/README.md",
  "docs/phase-gates/phase-0.md"
];

const missing = required.filter((path) => !existsSync(path));
if (missing.length > 0) {
  console.error(`Missing repository policy files:\n${missing.join("\n")}`);
  process.exit(1);
}

const codeowners = readFileSync(".github/CODEOWNERS", "utf8");
if (!codeowners.includes("@sinanganiz")) {
  console.error("CODEOWNERS must include @sinanganiz.");
  process.exit(1);
}

console.log("Repository policy check passed.");
