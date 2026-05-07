# Issue #011: OTA Firmware Has No Signature Verification

**Severity:** HIGH

## Issue

Offline OTA firmware images are applied without any cryptographic signature check. An attacker with access to the OTA upload endpoint can flash arbitrary firmware.

## Location

`ota_offline.go` — apply handler.

## Fix

Require firmware packages to include a detached Ed25519 signature verified against a public key baked into the binary. Reject unsigned or invalid packages. Add a test with a tampered payload.

## Status

- [ ] TODO
