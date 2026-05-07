package kvm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"kvm/internal/hidrpc"
	"kvm/internal/logging"
	"kvm/internal/usbgadget"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
)

type Session struct {
	peerConnection *webrtc.PeerConnection
	VideoTrack     *webrtc.TrackLocalStaticSample
	AudioTrack     *webrtc.TrackLocalStaticRTP
	//AudioTrack               *webrtc.TrackLocalStaticSample
	ControlChannel           *webrtc.DataChannel
	RPCChannel               *webrtc.DataChannel
	HidChannel               *webrtc.DataChannel
	shouldUmountVirtualMedia bool

	rpcQueue chan webrtc.DataChannelMessage

	hidRPCAvailable          bool
	lastKeepAliveArrivalTime time.Time  // Track when last keep-alive packet arrived
	lastTimerResetTime       time.Time  // Track when auto-release timer was last reset
	keepAliveJitterLock      sync.Mutex // Protect jitter compensation timing state
	hidQueueLock             sync.Mutex
	hidQueue                 []chan hidQueueMessage

	keysDownStateQueue chan usbgadget.KeysDownState
}

func (s *Session) resetKeepAliveTime() {
	s.keepAliveJitterLock.Lock()
	defer s.keepAliveJitterLock.Unlock()
	s.lastKeepAliveArrivalTime = time.Time{} // Reset keep-alive timing tracking
	s.lastTimerResetTime = time.Time{}       // Reset auto-release timer tracking
}

type hidQueueMessage struct {
	webrtc.DataChannelMessage
	channel string
}

type SessionConfig struct {
	ICEServers []string
	LocalIP    string
	ws         *websocket.Conn
	Logger     *zerolog.Logger
}

func (s *Session) ExchangeOffer(offerStr string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(offerStr)
	if err != nil {
		return "", err
	}
	offer := webrtc.SessionDescription{}
	err = json.Unmarshal(b, &offer)
	if err != nil {
		return "", err
	}
	// Set the remote SessionDescription
	if err = s.peerConnection.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	// Create answer
	answer, err := s.peerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = s.peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	localDescription, err := json.Marshal(s.peerConnection.LocalDescription())
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(localDescription), nil
}

func (s *Session) initQueues() {
	s.hidQueueLock.Lock()
	defer s.hidQueueLock.Unlock()

	s.hidQueue = make([]chan hidQueueMessage, 0)
	for i := 0; i < 4; i++ {
		q := make(chan hidQueueMessage, 256)
		s.hidQueue = append(s.hidQueue, q)
	}
}

func (s *Session) handleQueues(index int) {
	for msg := range s.hidQueue[index] {
		onHidMessage(msg, s, index)
	}
}

const keysDownStateQueueSize = 64

func (s *Session) initKeysDownStateQueue() {
	s.keysDownStateQueue = make(chan usbgadget.KeysDownState, keysDownStateQueueSize)
	go s.handleKeysDownStateQueue()
}

func (s *Session) handleKeysDownStateQueue() {
	for state := range s.keysDownStateQueue {
		s.reportHidRPCKeysDownState(state)
	}
}

func (s *Session) enqueueKeysDownState(state usbgadget.KeysDownState) {
	if s == nil || s.keysDownStateQueue == nil {
		return
	}

	select {
	case s.keysDownStateQueue <- state:
	default:
		hidRPCLogger.Warn().Msg("dropping keys down state update; queue full")
	}
}

func getOnHidMessageHandler(session *Session, scopedLogger *zerolog.Logger, channel string) func(msg webrtc.DataChannelMessage) {
	return func(msg webrtc.DataChannelMessage) {
		l := scopedLogger.With().
			Str("channel", channel).
			Int("length", len(msg.Data)).
			Logger()
		if scopedLogger.GetLevel() > zerolog.DebugLevel {
			l = l.With().Str("data", string(msg.Data)).Logger()
		}

		if msg.IsString {
			l.Warn().Msg("received string data in HID RPC message handler")
			return
		}

		if len(msg.Data) < 1 {
			l.Warn().Msg("received empty data in HID RPC message handler")
			return
		}

		l.Trace().Msg("received data in HID RPC message handler")

		queueIndex := hidrpc.GetQueueIndex(hidrpc.MessageType(msg.Data[0]))
		if queueIndex >= len(session.hidQueue) || queueIndex < 0 {
			l.Warn().Int("queueIndex", queueIndex).Msg("received data in HID RPC message handler, but queue index not found")
			queueIndex = 3
		}

		queue := session.hidQueue[queueIndex]
		if queue != nil {
			queue <- hidQueueMessage{
				DataChannelMessage: msg,
				channel:            channel,
			}
		} else {
			l.Warn().Int("queueIndex", queueIndex).Msg("received data in HID RPC message handler, but queue is nil")
			return
		}
	}
}

