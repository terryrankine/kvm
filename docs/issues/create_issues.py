import os

issues_dir = r'C:\Users\terryr\code\kvm\kvm\docs\issues'
os.makedirs(issues_dir, exist_ok=True)

issues = [
    ("001", "CRITICAL", "setConfigRaw Auth Bypass", """## Issue

`setConfigRaw` endpoint (or equivalent config write path) does not require authentication. Any unauthenticated caller can overwrite device configuration including the auth token, effectively locking out the legitimate owner or gaining persistent access.

## Location

`web.go` — config update handler registered without auth middleware.

## Fix

Register all `/api/` config mutation endpoints behind `authMiddleware()`. Add integration test that POSTs to `/api/config` without a session cookie and asserts HTTP 401.

## Status

- [ ] TODO
"""),
    ("002", "CRITICAL", "Plaintext Auth Token in Config", """## Issue

The session auth token (or password hash seed) is stored in plaintext inside the JSON config file on disk (`/userdata/picokvm/config.json`). If an attacker reads the config file via the OTA endpoint, directory traversal, or physical access they obtain the credential directly.

## Location

`config.go` — `authToken` field written as plain string.

## Fix

Store only a bcrypt hash of the token. On first boot generate a random 32-byte token, print it once to serial, store the hash. Add a test that reads back the config and asserts the stored value is a bcrypt hash (starts with `$2a$`).

## Status

- [ ] TODO
"""),
    ("003", "CRITICAL", "bcrypt Cost Too Low", """## Issue

Password hashing uses bcrypt with cost factor 10 (library default). For an embedded device that is rarely used for login, cost 10 is acceptable, but the value is not explicitly set and may silently change with library upgrades. More critically, the hash is compared in a timing-unsafe way in at least one code path.

## Location

`auth.go` (or equivalent) — `bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)`.

## Fix

Pin cost to `bcrypt.DefaultCost` (explicit constant). Use `bcrypt.CompareHashAndPassword` (already constant-time) everywhere — remove any `==` string comparisons on hashes. Add a test asserting the stored hash round-trips correctly via `CompareHashAndPassword`.

## Status

- [ ] TODO
"""),
    ("004", "CRITICAL", "Sequence Number Use-After-Increment Bug", """## Issue

In `native.go`, after sending a request the code does `delete(ongoingRequests, seq)` where `seq` has already been post-incremented. The delete targets the NEXT slot, not the current one, leaving the current entry to leak forever and potentially collide with a future request.

## Location

`native.go` — `ongoingRequests` cleanup after send.

## Fix

Capture `seq` into a local variable BEFORE incrementing, then use that local in `delete()`. Add a test that fires two sequential requests and verifies neither leaks in the map.

## Status

- [ ] TODO
"""),
    ("005", "CRITICAL", "native.go Concurrent Map Access Without Mutex", """## Issue

`ctrlSocketConn` and `ongoingRequests` in `native.go` are accessed from multiple goroutines (the read loop, the write loop, and caller goroutines) without any synchronization. This is a data race that the Go race detector will flag and that can cause map corruption or nil-pointer panics in production.

## Location

`native.go` — global `ctrlSocketConn *net.Conn` and `ongoingRequests map[uint32]chan`.

## Fix

Protect both with a `sync.Mutex` (or replace map with `sync.Map`). Run `go test -race ./...` and add a concurrent stress test.

## Status

- [ ] TODO
"""),
    ("006", "CRITICAL", "currentSession Global Without Mutex", """## Issue

`currentSession` is a package-level variable written by login/logout handlers and read by the auth middleware. No mutex protects it. Under concurrent requests (browser pre-fetch, WebSocket upgrade, API calls) this is a data race.

## Location

`web.go` — `var currentSession *Session`.

## Fix

Replace bare global with a `sync.RWMutex`-protected accessor pair `getSession()` / `setSession()`. Add `-race` to CI test flags.

## Status

- [ ] TODO
"""),
    ("007", "CRITICAL", "Keyboard HID Package-Level Lock", """## Issue

`hid_keyboard.go` uses a package-level mutex or global state shared across all HID keyboard instances. If multiple sessions or goroutines use the keyboard simultaneously the lock becomes a bottleneck and may deadlock if a caller panics while holding it.

## Location

`internal/usbgadget/hid_keyboard.go`.

## Fix

Move the lock inside the struct so each instance has its own lock. Add a test that creates two instances and writes concurrently to assert no deadlock.

## Status

- [ ] TODO
"""),
    ("008", "CRITICAL", "OTA Handler Missing Panic Recovery", """## Issue

`ota_offline.go` upload/apply handlers have no `recover()`. A panic during firmware extraction (malformed zip, out-of-memory) will crash the entire `kvm_app` process, requiring a manual power cycle to recover the device.

## Location

`ota_offline.go` — HTTP handlers for OTA upload and apply.

## Fix

Add a deferred `recover()` at the top of each handler that returns HTTP 500 with a safe error message. Add a test with a deliberately malformed payload and assert the server returns 500 and stays alive.

## Status

- [ ] TODO
"""),
    ("009", "CRITICAL", "Settings Persisted to localStorage (XSS Risk)", """## Issue

Sensitive UI settings (possibly including session tokens or device credentials visible in the UI) are written to `localStorage`. Any XSS vulnerability in the app can exfiltrate these values. `localStorage` also persists across sessions and is readable by any same-origin script.

## Location

`ui/src/hooks/stores.ts` — Zustand persist middleware writing to `localStorage`.

## Fix

Audit what is persisted. Move security-sensitive values (tokens, credentials) to `sessionStorage` or in-memory only. Add a test that mounts the store and asserts sensitive keys are not present in `localStorage`.

## Status

- [ ] TODO
"""),
    ("010", "HIGH", "WebSocket Origin Check Disabled", """## Issue

The WebSocket upgrade handler disables origin checking (`CheckOrigin: func(r *http.Request) bool { return true }`). This allows any origin to establish a WebSocket connection, enabling CSRF-style attacks via a malicious webpage visited by a user on the same network.

## Location

`web.go` — `websocket.Upgrader` configuration.

## Fix

Validate the `Origin` header against the device hostname and `localhost`. Reject connections from unknown origins with HTTP 403. Add a test with a spoofed Origin header.

## Status

- [ ] TODO
"""),
    ("011", "HIGH", "OTA Firmware Has No Signature Verification", """## Issue

Offline OTA firmware images are applied without any cryptographic signature check. An attacker with access to the OTA upload endpoint can flash arbitrary firmware.

## Location

`ota_offline.go` — apply handler.

## Fix

Require firmware packages to include a detached Ed25519 signature verified against a public key baked into the binary. Reject unsigned or invalid packages. Add a test with a tampered payload.

## Status

- [ ] TODO
"""),
    ("012", "HIGH", "HID Report Missing Input Validation", """## Issue

Mouse and keyboard HID reports accept raw values from the JSON-RPC call without range-checking. Out-of-range values written to the HID gadget device file can corrupt the USB descriptor or cause kernel driver errors.

## Location

`hid.go` or equivalent RPC handlers — `absMouseReport`, `relMouseReport`, `keyboardReport`.

## Fix

Clamp or reject out-of-range values before writing to the gadget device. Add tests for boundary values (0, 32767, -1, 32768 for abs mouse).

## Status

- [ ] TODO
"""),
    ("013", "HIGH", "actionSessions Map Race Condition", """## Issue

`actionSessions` (or equivalent active-session tracking map) is read and written from HTTP handler goroutines without a lock.

## Location

`web.go` — session management map.

## Fix

Protect with `sync.RWMutex`. Add `-race` test.

## Status

- [ ] TODO
"""),
    ("014", "HIGH", "Goroutine Leak in WebSocket Handler", """## Issue

When a WebSocket client disconnects abnormally the read goroutine may not be cleaned up, leaking goroutines over time. On a resource-constrained device (128 MB RAM) this can exhaust goroutine stack space.

## Location

`web.go` — WebSocket read loop.

## Fix

Use a `context.Context` cancellation or `close` channel. Ensure the read goroutine exits on any write error and vice versa. Add a test that connects and abruptly closes the connection and asserts goroutine count returns to baseline.

## Status

- [ ] TODO
"""),
    ("015", "HIGH", "Goroutine Leak in Native RPC Read Loop", """## Issue

The native RPC read goroutine in `native.go` may not exit when the underlying socket closes, leaking the goroutine and blocking callers waiting on response channels.

## Location

`native.go` — socket read loop.

## Fix

Signal all pending `ongoingRequests` channels with an error when the read loop exits. Add a test that closes the socket mid-request and asserts the caller unblocks with an error.

## Status

- [ ] TODO
"""),
    ("016", "HIGH", "Goroutine Leak in HTTP Stream Handler", """## Issue

The HTTP video stream handler (`/video/stream`) spawns goroutines that may not be cancelled when the client disconnects, leading to resource leaks.

## Location

`web.go` or `video.go` — `/video/stream` handler.

## Fix

Pass `r.Context()` to the streaming goroutine and check for cancellation. Add a test.

## Status

- [ ] TODO
"""),
    ("017", "HIGH", "USB Gadget Callbacks Race", """## Issue

USB gadget state change callbacks are invoked from a system event goroutine while the HID write path also accesses gadget state. No synchronization exists between them.

## Location

`internal/usbgadget/` — state change callbacks.

## Fix

Protect gadget state reads/writes with a mutex. Add `-race` test.

## Status

- [ ] TODO
"""),
    ("018", "HIGH", "WriteSample Errors Silently Dropped", """## Issue

`WriteSample` calls (video/audio frame writing to WebRTC track) discard errors with `_ =`. A write failure means frames are silently dropped rather than triggering reconnection or surfacing an error to the user.

## Location

`native.go` or `webrtc.go` — `track.WriteSample(...)`.

## Fix

Log write errors at WARN level, increment a counter, and trigger peer connection renegotiation after N consecutive failures. Add a test.

## Status

- [ ] TODO
"""),
    ("019", "HIGH", "OTA Firmware Partial Copy Not Rolled Back", """## Issue

If the device loses power or the process is killed mid-firmware-copy, the partially-written firmware file is left in place. On next boot the device may attempt to boot from a corrupt image.

## Location

`ota_offline.go` — file copy during apply.

## Fix

Write firmware to a `.tmp` file then `os.Rename()` atomically. Add a test that kills the write mid-stream and verifies no partial file remains at the target path.

## Status

- [ ] TODO
"""),
    ("020", "HIGH", "Canvas Stream Bar Detection Runs After Component Unmount", """## Issue

`detectStreamBars` is called via `setTimeout(..., 1000)` after video starts playing. If the component unmounts before the 1-second timeout fires, the callback still runs and calls `setStreamContentBounds` on an unmounted store, which may cause React state update warnings or stale closures.

## Location

`ui/src/layout/core/desktop/hooks/useVideoStream.ts` — `markAsPlaying` / `detectStreamBars`.

## Fix

Store the timeout ID and clear it in the `useEffect` cleanup. Add a test that unmounts the component before 1s and asserts no store updates occur.

## Status

- [ ] TODO
"""),
    ("021", "HIGH", "absMouseHidFile TOCTOU Race", """## Issue

The absolute mouse HID handler checks for file existence then opens it in two separate syscalls (check-then-act). Between the check and the open the file could be removed or replaced, causing writes to go to an unexpected file descriptor.

## Location

`hid.go` — absolute mouse device file open.

## Fix

Open the file directly and handle the error rather than pre-checking existence. Add a test.

## Status

- [ ] TODO
"""),
    ("022", "MEDIUM", "No CSRF Protection on State-Changing Endpoints", """## Issue

Endpoints that change device state (reboot, set password, OTA apply) have no CSRF token requirement. A malicious page visited by a logged-in user can trigger these actions via a cross-origin form POST.

## Location

`web.go` — state-changing API handlers.

## Fix

Require a `X-Requested-With: XMLHttpRequest` header (simple CSRF barrier) or implement proper CSRF tokens. Add a test.

## Status

- [ ] TODO
"""),
    ("023", "MEDIUM", "OTA Apply Has No Rollback on Boot Failure", """## Issue

After flashing new firmware there is no health check or rollback mechanism. If the new firmware fails to boot, the device is bricked until manual reflash.

## Location

`ota_offline.go`, bootloader integration.

## Fix

Implement a boot counter in persistent storage. If the device fails to reach a healthy state within N boots, revert to the previous firmware slot. Document the mechanism.

## Status

- [ ] TODO
"""),
    ("024", "MEDIUM", "Config File Not Written Atomically", """## Issue

Config is written by truncating and rewriting the existing file. A crash mid-write produces a zero-length or partially-written config, bricking the device configuration.

## Location

`config.go` — `saveConfig()` or equivalent.

## Fix

Write to a `.tmp` file then `os.Rename()`. Add a test that kills the write process mid-way and verifies the original config is intact.

## Status

- [ ] TODO
"""),
    ("025", "MEDIUM", "No Rate Limiting on Login Endpoint", """## Issue

The login endpoint has no rate limiting or lockout. An attacker on the local network can brute-force the password indefinitely.

## Location

`web.go` — login handler.

## Fix

Add exponential backoff or a fixed delay after failed attempts (e.g., 1s after 3 failures, lockout after 10). Add a test.

## Status

- [ ] TODO
"""),
    ("026", "MEDIUM", "otaState Not Protected by Mutex", """## Issue

`otaState` (OTA progress tracking struct) is read and written from HTTP handlers and background goroutines without a lock.

## Location

`ota_offline.go` — `otaState` struct.

## Fix

Add a `sync.Mutex` to `otaState`. Add `-race` test.

## Status

- [ ] TODO
"""),
    ("027", "MEDIUM", "Broadcaster Has Race on Subscriber Map", """## Issue

The event broadcaster's subscriber map is modified (add/remove) while potentially being iterated over in the broadcast goroutine, causing a map concurrent read/write panic.

## Location

`broadcast.go` or equivalent.

## Fix

Protect the subscriber map with a `sync.RWMutex` or use a channel-based design. Add `-race` test.

## Status

- [ ] TODO
"""),
    ("028", "MEDIUM", "chmod Return Value Not Checked", """## Issue

`os.Chmod()` calls on HID device files or config directories ignore the error return. On a read-only filesystem or permission error this silently fails, leading to confusing downstream failures.

## Location

Various — `chmod` calls in `hid.go`, `config.go`.

## Fix

Check and log `chmod` errors. Add a test with a read-only target.

## Status

- [ ] TODO
"""),
    ("029", "MEDIUM", "EDID Read Errors Silently Dropped", """## Issue

EDID read failures are swallowed with `_ =` or empty error handlers. The UI never learns that EDID is unavailable, potentially displaying incorrect resolution options.

## Location

`video.go` or `edid.go`.

## Fix

Log EDID errors at WARN and surface a `edidAvailable: false` flag in the device status API. Add a test.

## Status

- [ ] TODO
"""),
    ("030", "MEDIUM", "GPIO Errors Silently Dropped", """## Issue

GPIO operations (ATX power button, reset button) ignore errors from the sysfs write. A failed button press gives no feedback to the user.

## Location

`atx.go` or `gpio.go`.

## Fix

Return and surface GPIO errors to the JSON-RPC caller. Add a test with a mocked GPIO path.

## Status

- [ ] TODO
"""),
    ("031", "MEDIUM", "Supervisor Restart Has No Exponential Backoff", """## Issue

If `kvm_app` crashes it is restarted immediately by the supervisor script with no delay or backoff. A crash loop can saturate the CPU and prevent the device from accepting recovery SSH connections.

## Location

`S95kvmd` init script or equivalent supervisor.

## Fix

Add exponential backoff (1s, 2s, 4s, … cap 60s) to the restart loop. Log each restart with a timestamp.

## Status

- [ ] TODO
"""),
    ("032", "MEDIUM", "WebSocket Connection Not Cleanly Closed on Server Shutdown", """## Issue

On graceful shutdown the server does not send WebSocket close frames to connected clients. Clients detect the drop as an error and show a disconnection error rather than a clean reconnect prompt.

## Location

`web.go` — server shutdown handler.

## Fix

On shutdown, iterate active WebSocket connections and send `CloseNormalClosure` frames before `httpServer.Shutdown()`. Add a test.

## Status

- [ ] TODO
"""),
    ("033", "MEDIUM", "Keyboard Capture Mode Has No Visual Confirmation", """## Issue

When the browser captures keyboard input (pointer lock or focus mode) there is no persistent visible indicator. Users may type into unintended applications thinking the KVM is not capturing.

## Location

`ui/src/layout/core/desktop/` — keyboard capture state display.

## Fix

Show a persistent banner or border highlight when keyboard capture is active. Add a Playwright test.

## Status

- [ ] TODO
"""),
    ("034", "MEDIUM", "Macro Payload Length Not Validated", """## Issue

Keyboard macro payloads are accepted without a length limit. A very long macro can cause the HID write loop to block for an extended period, locking out other input.

## Location

`web.go` or `hid.go` — macro execution handler.

## Fix

Limit macro payloads to a reasonable maximum (e.g., 1000 keystrokes). Return an error for oversized payloads. Add a test.

## Status

- [ ] TODO
"""),
    ("035", "MEDIUM", "XHR Response Parsed Without Schema Validation", """## Issue

API responses in the UI are parsed with direct property access (`data.field`) without validating the schema. Unexpected server responses (partial JSON, schema changes) cause silent undefined errors rather than surfaced error states.

## Location

`ui/src/hooks/useJsonRpc.ts` or API fetch hooks.

## Fix

Add runtime schema validation (zod or simple type guards) on API responses. Add a test with a malformed response.

## Status

- [ ] TODO
"""),
    ("036", "MEDIUM", "OTA Upload Path Not Validated Against Traversal", """## Issue

The OTA upload endpoint writes the uploaded file to a path derived from the request without fully sanitizing it. A crafted filename containing `../` could write outside the intended directory.

## Location

`ota_offline.go` — upload handler filename handling.

## Fix

Use `filepath.Base()` on the uploaded filename and join it to a fixed base directory. Assert the resulting path is within the expected directory. Add a test with a traversal payload.

## Status

- [ ] TODO
"""),
    ("037", "MEDIUM", "NTP Sync Status Not Surfaced to Frontend", """## Issue

The device syncs time via NTP but the frontend has no visibility into NTP sync status or last-sync time. Users cannot diagnose certificate or TLS errors that are actually caused by clock skew.

## Location

`web.go` — device status API; `ui/src/` — status display.

## Fix

Add `ntpSynced: bool` and `ntpLastSync: timestamp` to the device status API response. Display in the UI. Add a test.

## Status

- [ ] TODO
"""),
    ("038", "MEDIUM", "Hardcoded Paths for Luckfox vs JetKVM", """## Issue

Several file paths are hardcoded as `/userdata/jetkvm/` rather than `/userdata/picokvm/` (or vice versa). On a Luckfox device some paths will fail silently, falling back to default values or failing to persist config.

## Location

Various — `config.go`, `ota_offline.go`, init scripts.

## Fix

Define a single `dataDir` constant set at build time (via ldflags). Replace all hardcoded paths with the constant. Add a test that asserts all config operations use the correct base path.

## Status

- [ ] TODO
"""),
    ("039", "LOW", "License File Missing for Forked Dependencies", """## Issue

The repo includes vendored or modified third-party code without accompanying LICENSE files for those dependencies. This may violate the license terms of those dependencies (MIT/Apache require attribution).

## Location

`ui/` — vendored JS packages; Go module vendor directory.

## Fix

Run `go mod vendor` and `npm run build` with license extraction. Add a `LICENSES.md` or `THIRD_PARTY_NOTICES` file with all dependency licenses. Add a CI check.

## Status

- [ ] TODO
"""),
]

for num, severity, title, content in issues:
    filename = f"{num}-{title.lower().replace(' ', '-').replace('/', '-').replace('(', '').replace(')', '').replace(',', '')}.md"
    filepath = os.path.join(issues_dir, filename)
    header = f"# Issue #{num}: {title}\n\n**Severity:** {severity}\n\n"
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(header + content)
    print(f"Created: {filename}")

print(f"\nDone. {len(issues)} files created in {issues_dir}")
