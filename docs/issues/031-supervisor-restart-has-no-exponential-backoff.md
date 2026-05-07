# Issue #031: Supervisor Restart Has No Exponential Backoff

**Severity:** MEDIUM

## Issue

If `kvm_app` crashes it is restarted immediately by the supervisor script with no delay or backoff. A crash loop can saturate the CPU and prevent the device from accepting recovery SSH connections.

## Location

`S95kvmd` init script or equivalent supervisor.

## Fix

Add exponential backoff (1s, 2s, 4s, … cap 60s) to the restart loop. Log each restart with a timestamp.

## Status

- [ ] TODO — Init script lives in the Buildroot SDK overlay (`board/luckfox/picokvm/rootfs_overlay/etc/init.d/S95kvmd`) not in this repo. Fix requires updating the SDK overlay, not the Go application.
