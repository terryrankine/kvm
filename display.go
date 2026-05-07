package kvm

import (
	"errors"
	"os"
	"strconv"
	"sync"
	"time"
)

var backlightState = 0 // 0 - NORMAL, 1 - DIMMED, 2 - OFF

var (
	displayedTexts  = make(map[string]string)
	screenStateLock = sync.Mutex{}
)

var (
	dimTicker *time.Ticker
	offTicker *time.Ticker
)

const (
	touchscreenDevice     string = "/dev/input/event0"
	backlightControlClass string = "/sys/class/backlight/backlight/brightness"
)

func lvObjSetState(objName string, state string) (*CtrlResponse, error) {
	return CallDisplayCtrlAction("lv_obj_set_state", map[string]any{"obj": objName, "state": state})
}

func lvLabelSetText(objName string, text string) (*CtrlResponse, error) {
	return CallDisplayCtrlAction("lv_label_set_text", map[string]any{"obj": objName, "text": text})
}

func lvDispSetRotation(rotation string) (*CtrlResponse, error) {
	return CallDisplayCtrlAction("lv_disp_set_rotation", map[string]any{"rotation": rotation})
}

func updateLabelIfChanged(objName string, newText string) {
	screenStateLock.Lock()
	defer screenStateLock.Unlock()

	if newText != "" && newText != displayedTexts[objName] {
		_, _ = lvLabelSetText(objName, newText)
		displayedTexts[objName] = newText
	}
}

func updateDisplay() {
	updateLabelIfChanged("Network_Address_IP_Label", networkState.IPv4String())
	updateLabelIfChanged("Version_Hostname_Label", GetHostname())

	if usbState == "configured" {
		_, _ = lvObjSetState("Main", "USB_CONNECTED")
	} else {
		_, _ = lvObjSetState("Main", "USB_DISCONNECTED")
	}
	_ = os.WriteFile("/userdata/usb_state", []byte(usbState), 0644)

	if lastVideoState.Ready {
		_, _ = lvObjSetState("Main", "HDMI_CONNECTED")
		_ = os.WriteFile("/userdata/hdmi_state", []byte("connected"), 0644)
	} else {
		_, _ = lvObjSetState("Main", "HDMI_DISCONNECTED")
		_ = os.WriteFile("/userdata/hdmi_state", []byte("disconnected"), 0644)
	}

	if networkState.IsUp() {
		_, _ = lvObjSetState("Network", "NETWORK")
	} else {
		_, _ = lvObjSetState("Network", "NO_NETWORK")
	}

}

var (
	displayInited     = false
	displayUpdateLock = sync.Mutex{}
	waitDisplayUpdate = sync.Mutex{}
)

func requestDisplayUpdate(shouldWakeDisplay bool) {
	displayUpdateLock.Lock()
	defer displayUpdateLock.Unlock()

	if !displayInited {
		displayLogger.Info().Msg("display not inited, skipping updates")
		return
	}
	go func() {
		if shouldWakeDisplay {
			wakeDisplay(false)
		}
		displayLogger.Debug().Msg("display updating")
		//TODO: only run once regardless how many pending updates
		updateDisplay()
	}()
}

func waitCtrlAndRequestDisplayUpdate(shouldWakeDisplay bool) {
	waitDisplayUpdate.Lock()
	defer waitDisplayUpdate.Unlock()

	waitDisplayCtrlClientConnected()
	requestDisplayUpdate(shouldWakeDisplay)
}

func updateStaticContents() {
	//contents that never change
	updateLabelIfChanged("Network_Address_Mac_Label", networkState.MACString())
	_, appVersion, err := GetLocalVersion()
	if err == nil {
		updateLabelIfChanged("Version_App_Version_Label", appVersion.String())
	}
}

// setDisplayBrightness sets /sys/class/backlight/backlight/brightness to alter
// the backlight brightness of the KVM hardware's display.
func setDisplayBrightness(brightness int) error {
	// NOTE: The actual maximum value for this is 255, but out-of-the-box, the value is set to 64.
	// The maximum set here is set to 100 to reduce the risk of drawing too much power (and besides, 255 is very bright!).
	if brightness > 200 || brightness < 0 {
		return errors.New("brightness value out of bounds, must be between 0 and 100")
	}

	// Check the display backlight class is available
	if _, err := os.Stat(backlightControlClass); errors.Is(err, os.ErrNotExist) {
		return errors.New("brightness value cannot be set, possibly not running on KVM hardware")
	}

	// Set the value
	bs := []byte(strconv.Itoa(brightness))
	err := os.WriteFile(backlightControlClass, bs, 0644)
	if err != nil {
		return err
	}

	displayLogger.Info().Int("brightness", brightness).Msg("set brightness")
	return nil
}

