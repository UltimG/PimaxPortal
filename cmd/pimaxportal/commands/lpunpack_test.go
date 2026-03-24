package commands

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestLPMetadataGeometryMagic(t *testing.T) {
	if lpGeometryMagic != 0x616c4467 {
		t.Fatalf("expected 0x616c4467, got 0x%x", lpGeometryMagic)
	}
}

func TestParseLPGeometry_Valid(t *testing.T) {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, uint32(0x616c4467)) // magic
	binary.Write(buf, binary.LittleEndian, uint32(128))         // struct_size
	buf.Write(make([]byte, 32))                                 // checksum
	binary.Write(buf, binary.LittleEndian, uint32(848))          // metadata_max_size
	binary.Write(buf, binary.LittleEndian, uint32(3))            // metadata_slot_count
	binary.Write(buf, binary.LittleEndian, uint32(512))          // logical_block_size
	geo, err := parseLPGeometry(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if geo.MetadataMaxSize != 848 {
		t.Fatalf("expected 848, got %d", geo.MetadataMaxSize)
	}
}

func TestParseLPGeometry_BadMagic(t *testing.T) {
	buf := make([]byte, 128)
	binary.LittleEndian.PutUint32(buf[0:4], 0xDEADBEEF)
	_, err := parseLPGeometry(buf)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestNullTermString(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{[]byte("vendor_a\x00\x00\x00"), "vendor_a"},
		{[]byte("system_a\x00"), "system_a"},
		{[]byte{0x00}, ""},
		{[]byte("noterm"), "noterm"},
	}
	for _, tt := range tests {
		got := nullTermString(tt.input)
		if got != tt.expected {
			t.Errorf("nullTermString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseLPPartition(t *testing.T) {
	buf := make([]byte, 52)
	copy(buf[0:36], "vendor_a\x00")
	binary.LittleEndian.PutUint32(buf[36:40], 0x01) // attributes
	binary.LittleEndian.PutUint32(buf[40:44], 2)    // first_extent
	binary.LittleEndian.PutUint32(buf[44:48], 5)    // num_extents
	binary.LittleEndian.PutUint32(buf[48:52], 1)    // group_index

	p, err := parseLPPartition(buf)
	if err != nil {
		t.Fatal(err)
	}
	if nullTermString(p.Name[:]) != "vendor_a" {
		t.Fatalf("expected vendor_a, got %q", nullTermString(p.Name[:]))
	}
	if p.FirstExtent != 2 {
		t.Fatalf("expected FirstExtent=2, got %d", p.FirstExtent)
	}
	if p.NumExtents != 5 {
		t.Fatalf("expected NumExtents=5, got %d", p.NumExtents)
	}
}

func TestParseLPExtent(t *testing.T) {
	buf := make([]byte, 24)
	binary.LittleEndian.PutUint64(buf[0:8], 1024)  // num_sectors
	binary.LittleEndian.PutUint32(buf[8:12], 0)     // target_type (LINEAR)
	binary.LittleEndian.PutUint64(buf[12:20], 2048) // target_data
	binary.LittleEndian.PutUint32(buf[20:24], 0)    // target_source

	e, err := parseLPExtent(buf)
	if err != nil {
		t.Fatal(err)
	}
	if e.NumSectors != 1024 {
		t.Fatalf("expected 1024 sectors, got %d", e.NumSectors)
	}
	if e.TargetType != lpExtentTypeLinear {
		t.Fatalf("expected LINEAR type, got %d", e.TargetType)
	}
	if e.TargetData != 2048 {
		t.Fatalf("expected target_data=2048, got %d", e.TargetData)
	}
}

func TestParseLPTableDesc(t *testing.T) {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], 100)
	binary.LittleEndian.PutUint32(buf[4:8], 10)
	binary.LittleEndian.PutUint32(buf[8:12], 52)

	desc := parseLPTableDesc(buf)
	if desc.Offset != 100 {
		t.Fatalf("expected offset=100, got %d", desc.Offset)
	}
	if desc.NumEntries != 10 {
		t.Fatalf("expected num_entries=10, got %d", desc.NumEntries)
	}
	if desc.EntrySize != 52 {
		t.Fatalf("expected entry_size=52, got %d", desc.EntrySize)
	}
}

func TestParseLPMetadataHeader_Valid(t *testing.T) {
	// Build a minimal metadata header (128 bytes).
	buf := make([]byte, 256)
	binary.LittleEndian.PutUint32(buf[0:4], lpMetadataMagic)  // magic
	binary.LittleEndian.PutUint16(buf[4:6], 10)                // major_version
	binary.LittleEndian.PutUint16(buf[6:8], 2)                 // minor_version
	binary.LittleEndian.PutUint32(buf[8:12], 128)              // header_size
	// bytes 12..44: header_checksum (zeros)
	binary.LittleEndian.PutUint32(buf[44:48], 512)             // tables_size
	// bytes 48..80: tables_checksum (zeros)

	// Partitions descriptor at offset 80
	binary.LittleEndian.PutUint32(buf[80:84], 0)   // offset (relative to tables)
	binary.LittleEndian.PutUint32(buf[84:88], 3)   // num_entries
	binary.LittleEndian.PutUint32(buf[88:92], 52)  // entry_size

	// Extents descriptor at offset 92
	binary.LittleEndian.PutUint32(buf[92:96], 156)  // offset
	binary.LittleEndian.PutUint32(buf[96:100], 8)   // num_entries
	binary.LittleEndian.PutUint32(buf[100:104], 24) // entry_size

	// Groups descriptor at offset 104
	binary.LittleEndian.PutUint32(buf[104:108], 348) // offset
	binary.LittleEndian.PutUint32(buf[108:112], 2)   // num_entries
	binary.LittleEndian.PutUint32(buf[112:116], 48)  // entry_size

	// BlockDevices descriptor at offset 116
	binary.LittleEndian.PutUint32(buf[116:120], 444) // offset
	binary.LittleEndian.PutUint32(buf[120:124], 1)   // num_entries
	binary.LittleEndian.PutUint32(buf[124:128], 64)  // entry_size

	hdr, err := parseLPMetadataHeader(buf)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.MajorVersion != 10 {
		t.Fatalf("expected major=10, got %d", hdr.MajorVersion)
	}
	if hdr.HeaderSize != 128 {
		t.Fatalf("expected header_size=128, got %d", hdr.HeaderSize)
	}
	// Offsets should be adjusted by HeaderSize.
	if hdr.Partitions.Offset != 128 {
		t.Fatalf("expected partitions offset=128 (0+128), got %d", hdr.Partitions.Offset)
	}
	if hdr.Extents.Offset != 284 {
		t.Fatalf("expected extents offset=284 (156+128), got %d", hdr.Extents.Offset)
	}
	if hdr.Partitions.NumEntries != 3 {
		t.Fatalf("expected 3 partition entries, got %d", hdr.Partitions.NumEntries)
	}
}

func TestParseLPMetadataHeader_BadMagic(t *testing.T) {
	buf := make([]byte, 256)
	binary.LittleEndian.PutUint32(buf[0:4], 0xDEADBEEF)
	_, err := parseLPMetadataHeader(buf)
	if err == nil {
		t.Fatal("expected error for bad metadata magic")
	}
}
