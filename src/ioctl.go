// (c) Copyright 2019 Hewlett Packard Enterprise Development LP

//go:build windows
// +build windows

// Package ioctl provides Windows IOCTL support
package main

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/sys/windows"
)

const (
	FILE_ANY_ACCESS     = 0
	FILE_READ_ACCESS    = 1
	FILE_SPECIAL_ACCESS = 0
	FILE_WRITE_ACCESS   = 2
	METHOD_BUFFERED     = 0
	METHOD_NEITHER      = 3
)

const (
	IOCTL_DISK_BASE    = 0x00000007
	IOCTL_SCSI_BASE    = 0x00000004
	IOCTL_STORAGE_BASE = 0x0000002D
	IOCTL_VOLUME_BASE  = 0x00000056
)

const (
	IOCTL_DISK_GET_DRIVE_GEOMETRY_EX     = (IOCTL_DISK_BASE << 16) | (FILE_ANY_ACCESS << 14) | (0x0028 << 2) | METHOD_BUFFERED
	IOCTL_SCSI_GET_ADDRESS               = (IOCTL_SCSI_BASE << 16) | (FILE_ANY_ACCESS << 14) | (0x0406 << 2) | METHOD_BUFFERED
	IOCTL_STORAGE_CHECK_VERIFY           = (IOCTL_STORAGE_BASE << 16) | (FILE_READ_ACCESS << 14) | (0x0200 << 2) | METHOD_BUFFERED
	IOCTL_STORAGE_CHECK_VERIFY2          = (IOCTL_STORAGE_BASE << 16) | (FILE_ANY_ACCESS << 14) | (0x0200 << 2) | METHOD_BUFFERED
	IOCTL_STORAGE_QUERY_PROPERTY         = (IOCTL_STORAGE_BASE << 16) | (FILE_ANY_ACCESS << 14) | (0x0500 << 2) | METHOD_BUFFERED
	IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS = (IOCTL_VOLUME_BASE << 16) | (FILE_ANY_ACCESS << 14) | (0x0000 << 2) | METHOD_BUFFERED
)

// MEDIA_TYPE enumeration
type MEDIA_TYPE uint32

const (
	Unknown        = iota // Format is unknown
	F5_1Pt2_512           // 5.25", 1.2MB,  512 bytes/sector
	F3_1Pt44_512          // 3.5",  1.44MB, 512 bytes/sector
	F3_2Pt88_512          // 3.5",  2.88MB, 512 bytes/sector
	F3_20Pt8_512          // 3.5",  20.8MB, 512 bytes/sector
	F3_720_512            // 3.5",  720KB,  512 bytes/sector
	F5_360_512            // 5.25", 360KB,  512 bytes/sector
	F5_320_512            // 5.25", 320KB,  512 bytes/sector
	F5_320_1024           // 5.25", 320KB,  1024 bytes/sector
	F5_180_512            // 5.25", 180KB,  512 bytes/sector
	F5_160_512            // 5.25", 160KB,  512 bytes/sector
	RemovableMedia        // Removable media other than floppy
	FixedMedia            // Fixed hard disk media
	F3_120M_512           // 3.5", 120M Floppy
	F3_640_512            // 3.5" ,  640KB,  512 bytes/sector
	F5_640_512            // 5.25",  640KB,  512 bytes/sector
	F5_720_512            // 5.25",  720KB,  512 bytes/sector
	F3_1Pt2_512           // 3.5" ,  1.2Mb,  512 bytes/sector
	F3_1Pt23_1024         // 3.5" ,  1.23Mb, 1024 bytes/sector
	F5_1Pt23_1024         // 5.25",  1.23MB, 1024 bytes/sector
	F3_128Mb_512          // 3.5" MO 128Mb   512 bytes/sector
	F3_230Mb_512          // 3.5" MO 230Mb   512 bytes/sector
	F8_256_128            // 8",     256KB,  128 bytes/sector
	F3_200Mb_512          // 3.5",   200M Floppy (HiFD)
	F3_240M_512           // 3.5",   240Mb Floppy (HiFD)
	F3_32M_512            // 3.5",   32Mb Floppy
)

// PARTITION_STYLE enumeration
type PARTITION_STYLE uint32

const (
	PARTITION_STYLE_MBR = iota
	PARTITION_STYLE_GPT
	PARTITION_STYLE_RAW
)

// STORAGE_BUS_TYPE enumeration
type STORAGE_BUS_TYPE uint32

const (
	BusTypeUnknown = iota
	BusTypeScsi
	BusTypeAtapi
	BusTypeAta
	BusType1394
	BusTypeSsa
	BusTypeFibre
	BusTypeUsb
	BusTypeRAID
	BusTypeiScsi
	BusTypeSas
	BusTypeSata
	BusTypeSd
	BusTypeMmc
	BusTypeVirtual
	BusTypeFileBackedVirtual
	BusTypeMax
	BusTypeMaxReserved = 0x7F
)

// STORAGE_PROPERTY_ID enumeration
type STORAGE_PROPERTY_ID uint32

