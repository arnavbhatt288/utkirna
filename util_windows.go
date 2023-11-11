//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

type Handles struct {
	hVolume windows.Handle
	hDisk   windows.Handle
	hImage  windows.Handle
}

const driveLetters string = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func isPermAvailable() bool {
	elevated := windows.GetCurrentProcessToken().IsElevated()
	return elevated
}

func checkDrive(driveLetter string) bool {
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fmt.Sprintf("\\\\.\\%s", driveLetter)),
		windows.FILE_READ_DATA,
		windows.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)

	defer windows.CloseHandle(handle)
	if err == nil && VerifyVolume(handle) {
		driveType := windows.GetDriveType(windows.StringToUTF16Ptr(driveLetter + "\\"))

		// GetDriveType is not reliable for USB hard drives. Must use a better way for detection.
		if driveType == windows.DRIVE_FIXED || driveType == windows.DRIVE_REMOVABLE {
			deviceDescriptor, err := GetStorageProperty(handle)
			if err == nil {
				if ((driveType == windows.DRIVE_REMOVABLE) && (deviceDescriptor.BusType != BusTypeSata)) ||
					((driveType == windows.DRIVE_FIXED) && ((deviceDescriptor.BusType == BusTypeUsb) || (deviceDescriptor.BusType == BusTypeSd) || (deviceDescriptor.BusType == BusTypeMmc))) {
					return true
				}
			}
		}
	}
	return false
}

func getDevicePath(hVolume windows.Handle) (string, error) {
	diskExtends, err := GetVolumeDiskExtents(hVolume)
	if err != nil {
		return "", err
	}

	return DiskPathFromNumber(diskExtends[0].DiskNumber), nil
}

func CloseRequiredHandles(handles Handles) {
	UnlockVolume(handles.hVolume)
	windows.CloseHandle(handles.hVolume)
	windows.CloseHandle(handles.hDisk)
	windows.CloseHandle(handles.hImage)
}

func GetDisks() []string {
	drives := []string{}
	driveMask, _ := windows.GetLogicalDrives()

	for i := 0; driveMask != 0; i++ {
		if (driveMask & 1) == 1 {
			driveLetter := string(driveLetters[i]) + ":"
			if checkDrive(driveLetter) {
				drives = append(drives, string(driveLetters[i])+":")
			}
		}
		driveMask >>= 1
	}
	return drives
}

func GetNumDiskSector(handle windows.Handle) (int64, int, error) {
	diskGeometry, err := GetDiskGeometry(handle)
	if err != nil {
		return 0, 0, err
	}
	return int64(
			diskGeometry.DiskSize,
		) / int64(
			diskGeometry.Geometry.BytesPerSector,
		), int(
			diskGeometry.Geometry.BytesPerSector,
		), nil
}

/* To get physical handle, first get volume handle */
func GetRequiredHandles(handles *Handles, taskType TaskType, volPath string, imgPath string) error {
	var err error
	var diskAccess, imageAccess, diskFileFlags, imageFileFlags uint32

	if taskType == START_WRITE {
		diskAccess = windows.GENERIC_READ | windows.GENERIC_WRITE
		imageAccess = windows.GENERIC_READ
		diskFileFlags = windows.FILE_FLAG_WRITE_THROUGH | windows.FILE_FLAG_NO_BUFFERING
		imageFileFlags = windows.FILE_FLAG_SEQUENTIAL_SCAN
	} else if taskType == START_VERIFY {
		diskAccess = windows.GENERIC_READ
		imageAccess = windows.GENERIC_READ
		diskFileFlags = windows.FILE_FLAG_NO_BUFFERING
		imageFileFlags = windows.FILE_FLAG_SEQUENTIAL_SCAN
	} else {
		diskAccess = windows.GENERIC_READ
		imageAccess = windows.GENERIC_WRITE
		diskFileFlags = windows.FILE_FLAG_NO_BUFFERING
	}

	handles.hVolume, err = windows.CreateFile(
		windows.StringToUTF16Ptr(fmt.Sprintf("\\\\.\\%s", volPath)),
		diskAccess,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return err
	}

	devicePath, err := getDevicePath(handles.hVolume)
	if err != nil {
		windows.CloseHandle(handles.hVolume)
		return err
	}
	err = LockVolume(handles.hVolume)
	if err != nil {
		windows.CloseHandle(handles.hVolume)
		return err
	}
	err = UnmountVolume(handles.hVolume)
	if err != nil {
		UnlockVolume(handles.hVolume)
		windows.CloseHandle(handles.hVolume)
		return err
	}

	handles.hDisk, err = windows.CreateFile(
		windows.StringToUTF16Ptr(devicePath),
		diskAccess,
		windows.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		diskFileFlags,
		0,
	)
	if err != nil {
		UnlockVolume(handles.hVolume)
		windows.CloseHandle(handles.hVolume)
		return err
	}

	handles.hImage, err = windows.CreateFile(
		windows.StringToUTF16Ptr(imgPath),
		imageAccess,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		imageFileFlags,
		0,
	)
	if err != nil {
		UnlockVolume(handles.hVolume)
		windows.CloseHandle(handles.hVolume)
		windows.CloseHandle(handles.hDisk)
		return err
	}
	return nil
}

func ReadSectorDataFromHandle(
	hRead windows.Handle,
	data *[]byte,
	startsector int64,
	sectorsize int,
) error {
	_, err := windows.Seek(hRead, startsector*int64(sectorsize), windows.FILE_BEGIN)
	if err != nil {
		return err
	}
	_, err = windows.Read(hRead, *data)
	return err
}

func WriteSectorDataFromHandle(
	hWrite windows.Handle,
	data *[]byte,
	startsector int64,
	sectorsize int,
) error {
	_, err := windows.Seek(hWrite, startsector*int64(sectorsize), windows.FILE_BEGIN)
	if err != nil {
		return err
	}

	_, err = windows.Write(hWrite, *data)
	return err
}
