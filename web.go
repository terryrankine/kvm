package kvm

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"kvm/internal/logging"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	gin_logger "github.com/gin-contrib/logger"
	"github.com/gin-gonic/contrib/secure"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/vearutop/statigz"
	"golang.org/x/crypto/bcrypt"
)

//nolint:typecheck
//go:embed all:static
var staticFiles embed.FS

type WebRTCSessionRequest struct {
	Sd         string   `json:"sd"`
	OidcGoogle string   `json:"OidcGoogle,omitempty"`
	IP         string   `json:"ip,omitempty"`
	ICEServers []string `json:"iceServers,omitempty"`
}

type SetPasswordRequest struct {
	Password string `json:"password"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type LocalDevice struct {
	AuthMode     *string `json:"authMode"`
	DeviceID     string  `json:"deviceId"`
	LoopbackOnly bool    `json:"loopbackOnly"`
}

type DeviceStatus struct {
	IsSetup bool `json:"isSetup"`
}

type SetupRequest struct {
	LocalAuthMode string `json:"localAuthMode"`
	Password      string `json:"password,omitempty"`
}

var cachableFileExtensions = []string{
	".jpg", ".jpeg", ".png", ".svg", ".gif", ".webp", ".ico", ".woff2",
}

func setupRouter(isSecureServer bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
	r := gin.Default()
	r.Use(gin_logger.SetLogger(
		gin_logger.WithLogger(func(*gin.Context, zerolog.Logger) zerolog.Logger {
			return *ginLogger
		}),
	))

	if !isSecureServer && config.TLSEnforce {
		r.Use(secure.Secure(secure.Options{
			AllowedHosts:          []string{},
			SSLRedirect:           true,
			SSLHost:               "",
			SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
			STSSeconds:            0,
			STSIncludeSubdomains:  false,
			FrameDeny:             false,
			ContentTypeNosniff:    true,
			BrowserXssFilter:      true,
			ContentSecurityPolicy: "default-src 'self'",
		}))
		return r
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to get rooted static files subdirectory")
	}
	staticFileServer := http.StripPrefix("/static", statigz.FileServer(
		staticFS.(fs.ReadDirFS),
	))

	r.Any("/debug/pprof/*any", gin.WrapH(http.DefaultServeMux))

	// Add a custom middleware to set cache headers for images
	// This is crucial for optimizing the initial welcome screen load time
	// By enabling caching, we ensure that pre-loaded images are stored in the browser cache
	// This allows for a smoother enter animation and improved user experience on the welcome screen
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/assets/immutable/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable") // Cache for 1 year
			c.Next()
			return
		}

		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			ext := filepath.Ext(c.Request.URL.Path)
			if slices.Contains(cachableFileExtensions, ext) {
				c.Header("Cache-Control", "public, max-age=300") // Cache for 5 minutes
				c.Header("Pragma", "")
				c.Header("Expires", "")
			}
		}

		c.Next()
	})

	r.GET("/robots.txt", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain")
		c.Header("Cache-Control", "public, max-age=31536000, immutable") // Cache for 1 year
		c.String(http.StatusOK, "User-agent: *\nDisallow: /")
	})

	r.Any("/static/*w", func(c *gin.Context) {
		staticFileServer.ServeHTTP(c.Writer, c.Request)
	})

	// Public routes (no authentication required)
	r.POST("/auth/login-local", handleLogin)

	// We use this to determine if the device is setup
	r.GET("/device/status", handleDeviceStatus)

	// We use this to setup the device in the welcome page
	r.POST("/device/setup", handleSetup)

	// A Prometheus metrics endpoint.
	// Requires auth (cookie or basic auth) when password mode is enabled,
	// open when localAuthMode is noPassword (consistent with other endpoints).
	r.GET("/metrics", metricsAuthMiddleware(), gin.WrapH(promhttp.Handler()))

	// Developer mode protected routes
	developerModeRouter := r.Group("/developer/")
	developerModeRouter.Use(basicAuthProtectedMiddleware(true))
	{
		// pprof
		developerModeRouter.GET("/pprof/", gin.WrapF(pprof.Index))
		developerModeRouter.GET("/pprof/cmdline", gin.WrapF(pprof.Cmdline))
		developerModeRouter.GET("/pprof/profile", gin.WrapF(pprof.Profile))
		developerModeRouter.POST("/pprof/symbol", gin.WrapF(pprof.Symbol))
		developerModeRouter.GET("/pprof/symbol", gin.WrapF(pprof.Symbol))
		developerModeRouter.GET("/pprof/trace", gin.WrapF(pprof.Trace))
		developerModeRouter.GET("/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
		developerModeRouter.GET("/pprof/block", gin.WrapH(pprof.Handler("block")))
		developerModeRouter.GET("/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		developerModeRouter.GET("/pprof/heap", gin.WrapH(pprof.Handler("heap")))
		developerModeRouter.GET("/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
		developerModeRouter.GET("/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))

		logging.AttachSSEHandler(developerModeRouter)
	}

	// Protected routes (allows both password and noPassword modes)
	protected := r.Group("/")
	protected.Use(protectedMiddleware())
	{
		/*
		 * Legacy WebRTC session endpoint
		 *
		 * This endpoint is maintained for backward compatibility when users upgrade from a version
		 * using the legacy HTTP-based signaling method to the new WebSocket-based signaling method.
		 *
		 * During the upgrade process, when the "Rebooting device after update..." message appears,
		 * the browser still runs the previous JavaScript code which polls this endpoint to establish
		 * a new WebRTC session. Once the session is established, the page will automatically reload
		 * with the updated code.
		 *
		 * Without this endpoint, the stale JavaScript would fail to establish a connection,
		 * causing users to see the "Rebooting device after update..." message indefinitely
		 * until they manually refresh the page, leading to a confusing user experience.
		 */
		protected.POST("/webrtc/session", handleWebRTCSession)
		protected.GET("/webrtc/signaling/client", handleLocalWebRTCSignal)
		protected.GET("/device", handleDevice)
		protected.POST("/auth/logout", handleLogout)

		protected.POST("/auth/password-local", handleCreatePassword)
		protected.PUT("/auth/password-local", handleUpdatePassword)
		protected.DELETE("/auth/local-password", handleDeletePassword)
		protected.POST("/storage/upload", handleUploadHttp)
		protected.GET("/storage/download", handleDownloadHttp)

		protected.POST("/ota/upload", handleOfflineUpdateUpload)
		protected.POST("/ota/apply", handleOfflineUpdateApply)
		protected.GET("/storage/sd-download", handleSDDownloadHttp)
		protected.POST("/api/rpc", handleRpcRequest)
		protected.GET("/terminal/ws", handleTerminalWS)
		protected.GET("/serial/ws", handleSerialWS)
		protected.GET("/video/stream", handleVideoStream)
	}

	// Catch-all route for SPA
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == "GET" && c.NegotiateFormat(gin.MIMEHTML) == gin.MIMEHTML {
			c.FileFromFS("/", http.FS(staticFS))
			return
		}
		c.Status(http.StatusNotFound)
	})

	return r
}

var currentSession *Session
var currentSessionMu sync.RWMutex

func getSession() *Session {
	currentSessionMu.RLock()
	defer currentSessionMu.RUnlock()
	return currentSession
}

func setSession(s *Session) {
	currentSessionMu.Lock()
	defer currentSessionMu.Unlock()
	currentSession = s
}

var (
	currentHTTPSessionID    string
	httpSessionToNotify     string
	httpSessionMu           sync.Mutex
	httpSessionLockVersion  int64
	httpSessionSeenVersions map[string]int64
	invalidHTTPSessions     map[string]bool
)

func handleWebRTCSession(c *gin.Context) {
	var req WebRTCSessionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := newSession(SessionConfig{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	sd, err := session.ExchangeOffer(req.Sd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	if prev := getSession(); prev != nil {
		writeJSONRPCEvent("otherSessionConnected", nil, prev)
		peerConn := prev.peerConnection
		go func() {
			time.Sleep(1 * time.Second)
			_ = peerConn.Close()
		}()
	}

	// Cancel any ongoing keyboard macro when session changes
	cancelKeyboardMacro()

	setSession(session)
	c.JSON(http.StatusOK, gin.H{"sd": sd})
}

var (
	pingMessage = []byte("ping")
	pongMessage = []byte("pong")
)

func handleLocalWebRTCSignal(c *gin.Context) {
	// get the source from the request
	source := c.ClientIP()

	connectionID := c.Query("id")
	if connectionID == "" {
		connectionID = uuid.New().String()
	}

	scopedLogger := websocketLogger.With().
		Str("component", "websocket").
		Str("source", source).
		Str("sourceType", "local").
		Str("connectionID", connectionID).
		Logger()

	scopedLogger.Info().Msg("new websocket connection established")

	wsOptions := &websocket.AcceptOptions{
		// Allow same-origin connections and connections with no Origin header
		// (e.g. native clients, curl). Reject cross-origin browser requests to
		// prevent CSRF-style attacks from malicious pages on the local network.
		OriginPatterns: []string{c.Request.Host},
		OnPingReceived: func(ctx context.Context, payload []byte) bool {
			scopedLogger.Debug().Bytes("payload", payload).Msg("ping frame received")

			metricConnectionTotalPingReceivedCount.WithLabelValues("local", source).Inc()
			metricConnectionLastPingReceivedTimestamp.WithLabelValues("local", source).SetToCurrentTime()

			return true
		},
	}

	wsCon, err := websocket.Accept(c.Writer, c.Request, wsOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Now use conn for websocket operations
	defer wsCon.Close(websocket.StatusNormalClosure, "")

	err = wsjson.Write(context.Background(), wsCon, gin.H{
		"type": "device-metadata",
		"data": gin.H{"deviceVersion": builtAppVersion, "connectionID": connectionID},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = handleWebRTCSignalWsMessages(wsCon, source, connectionID, &scopedLogger)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func handleWebRTCSignalWsMessages(
	wsCon *websocket.Conn,
	source string,
	connectionID string,
	scopedLogger *zerolog.Logger,
) error {
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer func() {
		cancelRun()
	}()

	// connection type
	var sourceType = "local"

	l := scopedLogger.With().
		Str("source", source).
		Str("sourceType", sourceType).
		Str("connectionID", connectionID).
		Logger()

	l.Info().Msg("new websocket connection established")

	go func() {
		for {
			time.Sleep(WebsocketPingInterval)

			if ctxErr := runCtx.Err(); ctxErr != nil {
				if !errors.Is(ctxErr, context.Canceled) {
					l.Warn().Str("error", ctxErr.Error()).Msg("websocket connection closed")
				} else {
					l.Trace().Str("error", ctxErr.Error()).Msg("websocket connection closed as the context was canceled")
				}
				return
			}

			// set the timer for the ping duration
			timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
				metricConnectionLastPingDuration.WithLabelValues(sourceType, source).Set(v)
				metricConnectionPingDuration.WithLabelValues(sourceType, source).Observe(v)
			}))

			l.Trace().Msg("sending ping frame")
			err := wsCon.Ping(runCtx)

			if err != nil {
				l.Warn().Str("error", err.Error()).Msg("websocket ping error")
				cancelRun()
				return
			}

			// dont use `defer` here because we want to observe the duration of the ping
			duration := timer.ObserveDuration()

			metricConnectionTotalPingSentCount.WithLabelValues(sourceType, source).Inc()
			metricConnectionLastPingTimestamp.WithLabelValues(sourceType, source).SetToCurrentTime()

			l.Trace().Str("duration", duration.String()).Msg("received pong frame")
		}
	}()

	for {
		typ, msg, err := wsCon.Read(runCtx)
		if err != nil {
			l.Warn().Str("error", err.Error()).Msg("websocket read error")
			return err
		}
		if typ != websocket.MessageText {
			// ignore non-text messages
			continue
		}

		var message struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if bytes.Equal(msg, pingMessage) {
			l.Info().Str("message", string(msg)).Msg("ping message received")
			err = wsCon.Write(context.Background(), websocket.MessageText, pongMessage)
			if err != nil {
				l.Warn().Str("error", err.Error()).Msg("unable to write pong message")
				return err
			}

			metricConnectionTotalPingReceivedCount.WithLabelValues(sourceType, source).Inc()
			metricConnectionLastPingReceivedTimestamp.WithLabelValues(sourceType, source).SetToCurrentTime()

			continue
		}

		err = json.Unmarshal(msg, &message)
		if err != nil {
			l.Warn().Str("error", err.Error()).Msg("unable to parse ws message")
			continue
		}

		if message.Type == "offer" {
			l.Info().Msg("new session request received")
			var req WebRTCSessionRequest
			err = json.Unmarshal(message.Data, &req)
			if err != nil {
				l.Warn().Str("error", err.Error()).Msg("unable to parse session request data")
				continue
			}

			if req.OidcGoogle != "" {
				l.Info().Str("oidcGoogle", req.OidcGoogle).Msg("new session request with OIDC Google")
			}

			metricConnectionSessionRequestCount.WithLabelValues(sourceType, source).Inc()
			metricConnectionLastSessionRequestTimestamp.WithLabelValues(sourceType, source).SetToCurrentTime()
			err = handleSessionRequest(runCtx, wsCon, req, source, &l)
			if err != nil {
				l.Warn().Str("error", err.Error()).Msg("error starting new session")
				continue
			}
		} else if message.Type == "new-ice-candidate" {
			l.Info().Str("data", string(message.Data)).Msg("The client sent us a new ICE candidate")
			var candidate webrtc.ICECandidateInit

			// Attempt to unmarshal as a ICECandidateInit
			if err := json.Unmarshal(message.Data, &candidate); err != nil {
				l.Warn().Str("error", err.Error()).Msg("unable to parse incoming ICE candidate data")
				continue
			}

			if candidate.Candidate == "" {
				l.Warn().Msg("empty incoming ICE candidate, skipping")
				continue
			}

			l.Info().Str("data", fmt.Sprintf("%v", candidate)).Msg("unmarshalled incoming ICE candidate")

			sess := getSession()
			if sess == nil {
				l.Warn().Msg("no current session, skipping incoming ICE candidate")
				continue
			}

			l.Info().Str("data", fmt.Sprintf("%v", candidate)).Msg("adding incoming ICE candidate to current session")
			if err = sess.peerConnection.AddICECandidate(candidate); err != nil {
				l.Warn().Str("error", err.Error()).Msg("failed to add incoming ICE candidate to our peer connection")
			}
		}
	}
}

func handleLogin(c *gin.Context) {
	if config.LocalAuthMode == "noPassword" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Login is disabled in noPassword mode"})
		return
	}

	ip := c.ClientIP()
	if allowed, wait := CheckRateLimit(ip); !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": fmt.Sprintf("Too many failed attempts. Please try again in %s", wait.Round(time.Second)),
		})
		return
	}

	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(config.HashedPassword), []byte(req.Password))
	if err != nil {
		RecordFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	RecordSuccess(ip)

	config.LocalAuthToken = uuid.New().String()

	// Set the cookie (Session cookie, expires on browser close)
	c.SetCookie("authToken", config.LocalAuthToken, 0, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

func handleLogout(c *gin.Context) {
	config.LocalAuthToken = ""
	if err := SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	// Clear the auth cookie
	c.SetCookie("authToken", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

func protectedMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.LocalAuthMode == "noPassword" {
			c.Next()
			return
		}

		authToken, err := c.Cookie("authToken")
		if err != nil || authToken != config.LocalAuthToken || authToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func sendErrorJsonThenAbort(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
	c.Abort()
}

// metricsAuthMiddleware authenticates the /metrics endpoint using either the
// session cookie or HTTP basic auth (for Prometheus scrape configs). When
// localAuthMode is noPassword, all requests are allowed through — consistent
// with every other endpoint on the device.
func metricsAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.LocalAuthMode == "noPassword" {
			c.Next()
			return
		}

		// Try cookie auth first (browser access)
		authToken, err := c.Cookie("authToken")
		if err == nil && authToken == config.LocalAuthToken && authToken != "" {
			c.Next()
			return
		}

		// Fall back to basic auth (Prometheus scraper)
		_, password, ok := c.Request.BasicAuth()
		if ok {
			if err := bcrypt.CompareHashAndPassword([]byte(config.HashedPassword), []byte(password)); err == nil {
				c.Next()
				return
			}
		}

		c.Header("WWW-Authenticate", `Basic realm="JetKVM Metrics"`)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
	}
}

func basicAuthProtectedMiddleware(requireDeveloperMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if requireDeveloperMode {
			devModeState, err := rpcGetDevModeState()
			if err != nil {
				sendErrorJsonThenAbort(c, http.StatusInternalServerError, "Failed to get developer mode state")
				return
			}

			if !devModeState.Enabled {
				sendErrorJsonThenAbort(c, http.StatusUnauthorized, "Developer mode is not enabled")
				return
			}
		}

		if config.LocalAuthMode == "noPassword" {
			sendErrorJsonThenAbort(c, http.StatusForbidden, "The resource is not available in noPassword mode")
			return
		}

		// calculate basic auth credentials
		_, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", "Basic realm=\"KVM\"")
			sendErrorJsonThenAbort(c, http.StatusUnauthorized, "Basic auth is required")
			return
		}

		ip := c.ClientIP()
		if allowed, wait := CheckRateLimit(ip); !allowed {
			sendErrorJsonThenAbort(c, http.StatusTooManyRequests, fmt.Sprintf("Too many failed attempts. Please try again in %s", wait.Round(time.Second)))
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(config.HashedPassword), []byte(password))
		if err != nil {
			RecordFailure(ip)
			sendErrorJsonThenAbort(c, http.StatusUnauthorized, "Invalid password")
			return
		}

		RecordSuccess(ip)

		c.Next()
	}
}

var (
	updateWebRouter = make(chan struct{})
)

func getBindAddress(listenPort int) string {
	var bindAddress string
	useIPv4 := config.NetworkConfig.IPv4Mode.String != "disabled"
	useIPv6 := config.NetworkConfig.IPv6Mode.String != "disabled"

	if config.LocalLoopbackOnly {
		if useIPv4 && useIPv6 {
			bindAddress = fmt.Sprintf("localhost:%d", listenPort)
		} else if useIPv4 {
			bindAddress = fmt.Sprintf("127.0.0.1:%d", listenPort)
		} else if useIPv6 {
			bindAddress = fmt.Sprintf("[::1]:%d", listenPort)
		}
	} else {
		if useIPv4 && useIPv6 {
			bindAddress = fmt.Sprintf(":%d", listenPort)
		} else if useIPv4 {
			bindAddress = fmt.Sprintf("0.0.0.0:%d", listenPort)
		} else if useIPv6 {
			bindAddress = fmt.Sprintf("[::]:%d", listenPort)
		}
	}
	return bindAddress
}

func RunWebServer() {
	r := setupRouter(false)

	bindAddress := getBindAddress(80)

	logger.Info().Str("bindAddress", bindAddress).Bool("loopbackOnly", config.LocalLoopbackOnly).Msg("Starting web server")
	server := &http.Server{
		Addr:    bindAddress,
		Handler: r,
	}

	go func() {
		for range updateWebRouter {
			if config.TLSEnforce {
				time.Sleep(3 * time.Second)
			}
			server.Handler = setupRouter(false)
		}
	}()

	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}

	close(updateWebRouter)
}

func handleDevice(c *gin.Context) {
	response := LocalDevice{
		AuthMode:     &config.LocalAuthMode,
		DeviceID:     GetDeviceID(),
		LoopbackOnly: config.LocalLoopbackOnly,
	}

	c.JSON(http.StatusOK, response)
}

func handleCreatePassword(c *gin.Context) {
	if config.HashedPassword != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password already set"})
		return
	}

	// We only allow users with noPassword mode to set a new password
	// Users with password mode are not allowed to set a new password without providing the old password
	// We have a PUT endpoint for changing the password, use that instead
	if config.LocalAuthMode != "noPassword" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password mode is not enabled"})
		return
	}

	var req SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	config.HashedPassword = string(hashedPassword)
	config.LocalAuthToken = uuid.New().String()
	config.LocalAuthMode = "password"
	if err := SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	// Set the cookie (Session cookie, expires on browser close)
	c.SetCookie("authToken", config.LocalAuthToken, 0, "/", "", false, true)

	c.JSON(http.StatusCreated, gin.H{"message": "Password set successfully"})
}

func handleUpdatePassword(c *gin.Context) {
	if config.HashedPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is not set"})
		return
	}

	// We only allow users with password mode to change their password
	// Users with noPassword mode are not allowed to change their password
	if config.LocalAuthMode != "password" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password mode is not enabled"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OldPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(config.HashedPassword), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect old password"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash new password"})
		return
	}

	config.HashedPassword = string(hashedPassword)
	config.LocalAuthToken = uuid.New().String()
	if err := SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	// Set the cookie (Session cookie, expires on browser close)
	c.SetCookie("authToken", config.LocalAuthToken, 0, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func handleDeletePassword(c *gin.Context) {
	if config.HashedPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is not set"})
		return
	}

	if config.LocalAuthMode != "password" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password mode is not enabled"})
		return
	}

	var req LoginRequest // Reusing LoginRequest struct for password
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(config.HashedPassword), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect password"})
		return
	}

	// Disable password
	config.HashedPassword = ""
	config.LocalAuthToken = ""
	config.LocalAuthMode = "noPassword"
	if err := SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.SetCookie("authToken", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Password disabled successfully"})
}

func handleDeviceStatus(c *gin.Context) {
	response := DeviceStatus{
		IsSetup: config.LocalAuthMode != "",
	}

	c.JSON(http.StatusOK, response)
}

func handleDeviceUIConfig(c *gin.Context) {
	configData, _ := json.Marshal(gin.H{
		"DEVICE_VERSION": builtAppVersion,
	})
	if configData == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal config"})
		return
	}

	response := fmt.Sprintf("window.KVM_CONFIG = %s;", configData)

	c.Data(http.StatusOK, "text/javascript; charset=utf-8", []byte(response))
}

func handleSetup(c *gin.Context) {
	// Check if the device is already set up
	if config.LocalAuthMode != "" || config.HashedPassword != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device is already set up"})
		return
	}

	var req SetupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.LocalAuthMode != "password" && req.LocalAuthMode != "noPassword" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid localAuthMode"})
		return
	}

	config.LocalAuthMode = req.LocalAuthMode

	if req.LocalAuthMode == "password" {
		if req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required for password mode"})
			return
		}

		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		config.HashedPassword = string(hashedPassword)
		config.LocalAuthToken = uuid.New().String()

		// Set the cookie (Session cookie, expires on browser close)
		c.SetCookie("authToken", config.LocalAuthToken, 0, "/", "", false, true)
	} else {
		// For noPassword mode, ensure the password field is empty
		config.HashedPassword = ""
		config.LocalAuthToken = ""
	}

	err := SaveConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device setup completed successfully"})
}

func handleRpcRequest(c *gin.Context) {
	var req JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON RPC request"})
		return
	}
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		var err error
		sessionID, err = c.Cookie("httpSessionId")
		if err != nil || sessionID == "" {
			sessionID = uuid.New().String()
			c.SetCookie("httpSessionId", sessionID, 7*24*60*60, "/", "", false, false)
		}
	}

	var event *JSONRPCEvent

	httpSessionMu.Lock()
	if httpSessionSeenVersions == nil {
		httpSessionSeenVersions = make(map[string]int64)
	}

	if invalidHTTPSessions == nil {
		invalidHTTPSessions = make(map[string]bool)
	}

	invalid := false

	if req.Method == "confirmOtherSession" {
		httpSessionLockVersion++
		currentHTTPSessionID = sessionID
		httpSessionToNotify = ""
		httpSessionSeenVersions[sessionID] = httpSessionLockVersion

		for id := range httpSessionSeenVersions {
			if id == sessionID {
				delete(invalidHTTPSessions, id)
				continue
			}
			invalidHTTPSessions[id] = true
		}

		//logger.Info().
		//	Str("sessionID", sessionID).
		//	Str("currentHTTPSessionID", currentHTTPSessionID).
		//	Str("httpSessionToNotify", httpSessionToNotify).
		//	Int64("httpSessionLockVersion", httpSessionLockVersion).
		//	Str("rpcMethod", req.Method).
		//	Msg("handleRpcRequest confirmOtherSession")
	} else {
		if invalidHTTPSessions[sessionID] {
			invalid = true
			event = &JSONRPCEvent{
				JSONRPC: "2.0",
				Method:  "sessionInvalidated",
			}
		} else {
			if _, ok := httpSessionSeenVersions[sessionID]; !ok {
				httpSessionSeenVersions[sessionID] = httpSessionLockVersion
			}

			//logger.Info().
			//	Str("sessionID", sessionID).
			//	Str("currentHTTPSessionID", currentHTTPSessionID).
			//	Str("httpSessionToNotify", httpSessionToNotify).
			//	Int64("httpSessionLockVersion", httpSessionLockVersion).
			//	Str("rpcMethod", req.Method).
			//	Msg("handleRpcRequest before session check")

			if currentHTTPSessionID == "" {
				currentHTTPSessionID = sessionID
			} else if sessionID == httpSessionToNotify {
				event = &JSONRPCEvent{
					JSONRPC: "2.0",
					Method:  "otherSessionConnected",
				}
				httpSessionToNotify = ""
			} else if sessionID != currentHTTPSessionID {
				if version, ok := httpSessionSeenVersions[sessionID]; ok && version >= httpSessionLockVersion {
					httpSessionToNotify = currentHTTPSessionID
					currentHTTPSessionID = sessionID
				}
			}

			//logger.Info().
			//	Str("sessionID", sessionID).
			//	Str("currentHTTPSessionID", currentHTTPSessionID).
			//	Str("httpSessionToNotify", httpSessionToNotify).
			//	Int64("httpSessionLockVersion", httpSessionLockVersion).
			//	Bool("hasEvent", event != nil).
			//	Str("rpcMethod", req.Method).
			//	Msg("handleRpcRequest after session check")
		}
	}
	httpSessionMu.Unlock()

	if invalid {
		response := JSONRPCResponse{
			JSONRPC: "2.0",
			Error: map[string]interface{}{
				"code":    -32001,
				"message": "Session invalidated",
			},
			ID: req.ID,
		}

		if event != nil {
			c.JSON(http.StatusOK, gin.H{
				"response": response,
				"event":    event,
			})
			return
		}

		c.JSON(http.StatusOK, response)
		return
	}

	response, _ := DispatchRPCRequest(req)

	if event != nil {
		c.JSON(http.StatusOK, gin.H{
			"response": response,
			"event":    event,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func handleVideoStream(c *gin.Context) {
	logger.Info().Msg("HTTP video stream request received")

	c.Header("Content-Type", "video/x-h264")
	c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// Flush headers immediately so client knows the connection is established
	c.Writer.Flush()
	logger.Info().Msg("HTTP video stream headers flushed, waiting for video data")

	id, ch := videoBroadcaster.Subscribe()
	logger.Info().Str("subscriber_id", id).Msg("subscribed to video broadcaster")
	defer func() {
		videoBroadcaster.Unsubscribe(id)
		logger.Info().Str("subscriber_id", id).Msg("unsubscribed from video broadcaster")
	}()

	ctx := c.Request.Context()
	frameCount := 0

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				logger.Info().Int("total_frames", frameCount).Msg("video broadcaster channel closed")
				return
			}
			frameCount++
			if frameCount == 1 {
				logger.Info().Int("size", len(data)).Msg("first video frame received")
			}
			if _, err := c.Writer.Write(data); err != nil {
				logger.Warn().Err(err).Int("total_frames", frameCount).Msg("error writing video data")
				return
			}
			c.Writer.Flush()
		case <-ctx.Done():
			logger.Info().Int("total_frames", frameCount).Msg("client disconnected")
			return
		}
	}
}
