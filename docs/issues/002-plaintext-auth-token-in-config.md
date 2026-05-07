# Issue #002: Plaintext Auth Token in Config

**Severity:** CRITICAL

## Issue

The session auth token (or password hash seed) is stored in plaintext inside the JSON config file on disk (`/userdata/picokvm/config.json`). If an attacker reads the config file via the OTA endpoint, directory traversal, or physical access they obtain the credential directly.

## Location

`config.go` — `authToken` field written as plain string.

## Fix

Store only a bcrypt hash of the token. On first boot generate a random 32-byte token, print it once to serial, store the hash. Add a test that reads back the config and asserts the stored value is a bcrypt hash (starts with `$2a$`).

## Status

- [ ] TODO
