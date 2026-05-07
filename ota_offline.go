package kvm

// Offline firmware update via web UI.
//
// POST /ota/upload  – accepts a multipart upload containing:
//   - component: "app" or "system"
//   - file: the update archive (.tar.gz) whose contents are:
//       <binary>          – the executable or system tar
//       <binary>.sha256   – SHA-256 hex digest (optionally "hex  filename" format)
//
// The handler verifies the SHA-256 hash and stages the binary at the standard
// OTA path used by the existing TryUpdate flow.  No network access is needed.
//
// POST /ota/apply   – applies a previously uploaded and staged update, then
// reboots the device.
//
// Storage paths mirror the existing fork convention: /userdata/picokvm/…

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// maxOfflineUploadSize limits offline update archives to 200 MB.
	maxOfflineUploadSize = 200 << 20

	// Staged-binary paths – must match the paths already used by TryUpdate.
	offlineAppStagePath    = "/userdata/picokvm/bin/kvm_app"
	offlineSystemStagePath = "/userdata/picokvm/update_system.tar"
)

// offlineUpdateUploadResponse is the JSON body returned by POST /ota/upload.
type offlineUpdateUploadResponse struct {
	Verified bool   `json:"verified"`
	HashOK   bool   `json:"hashOK"`
	Error    string `json:"error,omitempty"`
}

// handleOfflineUpdateUpload handles POST /ota/upload.
// Accepts a multipart form with:
//   - component: "app" or "system"
//   - file:      .tar.gz archive containing <binary> and <binary>.sha256
func handleOfflineUpdateUpload(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Interface("panic", r).Msg("panic in offline update upload handler")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()
	if IsUpdatePending() {
		c.JSON(http.StatusConflict, offlineUpdateUploadResponse{
			Error: "an update is already in progress",
		})
		return
	}

	component := c.PostForm("component")
	if component != "app" && component != "system" {
		c.JSON(http.StatusBadRequest, offlineUpdateUploadResponse{
			Error: "component must be 'app' or 'system'",
		})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, offlineUpdateUploadResponse{
			Error: "missing or invalid file upload",
		})
		return
	}

	if file.Size > maxOfflineUploadSize {
		c.JSON(http.StatusRequestEntityTooLarge, offlineUpdateUploadResponse{
			Error: fmt.Sprintf("file exceeds maximum size of %d MB", maxOfflineUploadSize>>20),
		})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, offlineUpdateUploadResponse{
			Error: "failed to read uploaded file",
		})
		return
	}
	defer f.Close()

	// Extract to a temp directory so we can verify before staging.
	extractDir, err := os.MkdirTemp("", "picokvm-offline-update-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, offlineUpdateUploadResponse{
			Error: "failed to create temporary directory",
		})
		return
	}
	defer os.RemoveAll(extractDir)

	l := otaLogger.With().Str("component", component).Logger()

	bundle, err := extractOfflineArchive(f, extractDir, component)
	if err != nil {
		l.Warn().Err(err).Msg("offline archive extraction failed")
		c.JSON(http.StatusBadRequest, offlineUpdateUploadResponse{
			Error: fmt.Sprintf("invalid archive: %v", err),
		})
		return
	}

	if err := verifyOfflineBundle(bundle); err != nil {
		l.Warn().Err(err).Msg("offline bundle verification failed")
		c.JSON(http.StatusUnprocessableEntity, offlineUpdateUploadResponse{
			Error: fmt.Sprintf("verification failed: %v", err),
		})
		return
	}

	// Stage the verified binary at the standard OTA path.
	destPath, err := offlineComponentStagePath(component)
	if err != nil {
		c.JSON(http.StatusInternalServerError, offlineUpdateUploadResponse{
			Error: fmt.Sprintf("internal error: %v", err),
		})
		return
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		l.Error().Err(err).Msg("failed to create staging directory")
		c.JSON(http.StatusInternalServerError, offlineUpdateUploadResponse{
			Error: "failed to create staging directory",
		})
		return
	}

	if err := os.Rename(bundle.binaryPath, destPath); err != nil {
		// Rename may fail across filesystems; fall back to a copy.
		if err := offlineCopyFile(bundle.binaryPath, destPath); err != nil {
			l.Error().Err(err).Msg("failed to stage verified binary")
			c.JSON(http.StatusInternalServerError, offlineUpdateUploadResponse{
				Error: "failed to stage verified update file",
			})
			return
		}
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		l.Warn().Err(err).Msg("failed to set permissions on staged file")
	}

	l.Info().Bool("hashOK", true).Msg("offline update uploaded and verified")

	c.JSON(http.StatusOK, offlineUpdateUploadResponse{
		Verified: true,
		HashOK:   true,
	})
}

