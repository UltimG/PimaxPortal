package commands

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	lpGeometryMagic  = 0x616c4467
	lpMetadataMagic  = 0x414c5030
	lpGeometryOffset = 4096
	lpMetadataOffset = 8192 // lpGeometryOffset + 4096
	lpSectorSize     = 512

	lpExtentTypeLinear = 0
)

// lpGeometry is the LP metadata geometry header found at offset 4096 in a
// super partition image.
type lpGeometry struct {
	Magic             uint32
	StructSize        uint32
	Checksum          [32]byte
	MetadataMaxSize   uint32
	MetadataSlotCount uint32
	LogicalBlockSize  uint32
}

// lpMetadataHeader is the LP metadata header found at offset 8192.
type lpMetadataHeader struct {
	Magic          uint32
	MajorVersion   uint16
	MinorVersion   uint16
	HeaderSize     uint32
	HeaderChecksum [32]byte
	TablesSize     uint32
	TablesChecksum [32]byte
	Partitions     lpTableDescriptor
	Extents        lpTableDescriptor
	Groups         lpTableDescriptor
	BlockDevices   lpTableDescriptor
}

// lpTableDescriptor describes the location and layout of one metadata table.
type lpTableDescriptor struct {
	Offset     uint32
	NumEntries uint32
	EntrySize  uint32
}

// lpPartitionEntry represents a single partition in the LP metadata.
type lpPartitionEntry struct {
	Name         [36]byte
	Attributes   uint32
	FirstExtent  uint32
	NumExtents   uint32
	GroupIndex   uint32
}

// lpExtentEntry represents a single extent mapping in the LP metadata.
type lpExtentEntry struct {
	NumSectors   uint64
	TargetType   uint32
	TargetData   uint64
	TargetSource uint32
}

// parseLPGeometry parses an LP geometry header from the given byte slice.
func parseLPGeometry(data []byte) (lpGeometry, error) {
	if len(data) < 52 {
		return lpGeometry{}, fmt.Errorf("geometry data too short: %d bytes", len(data))
	}
	var geo lpGeometry
	geo.Magic = binary.LittleEndian.Uint32(data[0:4])
	if geo.Magic != lpGeometryMagic {
		return lpGeometry{}, fmt.Errorf("bad LP geometry magic: 0x%x (expected 0x%x)", geo.Magic, lpGeometryMagic)
	}
	geo.StructSize = binary.LittleEndian.Uint32(data[4:8])
	copy(geo.Checksum[:], data[8:40])
	geo.MetadataMaxSize = binary.LittleEndian.Uint32(data[40:44])
	geo.MetadataSlotCount = binary.LittleEndian.Uint32(data[44:48])
	geo.LogicalBlockSize = binary.LittleEndian.Uint32(data[48:52])
	return geo, nil
}

// parseLPTableDesc parses a 12-byte table descriptor.
func parseLPTableDesc(data []byte) lpTableDescriptor {
	return lpTableDescriptor{
		Offset:     binary.LittleEndian.Uint32(data[0:4]),
		NumEntries: binary.LittleEndian.Uint32(data[4:8]),
		EntrySize:  binary.LittleEndian.Uint32(data[8:12]),
	}
}

// parseLPMetadataHeader parses an LP metadata header from the given byte slice.
// Table descriptor offsets are adjusted to be relative to the start of data
// (i.e., HeaderSize is added to each raw offset so they point into the full
// metadata block).
func parseLPMetadataHeader(data []byte) (lpMetadataHeader, error) {
	if len(data) < 84+4*12 {
		return lpMetadataHeader{}, fmt.Errorf("metadata header too short: %d bytes", len(data))
	}
	var hdr lpMetadataHeader
	hdr.Magic = binary.LittleEndian.Uint32(data[0:4])
	if hdr.Magic != lpMetadataMagic {
		return lpMetadataHeader{}, fmt.Errorf("bad LP metadata magic: 0x%x (expected 0x%x)", hdr.Magic, lpMetadataMagic)
	}
	hdr.MajorVersion = binary.LittleEndian.Uint16(data[4:6])
	hdr.MinorVersion = binary.LittleEndian.Uint16(data[6:8])
	hdr.HeaderSize = binary.LittleEndian.Uint32(data[8:12])
	copy(hdr.HeaderChecksum[:], data[12:44])
	hdr.TablesSize = binary.LittleEndian.Uint32(data[44:48])
	copy(hdr.TablesChecksum[:], data[48:80])

	// Parse the four table descriptors starting at offset 80.
	hdr.Partitions = parseLPTableDesc(data[80:92])
	hdr.Extents = parseLPTableDesc(data[92:104])
	hdr.Groups = parseLPTableDesc(data[104:116])
	hdr.BlockDevices = parseLPTableDesc(data[116:128])

	// Adjust offsets so they are relative to the beginning of data (not the
	// beginning of the tables area). This lets callers index directly into
	// the full metadata blob.
	hdr.Partitions.Offset += hdr.HeaderSize
	hdr.Extents.Offset += hdr.HeaderSize
	hdr.Groups.Offset += hdr.HeaderSize
	hdr.BlockDevices.Offset += hdr.HeaderSize

	return hdr, nil
}

