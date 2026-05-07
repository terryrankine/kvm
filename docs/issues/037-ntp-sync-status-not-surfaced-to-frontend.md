# Issue #037: NTP Sync Status Not Surfaced to Frontend

**Severity:** MEDIUM

## Issue

The device syncs time via NTP but the frontend has no visibility into NTP sync status or last-sync time. Users cannot diagnose certificate or TLS errors that are actually caused by clock skew.

## Location

`web.go` — device status API; `ui/src/` — status display.

## Fix

Add `ntpSynced: bool` and `ntpLastSync: timestamp` to the device status API response. Display in the UI. Add a test.

## Status

- [ ] TODO
