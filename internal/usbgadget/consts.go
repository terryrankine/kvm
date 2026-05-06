package usbgadget

import "time"

const dwc3Path = "/sys/bus/platform/drivers/dwc3"
const udcPath = "/sys/kernel/config/usb_gadget/kvm/UDC"

const hidWriteTimeout = 10 * time.Millisecond
