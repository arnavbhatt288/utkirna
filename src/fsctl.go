//go:build windows
// +build windows

package main

import "golang.org/x/sys/windows"

const (
	FSCTL_LOCK_VOLUME     = uint32(0x90018)
	FSCTL_UNLOCK_VOLUME   = uint32(0x90019)
	FSCTL_DISMOUNT_VOLUME = uint32(0x90020)
)

func LockVolume(handle windows.Handle) error {
	var bytesReturned uint32
	err := windows.DeviceIoControl(handle, FSCTL_LOCK_VOLUME, nil, 0, nil, 0, &bytesReturned, nil)
	return err
}

func UnlockVolume(handle windows.Handle) {
	var bytesReturned uint32
	windows.DeviceIoControl(handle, FSCTL_UNLOCK_VOLUME, nil, 0, nil, 0, &bytesReturned, nil)
}

func UnmountVolume(handle windows.Handle) error {
	var bytesReturned uint32
	err := windows.DeviceIoControl(
		handle,
		FSCTL_DISMOUNT_VOLUME,
		nil,
		0,
		nil,
		0,
		&bytesReturned,
		nil,
	)
	return err
}