// offlineUpdateApplyRequest is the JSON body for POST /ota/apply.
type offlineUpdateApplyRequest struct {
	Component string `json:"component" binding:"required"`
}

// handleOfflineUpdateApply handles POST /ota/apply.
// Applies a previously uploaded and staged offline update.
func handleOfflineUpdateApply(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Interface("panic", r).Msg("panic in offline update apply handler")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()
	var req offlineUpdateApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Component != "app" && req.Component != "system" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component must be 'app' or 'system'"})
		return
	}

	destPath, err := offlineComponentStagePath(req.Component)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("internal error: %v", err)})
		return
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no staged update found; upload first"})
		return
	}

	l := otaLogger.With().Str("component", req.Component).Logger()
	l.Info().Msg("applying offline update")

	// Apply asynchronously — the device will reboot.
	go func() {
		if err := applyOfflineUpdate(context.Background(), req.Component); err != nil {
			l.Error().Err(err).Msg("offline update apply failed")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "update is being applied; device will reboot"})
}

// applyOfflineUpdate runs the flash step (system only) and then reboots.
// For app updates the staged binary is already in place; the boot sequence
// picks it up on next start.
func applyOfflineUpdate(ctx context.Context, component string) error {
	if otaState.Updating {
		return fmt.Errorf("update already in progress")
	}

	otaState.Updating = true
	now := time.Now()
	otaState.MetadataFetchedAt = &now
	triggerOTAStateUpdate()

	if component == "system" {
		systemTarPath := offlineSystemStagePath
		if _, err := os.Stat(systemTarPath); err != nil {
			otaState.Error = fmt.Sprintf("system update archive not found: %v", err)
			otaState.Updating = false
			triggerOTAStateUpdate()
			return fmt.Errorf("system update archive not found: %w", err)
		}

		cmd := exec.CommandContext(ctx, "rk_ota",
			"--misc=update",
			"--tar_path="+systemTarPath,
			"--save_dir=/userdata/picokvm/ota_save",
			"--partition=all",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			msg := fmt.Sprintf("rk_ota failed: %v\nOutput: %s", err, string(out))
			otaState.Error = msg
			otaState.Updating = false
			triggerOTAStateUpdate()
			return fmt.Errorf("%s", msg)
		}
	}

	// App: binary is already staged; mark progress complete.
	updatedAt := time.Now()
	otaState.AppUpdatedAt = &updatedAt
	otaState.AppUpdateProgress = 1
	triggerOTAStateUpdate()

	otaLogger.Info().Str("component", component).Msg("offline update applied; rebooting in 5s")

	time.Sleep(5 * time.Second)
	if err := exec.Command("reboot").Start(); err != nil {
		otaState.Error = fmt.Sprintf("failed to reboot: %v", err)
		otaState.Updating = false
		triggerOTAStateUpdate()
		return fmt.Errorf("failed to reboot: %w", err)
	}
	os.Exit(0)
	return nil
}

// --- archive extraction --------------------------------------------------

// offlineBundle holds the paths and metadata extracted from an offline archive.
type offlineBundle struct {
	binaryPath   string // absolute path to the extracted binary on disk
	expectedHash string // SHA-256 hex from the .sha256 file
	component    string
}

