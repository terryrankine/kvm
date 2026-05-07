package usbgadget

import (
	"errors"
)

func (u *UsbGadget) logWarn(msg string, err error) error {
	if err == nil {
		err = errors.New(msg)
	}

	u.log.Warn().Err(err).Msg(msg)

	if u.strictMode {
		return err
	}

	return nil
}

func (u *UsbGadget) logError(msg string, err error) error {
	if err == nil {
		err = errors.New(msg)
	}

	u.log.Error().Err(err).Msg(msg)

	if u.strictMode {
		return err
	}

	return nil
}
