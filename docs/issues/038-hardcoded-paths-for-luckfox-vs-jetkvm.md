# Issue #038: Hardcoded Paths for Luckfox vs JetKVM

**Severity:** MEDIUM

## Issue

Several file paths are hardcoded as `/userdata/jetkvm/` rather than `/userdata/picokvm/` (or vice versa). On a Luckfox device some paths will fail silently, falling back to default values or failing to persist config.

## Location

Various — `config.go`, `ota_offline.go`, init scripts.

## Fix

Define a single `dataDir` constant set at build time (via ldflags). Replace all hardcoded paths with the constant. Add a test that asserts all config operations use the correct base path.

## Status

- [x] ALREADY FIXED (for this fork) — All runtime paths already use `/userdata/picokvm/` (`native.go`, `ota.go`, `jsonrpc.go`, `internal/logging/`). `config.go` uses `/userdata/kvm_config.json` (neutral). No jetkvm paths remain in file system operations. Prometheus metric names still carry `jetkvm_` prefix (cosmetic, not a runtime path). No code change required.
