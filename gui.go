package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type MainData struct {
	taskType      TaskType
	selectedDrive string
	imagePath     string
	bQuitTimer    chan struct{}
	bQuitTask     chan struct{}
}

type GUI struct {
	cancelButton, readButton, writeButton, exitButton, openButton, reloadButton, verifyButton, saveButton *widget.Button
	selectDrive                                                                                           *widget.Select
	openPath, savePath                                                                                    *widget.Entry
	statusLabel, elapsedLabel, speedLabel                                                                 *widget.Label
	rwProgressBar                                                                                         *widget.ProgressBar
	window                                                                                                fyne.Window
	mbrCheck, ignoreSize                                                                                  *widget.Check
	guiTabs                                                                                               *container.AppTabs
}

func DisableCancelButton(widgets GUI, data MainData) {
	if data.taskType == START_WRITE || data.taskType == START_VERIFY {
		widgets.guiTabs.EnableIndex(1)
	} else if data.taskType == START_READ {
		widgets.guiTabs.EnableIndex(0)
	}

	widgets.selectDrive.Enable()
	widgets.reloadButton.Enable()
	widgets.openPath.Enable()
	widgets.savePath.Enable()
	widgets.openButton.Enable()
	widgets.exitButton.Enable()
	widgets.readButton.Enable()
	widgets.writeButton.Enable()
	widgets.saveButton.Enable()
	widgets.verifyButton.Enable()
	widgets.mbrCheck.Enable()
	widgets.ignoreSize.Enable()
	widgets.cancelButton.Disable()
	widgets.statusLabel.SetText("Standby...")
	widgets.speedLabel.SetText("")
	widgets.elapsedLabel.SetText("00:00:00")
	widgets.rwProgressBar.SetValue(0)
}

func enableCancelButton(widgets GUI, data MainData) {
	if data.taskType == START_WRITE || data.taskType == START_VERIFY {
		widgets.guiTabs.DisableIndex(1)
	} else if data.taskType == START_READ {
		widgets.guiTabs.DisableIndex(0)
	}

	widgets.selectDrive.Disable()
	widgets.reloadButton.Disable()
	widgets.openPath.Disable()
	widgets.savePath.Disable()
	widgets.openButton.Disable()
	widgets.exitButton.Disable()
	widgets.readButton.Disable()
	widgets.writeButton.Disable()
	widgets.saveButton.Disable()
	widgets.verifyButton.Disable()
	widgets.mbrCheck.Disable()
	widgets.ignoreSize.Disable()
	widgets.cancelButton.Enable()
}

func FileOpenDialog(myApp fyne.App, gui GUI) {
	window := myApp.NewWindow("Utkirna")
	window.CenterOnScreen()
	window.SetFixedSize(true)

	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, gui.window)
			window.Close()
		}
		if reader != nil {
			gui.openPath.SetText(reader.URI().Path())
		}
	}, window)
	fd.Show()
	fd.SetOnClosed(func() {
		window.Close()
	})

	requiredSize := fd.MinSize().Add(fd.MinSize())
	fd.Resize(requiredSize)

	window.Resize(requiredSize)
	window.Show()
}

func FileSaveDialog(myApp fyne.App, gui GUI) {
	window := myApp.NewWindow("Utkirna")
	window.CenterOnScreen()
	window.SetFixedSize(true)

	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, gui.window)
			window.Close()
		}
		if writer != nil {
			gui.savePath.SetText(writer.URI().Path())
		}
	}, window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".img"}))
	fd.Show()
	fd.SetOnClosed(func() {
		window.Close()
	})

	requiredSize := fd.MinSize().Add(fd.MinSize())
	fd.Resize(requiredSize)

	window.Resize(requiredSize)
	window.Show()
}

func HandleError(gui GUI, data *MainData, err error) {
	dialog.ShowError(err, gui.window)
	DisableCancelButton(gui, *data)
}

func HandleStartError() {
	tempApp := app.New()

	window := tempApp.NewWindow("Utkirna")
	window.CenterOnScreen()
	window.Resize(fyne.NewSize(350, 150))
	window.SetFixedSize(true)

	dialog := dialog.NewInformation(
		"Insufficient permissions",
		"Run the program with elevated permissions!",
		window,
	)
	dialog.Show()
	dialog.SetOnClosed(func() {
		window.Close()
	})

	requiredSize := window.Canvas().Size()
	dialog.Resize(requiredSize)

	window.ShowAndRun()
}

