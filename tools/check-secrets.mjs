import { execFileSync } from "node:child_process";

let result = "";
try {
  result = execFileSync("git", ["grep", "-n", "-I", "-E", "(ghp_|github_pat_|AKIA[0-9A-Z]{16}|BEGIN (RSA|OPENSSH|EC) PRIVATE KEY)", "--", "."], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "ignore"]
  });
} catch (error) {
  if (error.status !== 1) throw error;
}

if (result.trim().length > 0) {
  console.error(`Potential secret material found:\n${result}`);
  process.exit(1);
}

console.log("Secret pattern check passed.");