// parseLPPartition parses a single partition table entry.
func parseLPPartition(data []byte) (lpPartitionEntry, error) {
	if len(data) < 52 {
		return lpPartitionEntry{}, fmt.Errorf("partition entry too short: %d bytes", len(data))
	}
	var p lpPartitionEntry
	copy(p.Name[:], data[0:36])
	p.Attributes = binary.LittleEndian.Uint32(data[36:40])
	p.FirstExtent = binary.LittleEndian.Uint32(data[40:44])
	p.NumExtents = binary.LittleEndian.Uint32(data[44:48])
	p.GroupIndex = binary.LittleEndian.Uint32(data[48:52])
	return p, nil
}

// parseLPExtent parses a single extent table entry.
func parseLPExtent(data []byte) (lpExtentEntry, error) {
	if len(data) < 24 {
		return lpExtentEntry{}, fmt.Errorf("extent entry too short: %d bytes", len(data))
	}
	var e lpExtentEntry
	e.NumSectors = binary.LittleEndian.Uint64(data[0:8])
	e.TargetType = binary.LittleEndian.Uint32(data[8:12])
	e.TargetData = binary.LittleEndian.Uint64(data[12:20])
	e.TargetSource = binary.LittleEndian.Uint32(data[20:24])
	return e, nil
}

// nullTermString converts a null-terminated byte slice to a Go string,
// trimming at the first null byte.
func nullTermString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// ExtractVendorPartition opens lun0_super.bin in cacheDir/firmware/, parses
// its LP metadata, finds the vendor_a partition, and extracts it to
// cacheDir/extracted/vendor_a.img. After successful extraction, the super
// image and split 7z files are removed to reclaim disk space.
func ExtractVendorPartition(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	superPath := filepath.Join(cacheDir, "firmware", "lun0_super.bin")
	send(ProgressMsg{Text: "Opening super partition image", Percent: 0.0})

	f, err := os.Open(superPath)
	if err != nil {
		return fmt.Errorf("opening super image: %w", err)
	}
	defer f.Close()

	// Read LP geometry at offset 4096.
	send(ProgressMsg{Text: "Reading LP geometry", Percent: 0.05})
	geoBuf := make([]byte, 256)
	if _, err := f.ReadAt(geoBuf, lpGeometryOffset); err != nil {
		return fmt.Errorf("reading geometry: %w", err)
	}
	geo, err := parseLPGeometry(geoBuf)
	if err != nil {
		return fmt.Errorf("parsing geometry: %w", err)
	}

	// Read LP metadata at offset 8192.
	send(ProgressMsg{Text: "Reading LP metadata", Percent: 0.10})
	metaBuf := make([]byte, geo.MetadataMaxSize)
	if _, err := f.ReadAt(metaBuf, lpMetadataOffset); err != nil {
		return fmt.Errorf("reading metadata: %w", err)
	}
	hdr, err := parseLPMetadataHeader(metaBuf)
	if err != nil {
		return fmt.Errorf("parsing metadata header: %w", err)
	}

	// Walk partition table to find vendor_a.
	send(ProgressMsg{Text: "Searching for vendor_a partition", Percent: 0.15})
	var vendorPart *lpPartitionEntry
	for i := uint32(0); i < hdr.Partitions.NumEntries; i++ {
		off := hdr.Partitions.Offset + i*hdr.Partitions.EntrySize
		p, err := parseLPPartition(metaBuf[off:])
		if err != nil {
			return fmt.Errorf("parsing partition %d: %w", i, err)
		}
		if nullTermString(p.Name[:]) == "vendor_a" {
			vendorPart = &p
			break
		}
	}
	if vendorPart == nil {
		return fmt.Errorf("vendor_a partition not found in LP metadata")
	}

	// Create output directory and file.
	extractDir := filepath.Join(cacheDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("creating extracted directory: %w", err)
	}
	outPath := filepath.Join(extractDir, "vendor_a.img")
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating vendor_a.img: %w", err)
	}
	defer out.Close()

	// Copy each LINEAR extent to the output file.
	send(ProgressMsg{Text: "Extracting vendor_a partition", Percent: 0.20})
	totalExtents := vendorPart.NumExtents
	for i := uint32(0); i < totalExtents; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		extIdx := vendorPart.FirstExtent + i
		off := hdr.Extents.Offset + extIdx*hdr.Extents.EntrySize
		ext, err := parseLPExtent(metaBuf[off:])
		if err != nil {
			return fmt.Errorf("parsing extent %d: %w", i, err)
		}

		if ext.TargetType != lpExtentTypeLinear {
			continue
		}

		srcOffset := int64(ext.TargetData) * lpSectorSize
		length := int64(ext.NumSectors) * lpSectorSize

		sr := io.NewSectionReader(f, srcOffset, length)
		if _, err := io.Copy(out, sr); err != nil {
			return fmt.Errorf("copying extent %d: %w", i, err)
		}

		pct := 0.20 + 0.70*float64(i+1)/float64(totalExtents)
		send(ProgressMsg{
			Text:    fmt.Sprintf("Extracting vendor_a extent %d/%d", i+1, totalExtents),
			Percent: pct,
		})
	}

	// Sync and close the output file before cleanup.
	if err := out.Sync(); err != nil {
		return fmt.Errorf("syncing vendor_a.img: %w", err)
	}
	out.Close()

	// Cleanup: remove the super image and split archives.
	send(ProgressMsg{Text: "Cleaning up temporary files", Percent: 0.95})
	os.Remove(superPath)

	firmwareDir := filepath.Join(cacheDir, "firmware")
	matches, _ := filepath.Glob(filepath.Join(firmwareDir, "*.7z.*"))
	for _, m := range matches {
		os.Remove(m)
	}

	send(ProgressMsg{Text: "vendor_a extraction complete", Percent: 1.0})
	return nil
}
