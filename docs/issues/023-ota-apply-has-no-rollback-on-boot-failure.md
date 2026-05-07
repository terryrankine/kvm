# Issue #023: OTA Apply Has No Rollback on Boot Failure

**Severity:** MEDIUM

## Issue

After flashing new firmware there is no health check or rollback mechanism. If the new firmware fails to boot, the device is bricked until manual reflash.

## Location

`ota_offline.go`, bootloader integration.

## Fix

Implement a boot counter in persistent storage. If the device fails to reach a healthy state within N boots, revert to the previous firmware slot. Document the mechanism.

## Status

- [ ] TODO
