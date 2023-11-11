//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

type Handles struct {
	hDisk  int
	hImage int
}

func isPermAvailable() bool {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Printf("[isRoot] Unable to get current user: %s", err)
	}
	return currentUser.Uid == "0"
}

func gatherSizeInBytes(fd int) (int64, error) {
	diskSize, err := unix.IoctlGetInt(fd, unix.BLKGETSIZE64)
	if err != nil {
		return 0, err
	}
	return int64(diskSize), nil
}

func getDiskSectorSize(fd int) (int, error) {
	sector, err := unix.IoctlGetInt(fd, unix.BLKSSZGET)
	if err != nil {
		return 0, err
	}

	return sector, nil
}

func isBlockRemovable(block string) bool {
	str, _ := filepath.EvalSymlinks("/sys/class/block/" + block + "/device")
	return strings.Contains(str, "usb") || strings.Contains(str, "mmc")
}

func isBlockRW(block string) bool {
	filename := "/sys/block/" + block + "/ro"

	data, _ := os.ReadFile(filename)
	trimmed_data := strings.TrimSpace(string(data))

	return strings.Contains(trimmed_data, "0")
}

func unmountDisk(selectedDrive string) error {
	data, err := os.ReadFile("/etc/mtab")
	if err != nil {
		return err
	}

	for _, sen := range strings.Split(string(data), "\n") {
		if strings.Contains(sen, selectedDrive) {
			res := strings.Split(sen, " ")
			err := unix.Unmount(res[1], unix.MNT_FORCE|unix.MNT_DETACH)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func CloseRequiredHandles(handles Handles) {
	unix.Close(handles.hDisk)
	unix.Close(handles.hImage)
}

func GetDisks() []string {
	drives := []string{}

	block_dir, _ := os.Open("/sys/block")
	blocks, _ := block_dir.Readdirnames(0)

	block_dir.Close()
	sort.Strings(sort.StringSlice(blocks))

	for _, block := range blocks {
		if isBlockRW(block) && isBlockRemovable(block) {
			if isBlockRemovable(block) {
				drives = append(drives, "/dev/"+block)
			}
		}
	}
	return drives
}

func GetRequiredHandles(handles *Handles, taskType TaskType, devPath string, imgPath string) error {
	var diskAccess, imageAccess int

	err := unmountDisk(devPath)
	if err != nil {
		return err
	}
	if taskType == START_WRITE {
		diskAccess = unix.O_RDWR | unix.O_DIRECT
		imageAccess = unix.O_RDONLY
	} else if taskType == START_VERIFY {
		diskAccess = unix.O_RDONLY
		imageAccess = unix.O_RDONLY | unix.O_DIRECT
	} else if taskType == START_READ {
		diskAccess = unix.O_RDONLY
		imageAccess = unix.O_WRONLY | unix.O_DIRECT
	}

	handles.hDisk, err = unix.Open(devPath, diskAccess, 0777)
	if err != nil {
		return err
	}

	handles.hImage, err = unix.Open(imgPath, imageAccess, 0777)
	if err != nil {
		unix.Close(handles.hDisk)
		return err
	}

	return nil
}

func GetNumDiskSector(fd int) (int64, int, error) {
	diskSector, err := getDiskSectorSize(fd)
	if err != nil {
		return 0, 0, err
	}

	diskSize, err := gatherSizeInBytes(fd)
	if err != nil {
		return 0, 0, err
	}

	diskNumSectors := diskSize / int64(diskSector)
	return diskNumSectors, diskSector, nil
}

func ReadSectorDataFromHandle(
	fd int,
	data *[]byte,
	startsector int64,
	sectorsize int,
) error {
	_, err := unix.Seek(fd, startsector*int64(sectorsize), unix.SEEK_SET)
	if err != nil {
		return err
	}

	_, err = unix.Read(fd, *data)
	return err
}

func WriteSectorDataFromHandle(fd int, data *[]byte, startsector int64, sectorsize int) error {
	_, err := unix.Seek(fd, startsector*int64(sectorsize), unix.SEEK_SET)
	if err != nil {
		return err
	}

	_, err = unix.Write(fd, *data)
	return err
}