const (
	StorageDeviceProperty = iota
	StorageAdapterProperty
	StorageDeviceIdProperty
	StorageDeviceUniqueIdProperty
	StorageDeviceWriteCacheProperty
	StorageMiniportProperty
	StorageAccessAlignmentProperty
	StorageDeviceSeekPenaltyProperty
	StorageDeviceTrimProperty
	StorageDeviceWriteAggregationProperty
	StorageDeviceDeviceTelemetryProperty
	StorageDeviceLBProvisioningProperty
	StorageDevicePowerProperty
	StorageDeviceCopyOffloadProperty
	StorageDeviceResiliencyProperty
	StorageDeviceMediumProductType
	StorageAdapterRpmbProperty
	StorageAdapterCryptoProperty
	StorageDeviceTieringProperty
	StorageDeviceFaultDomainProperty
	StorageDeviceClusportProperty
	StorageDeviceDependantDevicesProperty
	StorageDeviceIoCapabilityProperty = 48
	StorageAdapterProtocolSpecificProperty
	StorageDeviceProtocolSpecificProperty
	StorageAdapterTemperatureProperty
	StorageDeviceTemperatureProperty
	StorageAdapterPhysicalTopologyProperty
	StorageDevicePhysicalTopologyProperty
	StorageDeviceAttributesProperty
	StorageDeviceManagementStatus
	StorageAdapterSerialNumberProperty
	StorageDeviceLocationProperty
	StorageDeviceNumaProperty
	StorageDeviceZonedDeviceProperty
	StorageDeviceUnsafeShutdownCount
	StorageDeviceEnduranceProperty
)

// STORAGE_QUERY_TYPE enumeration
type STORAGE_QUERY_TYPE uint32

const (
	PropertyStandardQuery = iota
	PropertyExistsQuery
	PropertyMaskQuery
	PropertyQueryMaxDefined
)

type DISK_EXTENT struct {
	DiskNumber     uint32
	StartingOffset uint64
	ExtentLength   uint64
}

type DISK_PARTITION_INFO_MBR struct {
	SizeOfPartitionInfo uint32
	PartitionStyle      PARTITION_STYLE
	Signature           uint32
	CheckSum            uint32
}

type DISK_PARTITION_INFO_GPT struct {
	SizeOfPartitionInfo uint32
	PartitionStyle      PARTITION_STYLE
	DiskId              uuid.UUID
}

type DISK_GEOMETRY struct {
	Cylinders         uint64
	MediaType         MEDIA_TYPE
	TracksPerCylinder uint32
	SectorsPerTrack   uint32
	BytesPerSector    uint32
}

type DISK_GEOMETRY_EX struct {
	Geometry         DISK_GEOMETRY
	DiskSize         uint64
	DiskPartitionMBR *DISK_PARTITION_INFO_MBR
	DiskPartitionGPT *DISK_PARTITION_INFO_GPT
}

type DISK_GEOMETRY_EX_RAW struct {
	Geometry DISK_GEOMETRY
	DiskSize uint64
}

type STORAGE_DEVICE_DESCRIPTOR struct {
	Version               uint32
	Size                  uint32
	DeviceType            byte
	DeviceTypeModifier    byte
	RemovableMedia        bool
	CommandQueueing       bool
	VendorIdOffset        uint32
	ProductIdOffset       uint32
	ProductRevisionOffset uint32
	SerialNumberOffset    uint32
	BusType               STORAGE_BUS_TYPE
	RawPropertiesLength   uint32
	RawDeviceProperties   [1]byte
}

type STORAGE_PROPERTY_QUERY struct {
	PropertyId           STORAGE_PROPERTY_ID
	QueryType            STORAGE_QUERY_TYPE
	AdditionalParameters [1]byte
}

// Helper function to convert a disk number to a disk path
func DiskPathFromNumber(diskNumber uint32) string {
	return fmt.Sprintf("\\\\.\\PHYSICALDRIVE%v", diskNumber)
}

