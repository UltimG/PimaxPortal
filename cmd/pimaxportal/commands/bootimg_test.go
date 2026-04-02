package commands

import (
	"encoding/binary"
	"testing"
)

// buildTestBootHeader creates a 1660-byte Android boot image v2 header
// with the given field values.
func buildTestBootHeader(kernelSize, ramdiskSize, secondSize, recoveryDTBOSize, dtbSize, pageSize uint32) []byte {
	buf := make([]byte, 1660)
	copy(buf[0:8], "ANDROID!")
	binary.LittleEndian.PutUint32(buf[8:12], kernelSize)       // kernel_size
	binary.LittleEndian.PutUint32(buf[12:16], 0x00008000)      // kernel_addr
	binary.LittleEndian.PutUint32(buf[16:20], ramdiskSize)     // ramdisk_size
	binary.LittleEndian.PutUint32(buf[20:24], 0x01000000)      // ramdisk_addr
	binary.LittleEndian.PutUint32(buf[24:28], secondSize)      // second_size
	binary.LittleEndian.PutUint32(buf[28:32], 0x00F00000)      // second_addr
	binary.LittleEndian.PutUint32(buf[32:36], 0x00000100)      // tags_addr
	binary.LittleEndian.PutUint32(buf[36:40], pageSize)        // page_size
	binary.LittleEndian.PutUint32(buf[40:44], 2)               // header_version
	binary.LittleEndian.PutUint32(buf[44:48], 0x0B09000A)      // os_version
	binary.LittleEndian.PutUint32(buf[1632:1636], recoveryDTBOSize) // recovery_dtbo_size
	binary.LittleEndian.PutUint32(buf[1644:1648], 1660)        // header_size
	binary.LittleEndian.PutUint32(buf[1648:1652], dtbSize)     // dtb_size
	return buf
}

func TestParseBootImgHeader_Valid(t *testing.T) {
	data := buildTestBootHeader(40175628, 1892936, 0, 0, 3717799, 4096)
	hdr, err := ParseBootImgHeader(data)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.PageSize != 4096 {
		t.Fatalf("expected PageSize=4096, got %d", hdr.PageSize)
	}
	if hdr.KernelSize != 40175628 {
		t.Fatalf("expected KernelSize=40175628, got %d", hdr.KernelSize)
	}
	if hdr.RamdiskSize != 1892936 {
		t.Fatalf("expected RamdiskSize=1892936, got %d", hdr.RamdiskSize)
	}
	if hdr.DTBSize != 3717799 {
		t.Fatalf("expected DTBSize=3717799, got %d", hdr.DTBSize)
	}
	if hdr.HeaderVersion != 2 {
		t.Fatalf("expected HeaderVersion=2, got %d", hdr.HeaderVersion)
	}
	if hdr.HeaderSize != 1660 {
		t.Fatalf("expected HeaderSize=1660, got %d", hdr.HeaderSize)
	}
}

func TestParseBootImgHeader_BadMagic(t *testing.T) {
	data := make([]byte, 1660)
	copy(data[0:8], "NOTBOOT!")
	_, err := ParseBootImgHeader(data)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestParseBootImgHeader_WrongVersion(t *testing.T) {
	data := buildTestBootHeader(40175628, 1892936, 0, 0, 3717799, 4096)
	// Overwrite header version to 3
	binary.LittleEndian.PutUint32(data[40:44], 3)
	_, err := ParseBootImgHeader(data)
	if err == nil {
		t.Fatal("expected error for unsupported header version")
	}
}

func TestDTBSectionOffset(t *testing.T) {
	// With kernelSize=40175628, ramdiskSize=1892936, secondSize=0,
	// recoveryDTBOSize=0, pageSize=4096:
	// header: 1 page
	// kernel: pageCount(40175628, 4096) = 9808 pages
	// ramdisk: pageCount(1892936, 4096) = 462 pages
	// second: pageCount(0, 4096) = 0 pages
	// recovery_dtbo: pageCount(0, 4096) = 0 pages
	// total = 10271 pages => offset = 10271 * 4096 = 42069504... let me recalculate
	// Actually the expected value per the task is 42078208.
	// 42078208 / 4096 = 10273 pages
	// 1 + 9808 + 462 + 0 + 0 = 10271 — discrepancy.
	// Let me recheck: pageCount(1892936, 4096) = (1892936 + 4095) / 4096 = 1897031 / 4096 = 463
	// 1 + 9808 + 463 + 0 + 0 = 10272 — still off by 1.
	// pageCount(40175628, 4096) = (40175628 + 4095) / 4096 = 40179723 / 4096 = 9809
	// 1 + 9809 + 463 + 0 + 0 = 10273 => 10273 * 4096 = 42078208 ✓
	data := buildTestBootHeader(40175628, 1892936, 0, 0, 3717799, 4096)
	hdr, err := ParseBootImgHeader(data)
	if err != nil {
		t.Fatal(err)
	}
	offset := DTBSectionOffset(hdr)
	if offset != 42078208 {
		t.Fatalf("expected DTB offset 42078208, got %d", offset)
	}
}

func TestPageCount(t *testing.T) {
	tests := []struct {
		size     uint32
		pageSize uint32
		expected uint32
	}{
		{0, 4096, 0},
		{1, 4096, 1},
		{4096, 4096, 1},
		{4097, 4096, 2},
		{40175628, 4096, 9809},
	}
	for _, tt := range tests {
		got := pageCount(tt.size, tt.pageSize)
		if got != tt.expected {
			t.Fatalf("pageCount(%d, %d) = %d, want %d", tt.size, tt.pageSize, got, tt.expected)
		}
	}
}
