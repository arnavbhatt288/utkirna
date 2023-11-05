package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"
)

type TaskType int

const (
	START_WRITE TaskType = iota
	START_READ
	START_VERIFY
)

func determineOptimalSize(numSectors int64, sector int64) int64 {
	if numSectors-sector >= int64(1024) {
		return int64(1024)
	} else {
		return numSectors - sector
	}
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func StartTimer(start time.Time, widgets GUI) chan struct{} {
	chQuit := make(chan struct{})
	go func() {
		for range time.Tick(time.Second) {
			select {
			case <-chQuit:
				return
			default:
				elapsed := time.Since(start)
				elapsedStr := fmtDuration(elapsed)
				widgets.elapsedLabel.SetText(elapsedStr)
			}
		}
	}()
	return chQuit
}

func cleanUp(data *MainData, gui GUI, handles Handles) {
	data.bQuitTimer <- struct{}{}
	close(data.bQuitTask)
	close(data.bQuitTimer)
	CloseRequiredHandles(handles)
	DisableCancelButton(gui, *data)
}

func StartMainTask(data *MainData, gui GUI) {
	var err error
	var handles Handles

	err = GetRequiredHandles(
		&handles,
		data.selectedDrive,
		data.imagePath,
	)
	if err != nil {
		HandleError(
			gui,
			data,
			errors.Join(errors.New("StartMainTask(): GetRequiredHandle failed"), err),
		)
		return
	}

	if data.taskType == START_WRITE || data.taskType == START_VERIFY {
		WriteVerifyDisk(data, gui, handles)
	} else if data.taskType == START_READ {
		ReadDisk(data, gui, handles)
	}
}

func ReadDisk(data *MainData, gui GUI, handles Handles) {
	elapsedTimer := time.Now()
	data.bQuitTimer = StartTimer(elapsedTimer, gui)

	diskNumSectors, diskSector, err := GetNumDiskSector(handles.hDisk)
	if err != nil {
		cleanUp(data, gui, handles)
		HandleError(gui, data, errors.Join(errors.New("ReadDisk(): GatherSizeInBytes failed"), err))
		return
	}

	if gui.mbrCheck.Checked {
		sectorData, err := ReadSectorDataFromHandle(handles.hDisk, 0, 1, 512)
		if err != nil {
			cleanUp(data, gui, handles)
			HandleError(
				gui,
				data,
				errors.Join(errors.New("ReadDisk(): ReadSectorDataFromHandle failed"), err),
			)
			return
		}
		diskNumSectors = int64(1)
		for i := 0; i < 4; i++ {
			partitionStartSector := binary.LittleEndian.Uint32(sectorData[0x1BE+8+16*i:])
			partitionNumSectors := binary.LittleEndian.Uint32(sectorData[0x1BE+12+16*i:])

			if int64(partitionStartSector+partitionNumSectors) > diskNumSectors {
				diskNumSectors = int64(partitionStartSector + partitionNumSectors)
			}
		}
	}

	gui.rwProgressBar.Max = float64(diskNumSectors - 1024)
	lasti := int64(0)
	updateTimer := time.Now()

	data.bQuitTask = make(chan struct{})
	go func() {
		defer cleanUp(data, gui, handles)
		for i := int64(0); i < diskNumSectors; i += 1024 {
			select {
			case <-data.bQuitTask:
				return
			default:
				set := determineOptimalSize(diskNumSectors, i)
				sectorData, err := ReadSectorDataFromHandle(
					handles.hDisk,
					i,
					set,
					diskSector,
				)

				if err != nil {
					HandleError(
						gui,
						data,
						errors.Join(
							errors.New("ReadDisk(): ReadSectorDataFromHandle failed"),
							err,
						),
					)
					return
				}

				err = WriteSectorDataFromHandle(handles.hImage, sectorData, i, diskSector)
				if err != nil {
					HandleError(
						gui,
						data,
						errors.Join(
							errors.New("ReadDisk(): WriteSectorDataFromHandle failed"),
							err,
						),
					)
					return
				}

				gui.rwProgressBar.SetValue(float64(i))
				if time.Since(updateTimer).Milliseconds() >= 1000 {
					mbPerSec := float64(
						(int64(diskSector) * (i - lasti)),
					) * (1000 / float64(time.Since(updateTimer).Milliseconds())) / 1024.0 / 1024.0
					setText := fmt.Sprintf("%f MB/s", mbPerSec)
					gui.speedLabel.SetText(setText)
					lasti = i
					updateTimer = time.Now()
				}
			}
		}
		gui.statusLabel.SetText("Success!")
	}()
}

func WriteVerifyDisk(data *MainData, gui GUI, handles Handles) {
	elapsedTimer := time.Now()
	data.bQuitTimer = StartTimer(elapsedTimer, gui)

	diskNumSectors, diskSector, err := GetNumDiskSector(handles.hDisk)
	if err != nil {
		cleanUp(data, gui, handles)
		HandleError(
			gui,
			data,
			errors.Join(errors.New("WriteVerifyDisk(): GatherSizeInBytes failed"), err),
		)
		return
	}

	imageStat, err := os.Stat(data.imagePath)
	if err != nil {
		cleanUp(data, gui, handles)
		HandleError(gui, data, err)
	}
	imageNumSectors := (imageStat.Size() / int64(diskSector)) + (imageStat.Size() % int64(diskSector))

	if imageNumSectors > diskNumSectors {
		if gui.ignoreSize.Checked {
			imageNumSectors = diskNumSectors
		} else {
			cleanUp(data, gui, handles)
			HandleError(gui, data, errors.New("WriteVerifyDisk(): Size of image is larger than of device"))
			return
		}
	}

	gui.rwProgressBar.Max = float64(imageNumSectors - 1024)
	lasti := int64(0)
	updateTimer := time.Now()

	data.bQuitTask = make(chan struct{})
	go func() {
		defer cleanUp(data, gui, handles)
		for i := int64(0); i < imageNumSectors; i += 1024 {
			select {
			case <-data.bQuitTask:
				return
			default:
				numSectors := determineOptimalSize(imageNumSectors, i)
				sectorData, err := ReadSectorDataFromHandle(
					handles.hImage,
					i,
					numSectors,
					diskSector,
				)

				if err != nil {
					HandleError(
						gui,
						data,
						errors.Join(
							errors.New("WriteVerifyDisk(): ReadSectorDataFromHandle failed"),
							err,
						),
					)
					return
				}

				if data.taskType == START_WRITE {
					err = WriteSectorDataFromHandle(
						handles.hDisk,
						sectorData,
						i,
						diskSector,
					)
					if err != nil {
						HandleError(
							gui,
							data,
							errors.Join(
								errors.New(
									"WriteVerifyDisk(): WriteSectorDataFromHandle failed",
								),
								err,
							),
						)
						return
					}
				}
				sectorData2, err := ReadSectorDataFromHandle(
					handles.hDisk,
					i,
					numSectors,
					diskSector,
				)
				if err != nil {
					HandleError(
						gui,
						data,
						errors.Join(
							errors.New("WriteVerifyDisk(): ReadSectorDataFromHandle failed"),
							err,
						),
					)
					return
				}

				if !bytes.Equal(sectorData, sectorData2) {
					strError := fmt.Sprintf(
						"WriteVerifyDisk(): Verification failed at sector: %d\n",
						i,
					)
					HandleError(gui, data, errors.New(strError))
					return
				}

				gui.rwProgressBar.SetValue(float64(i))
				if time.Since(updateTimer).Milliseconds() >= 1000 {
					mbPerSec := float64(
						(int64(diskSector) * (i - lasti)),
					) * (1000 / float64(time.Since(updateTimer).Milliseconds())) / 1024.0 / 1024.0
					setText := fmt.Sprintf("%f MB/s", mbPerSec)
					gui.speedLabel.SetText(setText)
					lasti = i
					updateTimer = time.Now()
				}
			}
		}
		gui.statusLabel.SetText("Success!")
	}()
}