// extractOfflineArchive reads a gzipped tar from r, extracts it into destDir,
// and returns a populated offlineBundle.  It accepts archives with or without
// a leading directory component.  Exactly a binary and a .sha256 file are
// expected (plus an optional .sig file that is silently skipped for
// forward-compatibility).
func extractOfflineArchive(r io.Reader, destDir, component string) (*offlineBundle, error) {
	binaryName, err := offlineBinaryName(component)
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	bundle := &offlineBundle{component: component}
	fileCount := 0
	const maxFiles = 5 // binary + .sha256 + optional .sig / .pub + dir entry

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar: %w", err)
		}

		// Skip directory entries.
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Strip any leading directory component.
		name := filepath.Base(filepath.Clean(header.Name))

		if strings.Contains(name, "..") || filepath.IsAbs(name) {
			return nil, fmt.Errorf("path traversal detected: %s", header.Name)
		}

		fileCount++
		if fileCount > maxFiles {
			return nil, fmt.Errorf("archive contains more than %d files", maxFiles)
		}

		destPath := filepath.Join(destDir, name)

		switch {
		case name == binaryName:
			if err := offlineExtractFile(tr, destPath, os.FileMode(header.Mode)|0644); err != nil {
				return nil, fmt.Errorf("error extracting binary: %w", err)
			}
			bundle.binaryPath = destPath

		case name == binaryName+".sha256":
			raw, err := io.ReadAll(io.LimitReader(tr, 256))
			if err != nil {
				return nil, fmt.Errorf("error reading hash file: %w", err)
			}
			hashStr := strings.TrimSpace(string(raw))
			// Support "hex  filename" or plain "hex" formats.
			if idx := strings.IndexByte(hashStr, ' '); idx > 0 {
				hashStr = hashStr[:idx]
			}
			bundle.expectedHash = strings.ToLower(hashStr)

		default:
			// .sig, .pub, or anything else: drain and ignore.
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return nil, fmt.Errorf("error draining entry %s: %w", name, err)
			}
		}
	}

	if bundle.binaryPath == "" {
		return nil, fmt.Errorf("archive missing required binary: %s", binaryName)
	}
	if bundle.expectedHash == "" {
		return nil, fmt.Errorf("archive missing required hash file: %s.sha256", binaryName)
	}

	return bundle, nil
}

// verifyOfflineBundle checks the SHA-256 hash of the extracted binary.
func verifyOfflineBundle(bundle *offlineBundle) error {
	actual, err := offlineHashFile(bundle.binaryPath)
	if err != nil {
		return fmt.Errorf("error hashing file: %w", err)
	}
	if actual != bundle.expectedHash {
		return fmt.Errorf("SHA-256 mismatch: got %s, expected %s", actual, bundle.expectedHash)
	}
	otaLogger.Info().Str("hash", actual).Str("component", bundle.component).Msg("offline SHA-256 verified")
	return nil
}

// offlineComponentStagePath returns the filesystem path where the verified
// binary should be staged for the given component.
func offlineComponentStagePath(component string) (string, error) {
	switch component {
	case "app":
		return offlineAppStagePath, nil
	case "system":
		return offlineSystemStagePath, nil
	default:
		return "", fmt.Errorf("unknown component: %s", component)
	}
}

// offlineBinaryName returns the expected binary filename for a component.
func offlineBinaryName(component string) (string, error) {
	switch component {
	case "app":
		return "kvm_app", nil
	case "system":
		return "update_system.tar", nil
	default:
		return "", fmt.Errorf("unknown component: %s", component)
	}
}

// offlineExtractFile writes a tar entry to destPath.
func offlineExtractFile(tr *tar.Reader, destPath string, mode os.FileMode) error {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", destPath, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, tr); err != nil {
		return fmt.Errorf("error writing %s: %w", destPath, err)
	}
	return nil
}

// offlineHashFile computes the SHA-256 hex digest of the file at path.
func offlineHashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// offlineCopyFile copies src to dst (used when os.Rename fails across filesystems).
func offlineCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Sync()
}