func newSession(sessionConfig SessionConfig) (*Session, error) {
	webrtcSettingEngine := webrtc.SettingEngine{
		LoggerFactory: logging.GetPionDefaultLoggerFactory(),
	}
	webrtcSettingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeUDP6,
	})
	//iceServer := webrtc.ICEServer{}

	var scopedLogger *zerolog.Logger
	if sessionConfig.Logger != nil {
		l := sessionConfig.Logger.With().Str("component", "webrtc").Logger()
		scopedLogger = &l
	} else {
		scopedLogger = webrtcLogger
	}

	iceServers := []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	}
	if config.STUN != "" {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: []string{config.STUN},
		})
	}

	api := webrtc.NewAPI(webrtc.WithSettingEngine(webrtcSettingEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: iceServers,
	})
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("Failed to create PeerConnection")
		return nil, err
	}

	session := &Session{peerConnection: peerConnection}
	session.rpcQueue = make(chan webrtc.DataChannelMessage, 256)
	session.initQueues()
	session.initKeysDownStateQueue()

	go func() {
		for msg := range session.rpcQueue {
			// TODO: only use goroutine if the task is asynchronous
			go onRPCMessage(msg, session)
		}
	}()

	for i := 0; i < len(session.hidQueue); i++ {
		go session.handleQueues(i)
	}

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		defer func() {
			if r := recover(); r != nil {
				scopedLogger.Error().Interface("error", r).Msg("Recovered from panic in DataChannel handler")
			}
		}()

		scopedLogger.Info().Str("label", d.Label()).Uint16("id", *d.ID()).Msg("New DataChannel")

		switch d.Label() {
		case "hidrpc":
			session.HidChannel = d
			d.OnMessage(getOnHidMessageHandler(session, scopedLogger, "hidrpc"))
		// we won't send anything over the unreliable channels
		case "hidrpc-unreliable-ordered":
			d.OnMessage(getOnHidMessageHandler(session, scopedLogger, "hidrpc-unreliable-ordered"))
		case "hidrpc-unreliable-nonordered":
			d.OnMessage(getOnHidMessageHandler(session, scopedLogger, "hidrpc-unreliable-nonordered"))
		case "rpc":
			session.RPCChannel = d
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				// Enqueue to ensure ordered processing
				session.rpcQueue <- msg
			})
			triggerOTAStateUpdate()
			triggerVideoStateUpdate()
			triggerUSBStateUpdate()
		case "terminal":
			handleTerminalChannel(d)
		case "serial":
			handleSerialChannel(d)
		default:
			if strings.HasPrefix(d.Label(), uploadIdPrefix) {
				go handleUploadChannel(d)
			}
		}
	})

	if streamEncodecType == "hevc" {
		session.VideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH265}, "video", "kvm")
	} else {
		session.VideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "kvm")
	}
	if err != nil {
		return nil, err
	}

	session.AudioTrack, err = webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "kvm")
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("Failed to create VideoTrack")
		return nil, err
	}

	rtpSender, err := peerConnection.AddTrack(session.VideoTrack)
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("Failed to add VideoTrack to PeerConnection")
		return nil, err
	}

	audioRtpSender, err := peerConnection.AddTrack(session.AudioTrack)
	if err != nil {
		return nil, err
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	go func() {
		audioRtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := audioRtpSender.Read(audioRtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	var isConnected bool

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		scopedLogger.Info().Interface("candidate", candidate).Msg("WebRTC peerConnection has a new ICE candidate")
		if candidate != nil {
			err := wsjson.Write(context.Background(), sessionConfig.ws, gin.H{"type": "new-ice-candidate", "data": candidate.ToJSON()})
			if err != nil {
				scopedLogger.Warn().Err(err).Msg("failed to write new-ice-candidate to WebRTC signaling channel")
			}
		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		scopedLogger.Info().Str("connectionState", connectionState.String()).Msg("ICE Connection State has changed")
		if connectionState == webrtc.ICEConnectionStateConnected {
			if !isConnected {
				isConnected = true
				actionSessionsMu.Lock()
				actionSessions++
				isFirst := actionSessions == 1
				actionSessionsMu.Unlock()
				onActiveSessionsChanged()
				setNpuAppStatus()
				if isFirst {
					onFirstSessionConnected()
				}
			}
		}
		//state changes on closing browser tab disconnected->failed, we need to manually close it
		if connectionState == webrtc.ICEConnectionStateFailed {
			scopedLogger.Debug().Msg("ICE Connection State is failed, closing peerConnection")
			_ = peerConnection.Close()
		}
		if connectionState == webrtc.ICEConnectionStateClosed {
			scopedLogger.Debug().Msg("ICE Connection State is closed, unmounting virtual media")
			if session == getSession() {
				// Cancel any ongoing keyboard report multi when session closes
				cancelKeyboardMacro()
				setSession(nil)
			}
			// Stop RPC processor
			if session.rpcQueue != nil {
				close(session.rpcQueue)
				session.rpcQueue = nil
			}

			// Stop HID RPC processor
			for i := 0; i < len(session.hidQueue); i++ {
				close(session.hidQueue[i])
				session.hidQueue[i] = nil
			}

			close(session.keysDownStateQueue)
			session.keysDownStateQueue = nil

			if session.shouldUmountVirtualMedia {
				if err := rpcUnmountImage(); err != nil {
					scopedLogger.Warn().Err(err).Msg("unmount image failed on connection close")
				}
			}
			if isConnected {
				isConnected = false
				actionSessionsMu.Lock()
				actionSessions--
				isLast := actionSessions == 0
				actionSessionsMu.Unlock()
				onActiveSessionsChanged()
				if isLast {
					onLastSessionDisconnected()
				}
			}
		}
	})
	return session, nil
}

var actionSessions = 0
var actionSessionsMu sync.Mutex

func onActiveSessionsChanged() {
	requestDisplayUpdate(true)
}

func onFirstSessionConnected() {
	_ = writeCtrlAction("start_video")
	if config.AudioMode != "disabled" {
		StartNtpAudioServer(handleAudioClient)
	}
}

func onLastSessionDisconnected() {
	_ = writeCtrlAction("stop_video")
	StopNtpAudioServer()
}