// tick_displayDim() is called when when dim ticker expires, it simply reduces the brightness
// of the display by half of the max brightness.
func tick_displayDim() {
	err := setDisplayBrightness(config.DisplayMaxBrightness / 2)
	if err != nil {
		displayLogger.Warn().Err(err).Msg("failed to dim display")
	}

	dimTicker.Stop()

	backlightState = 1
}

// tick_displayOff() is called when the off ticker expires, it turns off the display
// by setting the brightness to zero.
func tick_displayOff() {
	err := setDisplayBrightness(0)
	if err != nil {
		displayLogger.Warn().Err(err).Msg("failed to turn off display")
	}

	offTicker.Stop()

	backlightState = 2
}

// wakeDisplay sets the display brightness back to config.DisplayMaxBrightness and stores the time the display
// last woke, ready for displayTimeoutTick to put the display back in the dim/off states.
// Set force to true to skip the backlight state check, this should be done if altering the tickers.
func wakeDisplay(force bool) {
	if backlightState == 0 && !force {
		return
	}

	// Don't try to wake up if the display is turned off.
	if config.DisplayMaxBrightness == 0 {
		return
	}

	err := setDisplayBrightness(config.DisplayMaxBrightness)
	if err != nil {
		displayLogger.Warn().Err(err).Msg("failed to wake display")
	}

	if config.DisplayDimAfterSec != 0 {
		dimTicker.Reset(time.Duration(config.DisplayDimAfterSec) * time.Second)
	}

	if config.DisplayOffAfterSec != 0 {
		offTicker.Reset(time.Duration(config.DisplayOffAfterSec) * time.Second)
	}
	backlightState = 0
}

// watchTsEvents monitors the touchscreen for events and simply calls wakeDisplay() to ensure the
// touchscreen interface still works even with LCD dimming/off.
func watchTsEvents() {
	ts, err := os.OpenFile(touchscreenDevice, os.O_RDONLY, 0666)
	if err != nil {
		displayLogger.Warn().Err(err).Msg("failed to open touchscreen device")
		return
	}

	defer ts.Close()

	// This buffer is set to 24 bytes as that's the normal size of events on /dev/input
	// Reference: https://www.kernel.org/doc/Documentation/input/input.txt
	// This could potentially be set higher, to require multiple events to wake the display.
	buf := make([]byte, 24)
	for {
		_, err := ts.Read(buf)
		if err != nil {
			displayLogger.Warn().Err(err).Msg("failed to read from touchscreen device")
			return
		}

		wakeDisplay(false)
	}
}

// startBacklightTickers starts the two tickers for dimming and switching off the display
// if they're not already set. This is done separately to the init routine as the "never dim"
// option has the value set to zero, but time.NewTicker only accept positive values.
func startBacklightTickers() {
	// Don't start the tickers if the display is switched off.
	// Set the display to off if that's the case.
	if config.DisplayMaxBrightness == 0 {
		_ = setDisplayBrightness(0)
		return
	}

	// Stop existing tickers to prevent multiple active instances on repeated calls
	if dimTicker != nil {
		dimTicker.Stop()
	}

	if offTicker != nil {
		offTicker.Stop()
	}

	if config.DisplayDimAfterSec != 0 {
		displayLogger.Info().Msg("dim_ticker has started")
		dimTicker = time.NewTicker(time.Duration(config.DisplayDimAfterSec) * time.Second)

		go func() {
			for range dimTicker.C {
				tick_displayDim()
			}
		}()
	}

	if config.DisplayOffAfterSec != 0 {
		displayLogger.Info().Msg("off_ticker has started")
		offTicker = time.NewTicker(time.Duration(config.DisplayOffAfterSec) * time.Second)

		go func() {
			for range offTicker.C {
				tick_displayOff()
			}
		}()
	}
}

func initDisplay() {
	go func() {
		waitDisplayCtrlClientConnected()
		displayLogger.Info().Msg("setting initial display contents")
		time.Sleep(500 * time.Millisecond)
		_, _ = lvDispSetRotation(config.DisplayRotation)
		updateStaticContents()
		initTimeZone()
		displayInited = true
		displayLogger.Info().Msg("display inited")
		startBacklightTickers()
		wakeDisplay(true)
		requestDisplayUpdate(true)
	}()

	go watchTsEvents()
}
