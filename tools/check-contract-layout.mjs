import { existsSync } from "node:fs";

const requiredDirectories = ["proto", "openapi", "schemas"];
const missing = requiredDirectories.filter((path) => !existsSync(path));

if (missing.length > 0) {
  console.error(`Missing contract directories: ${missing.join(", ")}`);
  process.exit(1);
}

console.log("Contract layout check passed.");