func StartGui() {
	var data MainData
	var gui GUI

	myApp := app.New()

	gui.window = myApp.NewWindow("Utkirna")
	gui.window.CenterOnScreen()
	gui.window.Resize(fyne.NewSize(600, 360))
	gui.window.SetFixedSize(true)

	drive_label := widget.NewLabel(("Select Drive:"))

	gui.selectDrive = widget.NewSelect(GetDisks(), func(s string) {
		data.selectedDrive = s
	})
	gui.reloadButton = widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), func() {
		data.selectedDrive = ""
		gui.selectDrive.ClearSelected()
		gui.selectDrive.Options = GetDisks()
	})
	drive := container.NewGridWithColumns(2,
		gui.selectDrive,
		gui.reloadButton,
	)

	selectImageLabel := widget.NewLabel("Select Image:")
	saveImageLabel := widget.NewLabel("Save Image:")

	gui.openPath = widget.NewEntry()
	gui.openPath.SetPlaceHolder("Location of the image to open")
	gui.savePath = widget.NewEntry()
	gui.savePath.SetPlaceHolder("Location of the image to save")

	gui.openButton = widget.NewButton("Open Image", func() {
		FileOpenDialog(myApp, gui)
	})
	gui.saveButton = widget.NewButton("Save Image", func() {
		FileSaveDialog(myApp, gui)
	})

	openImage := container.NewGridWithColumns(2,
		gui.openPath,
		gui.openButton,
	)
	saveImage := container.NewGridWithColumns(2,
		gui.savePath,
		gui.saveButton,
	)

	gui.mbrCheck = widget.NewCheck("Read only allocated partitions", func(b bool) {})
	gui.ignoreSize = widget.NewCheck("Ignore size limitations", func(b bool) {})

	gui.rwProgressBar = widget.NewProgressBar()

	gui.speedLabel = widget.NewLabel("")
	gui.speedLabel.Alignment = fyne.TextAlignCenter
	gui.statusLabel = widget.NewLabel("Standby...")
	gui.elapsedLabel = widget.NewLabel("00:00:00")
	gui.elapsedLabel.Alignment = fyne.TextAlignTrailing
	bottom_labels := container.NewGridWithColumns(
		3,
		gui.statusLabel,
		gui.speedLabel,
		gui.elapsedLabel,
	)

	gui.cancelButton = widget.NewButton("Cancel", func() {
		var cancelStr string
		if data.taskType == START_WRITE {
			cancelStr = "Cancelling the current operation may corrupt the destination drive.\nAre you sure to continue?"
		} else if data.taskType == START_VERIFY {
			cancelStr = "Are you sure to skip the verification of the drive?"
		} else if data.taskType == START_READ {
			cancelStr = "Current operation has not been finished. Are you sure to continue?"
		}

		dialog.ShowConfirm(
			"Cancellation",
			cancelStr,
			func(b bool) {
				if b {
					data.bQuitTask <- struct{}{}
					gui.statusLabel.SetText("Cancelled")
				}
			},
			gui.window,
		)
	})
	gui.cancelButton.Disable()

	gui.readButton = widget.NewButton("Read", func() {
		if len(data.selectedDrive) < 1 {
			dialog.ShowInformation(
				"Insufficient fields",
				"Select a drive to read from!",
				gui.window,
			)
		} else if len(gui.savePath.Text) < 1 {
			dialog.ShowInformation("Insufficient fields", "Select an image to write to!", gui.window)
		} else {
			dialog.ShowConfirm("Reading", "Are you sure to continue?", func(b bool) {
				if b {
					gui.statusLabel.SetText("Reading...")
					data.imagePath = gui.savePath.Text
					data.taskType = START_READ
					enableCancelButton(gui, data)
					StartMainTask(&data, gui)
				}
			}, gui.window)
		}
	})
	gui.writeButton = widget.NewButton("Write", func() {
		if len(data.selectedDrive) < 1 {
			dialog.ShowInformation("Insufficient fields", "Select a drive to write to!", gui.window)
		} else if len(gui.openPath.Text) < 1 {
			dialog.ShowInformation("Insufficient fields", "Select an image to write from!", gui.window)
		} else {
			dialog.ShowConfirm("Writing", "Are you sure to continue?", func(b bool) {
				if b {
					data.imagePath = gui.openPath.Text
					data.taskType = START_WRITE
					enableCancelButton(gui, data)
					gui.statusLabel.SetText("Writing...")
					StartMainTask(&data, gui)
				}
			}, gui.window)
		}
	})
	gui.verifyButton = widget.NewButton("Verify Only", func() {
		if len(data.selectedDrive) < 1 {
			dialog.ShowInformation("Insufficient fields", "Select a drive to verify!", gui.window)
		} else if len(gui.openPath.Text) < 1 {
			dialog.ShowInformation("Insufficient fields", "Select an image to verify from!", gui.window)
		} else {
			dialog.ShowConfirm("Verifying", "Are you sure to continue?", func(b bool) {
				if b {
					gui.statusLabel.SetText("Verifying...")
					data.imagePath = gui.openPath.Text
					data.taskType = START_VERIFY
					enableCancelButton(gui, data)
					StartMainTask(&data, gui)
				}
			}, gui.window)
		}
	})
	gui.exitButton = widget.NewButton("Exit", func() {
		gui.window.Close()
	})
	writeButtons := container.NewGridWithColumns(4,
		gui.cancelButton,
		gui.writeButton,
		gui.verifyButton,
		gui.exitButton)
	readButtons := container.NewGridWithColumns(3,
		gui.cancelButton,
		gui.readButton,
		gui.exitButton)

	writeTab := container.NewVBox(
		drive_label,
		drive,
		selectImageLabel,
		openImage,
		gui.ignoreSize,
		layout.NewSpacer(),
		gui.rwProgressBar,
		writeButtons,
		bottom_labels,
	)
	readTab := container.NewVBox(
		drive_label,
		drive,
		saveImageLabel,
		saveImage,
		gui.mbrCheck,
		layout.NewSpacer(),
		gui.rwProgressBar,
		readButtons,
		bottom_labels,
	)

	gui.guiTabs = container.NewAppTabs(
		container.NewTabItem("Write To Disk", writeTab),
		container.NewTabItem("Read From Disk", readTab),
	)
	gui.guiTabs.SetTabLocation(container.TabLocationTop)

	gui.window.SetContent(gui.guiTabs)
	gui.window.SetOnDropped(func(_ fyne.Position, urix []fyne.URI) {
		if len(urix) < 1 {
			return
		}
		uri := urix[0]
		if uri == nil {
			return
		}
		isDir, _ := storage.CanList(uri)
		if isDir {
			return
		}
		if gui.guiTabs.SelectedIndex() == 0 {
			gui.openPath.SetText(urix[0].Path())
		}
	})
	gui.window.Show()

	myApp.Run()
}
