import { existsSync, readFileSync } from "node:fs";

const required = [
  "AGENTS.md",
  "SECURITY.md",
  "CONTRIBUTING.md",
  ".github/CODEOWNERS",
  ".github/PULL_REQUEST_TEMPLATE.md",
  ".github/workflows/pr.yml",
  ".github/workflows/full.yml",
  "docs/adr/README.md",
  "docs/change-proposals/0002-p1-w8-acceptance-reference.md",
  "docs/engineering/context-map.md",
  "docs/phase-gates/phase-0.md",
  "docs/phase-gates/phase-1.md",
  "docs/work-packages/phase-1/README.md",
  "docs/work-packages/phase-1/P1-G0.md",
  ...Array.from(
    { length: 11 },
    (_, index) => `docs/work-packages/phase-1/P1-W${index + 1}.md`
  )
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

const agents = readFileSync("AGENTS.md", "utf8");
if (agents.includes("approved Phase 0 work package")) {
  console.error("AGENTS.md must not restrict execution policy to Phase 0.");
  process.exit(1);
}

const issueForm = readFileSync(
  ".github/ISSUE_TEMPLATE/work-package.yml",
  "utf8"
);
if (issueForm.includes('title: "[P0-W]')) {
  console.error("The work-package issue form must be phase-agnostic.");
  process.exit(1);
}

const requiredSections = [
  "## 1. Purpose",
  "## 2. Why now",
  "## 3. Inputs and decisions",
  "## 4. Dependencies",
  "## 5. Scope",
  "## 6. Non-goals",
  "## 7. Allowed paths",
  "## 8. Security constraints",
  "## 9. Implementation requirements",
  "## 10. Required tests",
  "## 11. Acceptance criteria",
  "## 12. Evidence required",
  "## 13. Stop and escalate",
  "## 14. Deliverables"
];

for (const packagePath of required.filter((path) =>
  path.startsWith("docs/work-packages/phase-1/P1-")
)) {
  // Normalise line endings: Windows checkouts get CRLF for Markdown, and the
  // frontmatter assertions below are written against LF.
  const content = readFileSync(packagePath, "utf8").replace(/\r\n/g, "\n");
  const packageId = packagePath.match(/(P1-(?:G0|W\d+))\.md$/)?.[1];
  const expectedStatus = packageId === "P1-G0"
    ? "accepted"
    : "approved-for-implementation";
  if (!packageId || !content.startsWith("---\n") ||
      !content.includes(`\nid: ${packageId}\n`) ||
      !content.includes(`\nstatus: ${expectedStatus}\n`)) {
    console.error(`${packagePath} has invalid work-package frontmatter.`);
    process.exit(1);
  }

  const missingSections = requiredSections.filter(
    (section) => !content.includes(section)
  );
  if (missingSections.length > 0) {
    console.error(
      `${packagePath} is missing sections:\n${missingSections.join("\n")}`
    );
    process.exit(1);
  }
}

console.log("Repository policy check passed.");
