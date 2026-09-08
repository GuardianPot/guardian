/**
 * Generated from openapi/guardian.yaml. Do not edit.
 *
 * Run `npm run generate:constraints -w @guardianpot/web-console` after a
 * contract change. `generated:check` fails when this file is stale, so a
 * constraint cannot drift away from the contract it came from (WCX-11 8.2).
 */
export const CONSTRAINTS = {
  "EnvironmentWriteRequest": {
    "required": [
      "display_name"
    ],
    "fields": {
      "display_name": {
        "type": "string",
        "minLength": 1,
        "maxLength": 512
      }
    }
  },
  "ZoneWriteRequest": {
    "required": [
      "display_name",
      "cidr"
    ],
    "fields": {
      "display_name": {
        "type": "string",
        "minLength": 1,
        "maxLength": 512
      },
      "cidr": {
        "type": "string",
        "minLength": 9,
        "maxLength": 18,
        "pattern": "^(10|172|192)\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}/([1-9]|[12][0-9]|3[0-2])$"
      }
    }
  },
  "DecoyWriteRequest": {
    "required": [
      "zone_id",
      "display_name",
      "family",
      "persona",
      "address",
      "pack",
      "pack_version"
    ],
    "fields": {
      "zone_id": {
        "type": "string",
        "pattern": "^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
        "format": "uuid"
      },
      "display_name": {
        "type": "string",
        "minLength": 1,
        "maxLength": 512
      },
      "family": {
        "type": "string",
        "enum": [
          "ssh",
          "http",
          "postgres",
          "smb"
        ]
      },
      "persona": {
        "type": "string",
        "enum": [
          "linux_admin_server",
          "internal_admin_web_app",
          "database_server",
          "windows_file_service_host"
        ]
      },
      "address": {
        "type": "string",
        "minLength": 7,
        "maxLength": 15,
        "pattern": "^(10|172|192)\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$"
      },
      "pack": {
        "type": "string",
        "minLength": 1,
        "maxLength": 64,
        "pattern": "^[a-z][a-z0-9-]{0,63}$"
      },
      "pack_version": {
        "type": "string",
        "minLength": 5,
        "maxLength": 32,
        "pattern": "^[0-9]{1,6}\\.[0-9]{1,6}\\.[0-9]{1,6}$"
      }
    }
  }
} as const;
