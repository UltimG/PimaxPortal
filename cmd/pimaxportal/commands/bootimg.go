package commands

import (
	"encoding/binary"
	"fmt"
)

// BootImgHeader holds the parsed fields from an Android boot image v2 header.
// See https://source.android.com/docs/core/architecture/bootloader/boot-image-header
type BootImgHeader struct {
	KernelSize       uint32
	KernelAddr       uint32
	RamdiskSize      uint32
	RamdiskAddr      uint32
	SecondSize       uint32
	SecondAddr       uint32
	TagsAddr         uint32
	PageSize         uint32
	HeaderVersion    uint32
	OSVersion        uint32
	RecoveryDTBOSize uint32
	HeaderSize       uint32
	DTBSize          uint32
}

// ParseBootImgHeader parses an Android boot image v2 header from the first
// 1660 bytes of data. Returns an error if the magic is wrong, the data is
// too short, or the header version is not 2.
func ParseBootImgHeader(data []byte) (BootImgHeader, error) {
	if len(data) < 1660 {
		return BootImgHeader{}, fmt.Errorf("boot header too short: %d bytes", len(data))
	}
	if string(data[0:8]) != "ANDROID!" {
		return BootImgHeader{}, fmt.Errorf("bad boot magic: %q", data[0:8])
	}
	hdr := BootImgHeader{
		KernelSize:       binary.LittleEndian.Uint32(data[8:12]),
		KernelAddr:       binary.LittleEndian.Uint32(data[12:16]),
		RamdiskSize:      binary.LittleEndian.Uint32(data[16:20]),
		RamdiskAddr:      binary.LittleEndian.Uint32(data[20:24]),
		SecondSize:       binary.LittleEndian.Uint32(data[24:28]),
		SecondAddr:       binary.LittleEndian.Uint32(data[28:32]),
		TagsAddr:         binary.LittleEndian.Uint32(data[32:36]),
		PageSize:         binary.LittleEndian.Uint32(data[36:40]),
		HeaderVersion:    binary.LittleEndian.Uint32(data[40:44]),
		OSVersion:        binary.LittleEndian.Uint32(data[44:48]),
		RecoveryDTBOSize: binary.LittleEndian.Uint32(data[1632:1636]),
		HeaderSize:       binary.LittleEndian.Uint32(data[1644:1648]),
		DTBSize:          binary.LittleEndian.Uint32(data[1648:1652]),
	}
	if hdr.HeaderVersion != 2 {
		return BootImgHeader{}, fmt.Errorf("unsupported boot header version: %d (expected 2)", hdr.HeaderVersion)
	}
	return hdr, nil
}

// pageCount returns the number of pages needed to hold size bytes.
// Returns 0 when size is 0.
func pageCount(size, pageSize uint32) uint32 {
	if size == 0 {
		return 0
	}
	return (size + pageSize - 1) / pageSize
}

// DTBSectionOffset computes the byte offset of the DTB section within a
// boot image. Sections are page-aligned and laid out in order:
// header, kernel, ramdisk, second, recovery_dtbo, DTB.
func DTBSectionOffset(hdr BootImgHeader) int64 {
	pages := uint32(1) // header page
	pages += pageCount(hdr.KernelSize, hdr.PageSize)
	pages += pageCount(hdr.RamdiskSize, hdr.PageSize)
	pages += pageCount(hdr.SecondSize, hdr.PageSize)
	pages += pageCount(hdr.RecoveryDTBOSize, hdr.PageSize)
	return int64(pages) * int64(hdr.PageSize)
}
