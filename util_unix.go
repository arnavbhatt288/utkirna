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

func GetRequiredHandles(handles *Handles, devPath string, imgPath string) error {
	err := unmountDisk(devPath)
	if err != nil {
		return err
	}
	handles.hDisk, err = unix.Open(devPath, unix.O_RDWR|unix.O_SYNC|unix.S_IRUSR|unix.S_IWUSR, 0)
	if err != nil {
		return err
	}

	handles.hImage, err = unix.Open(imgPath, unix.O_RDWR|unix.O_SYNC, 0)
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
	startsector int64,
	numsectors int64,
	sectorsize int,
) ([]byte, error) {
	data := make([]byte, int64(sectorsize)*numsectors)

	_, err := unix.Seek(fd, startsector*int64(sectorsize), unix.SEEK_SET)
	if err != nil {
		return nil, err
	}
	_, err = unix.Read(fd, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func WriteSectorDataFromHandle(fd int, data []byte, startsector int64, sectorsize int) error {
	_, err := unix.Seek(fd, startsector*int64(sectorsize), unix.SEEK_SET)
	if err != nil {
		return err
	}

	_, err = unix.Write(fd, data)
	if err != nil {
		return err
	}
	return err
}