// GetVolumeDiskExtents issues an IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS to the given volume and
// returns back the volume's DISK_EXTENT array.
func GetVolumeDiskExtents(handle windows.Handle) (diskExtents []DISK_EXTENT, err error) {
	// We'll start with a buffer size of 256 bytes and grow it if IOCTL request indicates additional
	// space is needed.  Note that we default to 32 bytes in the C# code.  Extremely unlikely we'll
	// ever go beyond 256 bytes.
	dataBuffer := make([]uint8, 256)

	// Issue the IOCTL
	var bytesReturned uint32
	err = windows.DeviceIoControl(
		handle,
		IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS,
		nil,
		0,
		&dataBuffer[0],
		uint32(len(dataBuffer)),
		&bytesReturned,
		nil,
	)

	// If the buffer we passed in wasn't large enough, allocate a larger buffer and try once more
	if (err == windows.ERROR_INSUFFICIENT_BUFFER) || (err == windows.ERROR_MORE_DATA) {
		if bytesReturned >= 4 {

			// Extract the number of disk extents and determine required IOCTL buffer length
			numberOfDiskExtents := binary.LittleEndian.Uint32(dataBuffer[0:4])
			dataBufferLen := 8 + (numberOfDiskExtents * uint32(unsafe.Sizeof(diskExtents[0])))

			// We should only have one extent per volume so our buffer requirements should be
			// extremely small.  If there is a bug in the IOCTL handler, and it returns an
			// unreasonably large value, we'll limit the next request to 4K and log an error.
			const maxBufferLen = uint32(4096)
			if dataBufferLen > maxBufferLen {
				dataBufferLen = maxBufferLen
			}

			// Resize the buffer and try once more
			dataBuffer = make([]uint8, dataBufferLen)
			err = windows.DeviceIoControl(
				handle,
				IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS,
				nil,
				0,
				&dataBuffer[0],
				uint32(len(dataBuffer)),
				&bytesReturned,
				nil,
			)
		}
	}
	// If IOCTL was successful, extract the DISK_EXTENT array
	if (err == nil) && (bytesReturned >= 4) {
		numberOfDiskExtents := binary.LittleEndian.Uint32(dataBuffer[0:4])
		diskExtents = (*[1024]DISK_EXTENT)(unsafe.Pointer(&dataBuffer[8]))[:numberOfDiskExtents]
	}

	return diskExtents, err
}

// GetDiskGeometry issues an IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS to the given volume and
// returns back the volume's geometry details.
func GetDiskGeometry(handle windows.Handle) (diskGeometry *DISK_GEOMETRY_EX, err error) {
	// The DISK_GEOMETRY_EX structure is comprised of a DISK_GEOMETRY_EX structure, DISK_PARTITION_INFO
	// structure, and DISK_DETECTION_INFO structure which totals 112 bytes.  We'll allocate
	// a 128 byte buffer to submit with the IOCTL.
	dataBuffer := make([]uint8, 0x80)

	// Issue the IOCTL
	var bytesReturned uint32
	err = windows.DeviceIoControl(
		handle,
		IOCTL_DISK_GET_DRIVE_GEOMETRY_EX,
		nil,
		0,
		&dataBuffer[0],
		uint32(len(dataBuffer)),
		&bytesReturned,
		nil,
	)

	// If IOCTL was successful, extract the DISK_GEOMETRY_EX
	if err == nil {
		// Extract the raw structures
		diskGeometryBase := (*DISK_GEOMETRY_EX_RAW)(unsafe.Pointer(&dataBuffer[0x00]))
		diskPartitionMBR := (*DISK_PARTITION_INFO_MBR)(unsafe.Pointer(&dataBuffer[0x20]))
		diskPartitionGPT := (*DISK_PARTITION_INFO_GPT)(unsafe.Pointer(&dataBuffer[0x20]))

		// Build up and populate the return DISK_GEOMETRY_EX object
		diskGeometry = new(DISK_GEOMETRY_EX)
		diskGeometry.Geometry = diskGeometryBase.Geometry
		diskGeometry.DiskSize = diskGeometryBase.DiskSize
		switch diskPartitionMBR.PartitionStyle {
		case PARTITION_STYLE_MBR:
			diskGeometry.DiskPartitionMBR = diskPartitionMBR
		case PARTITION_STYLE_GPT:
			diskGeometry.DiskPartitionGPT = diskPartitionGPT
		}
	}

	return diskGeometry, err
}

func GetStorageProperty(
	handle windows.Handle,
) (deviceDescriptor STORAGE_DEVICE_DESCRIPTOR, err error) {
	var propertyQuery STORAGE_PROPERTY_QUERY
	var bytesReturned uint32
	outBuffer := make([]uint8, 128)

	propertyQuery.PropertyId = StorageDeviceProperty
	propertyQuery.QueryType = PropertyStandardQuery

	inBuffer := (*[unsafe.Sizeof(propertyQuery)]byte)(unsafe.Pointer(&propertyQuery))

	err = windows.DeviceIoControl(
		handle,
		IOCTL_STORAGE_QUERY_PROPERTY,
		&inBuffer[0],
		uint32(len(inBuffer)),
		&outBuffer[0],
		uint32(len(outBuffer)),
		&bytesReturned,
		nil,
	)

	if err == nil {
		deviceDescriptor = *(*STORAGE_DEVICE_DESCRIPTOR)(unsafe.Pointer(&outBuffer[0]))
	}
	return deviceDescriptor, err
}

func VerifyVolume(handle windows.Handle) bool {
	var bytesReturned uint32

	// ensure that the drive is actually accessible
	// multi-card hubs were reporting "removable" even when empty
	err := windows.DeviceIoControl(
		handle,
		IOCTL_STORAGE_CHECK_VERIFY2,
		nil,
		0,
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		// IOCTL_STORAGE_CHECK_VERIFY2 fails on sometimes, try the other (slower) method, just in case.
		err = windows.DeviceIoControl(
			handle,
			IOCTL_STORAGE_CHECK_VERIFY,
			nil,
			0,
			nil,
			0,
			&bytesReturned,
			nil,
		)
		if err != nil {
			return false
		}
	}
	return true
}
