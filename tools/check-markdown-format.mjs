import { execFileSync } from "node:child_process";

execFileSync("git", ["diff", "--check"], { stdio: "inherit" });
