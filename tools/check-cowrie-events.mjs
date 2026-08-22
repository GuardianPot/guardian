import { readFileSync } from "node:fs";

const args = process.argv.slice(2);
const validate = args.includes("--validate");
const forbidden = args[args.indexOf("--forbidden") + 1] ?? "";
const hostileMarker = args[args.indexOf("--hostile") + 1] ?? "";

function normalize(raw, index) {
  const eventType = typeof raw.eventid === "string" ? raw.eventid : "unknown";
  const sessionId = typeof raw.session === "string" ? raw.session : "unknown";
  const data = {};

  if (eventType === "cowrie.login.success" || eventType === "cowrie.login.failed") {
    data.username = raw.username ?? "";
    data.auth_result = eventType.endsWith("success") ? "success" : "failure";
  } else if (eventType === "cowrie.command.input") {
    data.command = raw.input ?? "";
    data.classification = "untrusted-attacker-input";
  } else if (eventType === "cowrie.session.params") {
    data.architecture = raw.arch ?? "";
  } else if (eventType === "cowrie.client.version") {
    data.client_version = raw.version ?? "";
  } else if (eventType === "cowrie.log.closed") {
    data.ttylog_sha256 = raw.shasum ?? "";
    data.duration_ms = raw.duration_ms ?? 0;
  }

  return {
    event_id: `cowrie:${sessionId}:${eventType}:${index}`,
    schema_version: "guardian.telemetry.v1",
    observed_at: raw.timestamp ?? null,
    session_id: sessionId,
    protocol: raw.protocol ?? "ssh",
    event_type: eventType,
    source: { ip: raw.src_ip ?? null, port: raw.src_port ?? null },
    destination: { ip: raw.dst_ip ?? null, port: raw.dst_port ?? null },
    data
  };
}

const input = readFileSync(0, "utf8");
const normalized = [];
let malformed = 0;

for (const [index, line] of input.split(/\r?\n/).entries()) {
  if (!line.trim()) continue;
  try {
    normalized.push(normalize(JSON.parse(line), index));
  } catch {
    malformed += 1;
  }
}

for (const event of normalized) {
  process.stdout.write(`${JSON.stringify(event)}\n`);
}

if (malformed > 0) {
  console.error(`Skipped ${malformed} malformed Cowrie event line(s); valid evidence was preserved.`);
}

if (validate) {
  const types = new Set(normalized.map((event) => event.event_type));
  const required = [
    "cowrie.session.connect",
    "cowrie.login.failed",
    "cowrie.login.success",
    "cowrie.command.input",
    "cowrie.session.closed"
  ];
  const missing = required.filter((eventType) => !types.has(eventType));
  if (missing.length > 0) {
    console.error(`Missing canonical Cowrie event types: ${missing.join(", ")}`);
    process.exit(1);
  }

  const commandEvents = normalized.filter((event) => event.event_type === "cowrie.command.input");
  if (!commandEvents.some((event) => event.data.command.includes(hostileMarker))) {
    console.error("Hostile command marker was not preserved as bounded command data.");
    process.exit(1);
  }
  if (forbidden && JSON.stringify(normalized).includes(forbidden)) {
    console.error("Sensitive authentication material leaked into the canonical event output.");
    process.exit(1);
  }
  if (normalized.some((event) => Object.hasOwn(event.data, "execute") || Object.hasOwn(event.data, "instruction"))) {
    console.error("Untrusted command data was promoted to an executable/instruction field.");
    process.exit(1);
  }

  const commandSessions = new Set(commandEvents.map((event) => event.session_id));
  if (commandSessions.size === 0) {
    console.error("Command/session correlation is missing.");
    process.exit(1);
  }

  console.error(`Cowrie canonical event validation passed: ${normalized.length} valid event(s), ${malformed} malformed line(s) tolerated.`);
}
