package kvm

import (
	"encoding/json"
	"sync"
)

// max frame size for 1080p video, specified in mpp venc setting
const maxFrameSize = 1920 * 1080 / 2

func writeCtrlAction(action string) error {
	actionMessage := map[string]string{
		"action": action,
	}
	jsonMessage, err := json.Marshal(actionMessage)
	if err != nil {
		return err
	}
	err = WriteCtrlMessage(jsonMessage)
	return err
}

type VideoInputState struct {
	Ready          bool    `json:"ready"`
	Error          string  `json:"error,omitempty"` //no_signal, no_lock, out_of_range
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FramePerSecond float64 `json:"fps"`
}

var lastVideoState VideoInputState
var lastVideoStateMu sync.RWMutex

func triggerVideoStateUpdate() {
	lastVideoStateMu.RLock()
	snapshot := lastVideoState
	lastVideoStateMu.RUnlock()
	go func() {
		writeJSONRPCEvent("videoInputState", snapshot, getSession())
	}()
}

func HandleVideoStateMessage(event CtrlResponse) {
	videoState := VideoInputState{}
	err := json.Unmarshal(event.Data, &videoState)
	if err != nil {
		logger.Warn().Err(err).Msg("Error parsing video state json")
		return
	}
	lastVideoStateMu.Lock()
	lastVideoState = videoState
	lastVideoStateMu.Unlock()
	triggerVideoStateUpdate()
	requestDisplayUpdate(true)
}

func rpcGetVideoState() (VideoInputState, error) {
	lastVideoStateMu.RLock()
	defer lastVideoStateMu.RUnlock()
	return lastVideoState, nil
}

func setForceHpd() error {
	err := rpcSetForceHpd(config.ForceHpd)
	return err
}

func setNpuAppStatus() error {
	err := rpcSetNpuAppStatus(config.NpuAppEnabled)
	return err
}
