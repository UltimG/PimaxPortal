package commands

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
)

const (
	fdtMagic      = 0xD00DFEED
	fdtBeginNode  = 0x00000001
	fdtEndNode    = 0x00000002
	fdtProp       = 0x00000003
	fdtNop        = 0x00000004
	fdtEnd        = 0x00000009
	bootPartition = "boot_a"
)

// OCPreset defines a GPU overclock preset with target frequency and voltage.
type OCPreset struct {
	Name    string
	FreqHz  uint32
	Voltage uint32 // RPMH voltage level for opp-microvolt
}

// Presets is the list of available GPU frequency presets for the Adreno 650.
var Presets = []OCPreset{
	{Name: "855 MHz (Stock)", FreqHz: 855000000, Voltage: 320},
	{Name: "905 MHz", FreqHz: 905000000, Voltage: 320},
	{Name: "940 MHz", FreqHz: 940000000, Voltage: 416},
	{Name: "985 MHz", FreqHz: 985000000, Voltage: 416},
}

// DTBInfo holds the offset and size of a single DTB within a concatenated blob.
type DTBInfo struct {
	Offset uint32
	Size   uint32
}

// fdtHeader holds parsed FDT header fields.
type fdtHeader struct {
	offDtStruct  uint32
	offDtStrings uint32
	sizeDtStruct uint32
	sizeDtStrings uint32
}

// ScanDTBs scans concatenated FDT blobs by looking for the magic 0xD00DFEED
// and reading totalsize (big-endian uint32 at offset 4).
func ScanDTBs(data []byte) []DTBInfo {
	var dtbs []DTBInfo
	pos := 0
	for pos+8 <= len(data) {
		magic := binary.BigEndian.Uint32(data[pos : pos+4])
		if magic != fdtMagic {
			pos += 4
			continue
		}
		totalSize := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		if totalSize < 40 || pos+int(totalSize) > len(data) {
			pos += 4
			continue
		}
		dtbs = append(dtbs, DTBInfo{
			Offset: uint32(pos),
			Size:   totalSize,
		})
		pos += int(totalSize)
	}
	return dtbs
}

// findStringOffset finds a null-terminated property name in the FDT strings
// block. Returns byte offset or -1. Checks that the match starts at a string
// boundary (preceded by \x00 or at offset 0).
func findStringOffset(strBlock []byte, name string) int {
	needle := []byte(name)
	for i := 0; i+len(needle) <= len(strBlock); i++ {
		// Check string boundary: must be at offset 0 or preceded by null byte
		if i > 0 && strBlock[i-1] != 0 {
			continue
		}
		// Check the name matches
		match := true
		for j := 0; j < len(needle); j++ {
			if strBlock[i+j] != needle[j] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		// Must be followed by null terminator
		endIdx := i + len(needle)
		if endIdx < len(strBlock) && strBlock[endIdx] == 0 {
			return i
		}
		// Edge case: name ends exactly at block end (shouldn't happen in valid FDT)
	}
	return -1
}

// parseFDTHeader parses FDT header fields from a DTB blob.
func parseFDTHeader(dtb []byte) (fdtHeader, error) {
	if len(dtb) < 40 {
		return fdtHeader{}, fmt.Errorf("DTB too short: %d bytes", len(dtb))
	}
	magic := binary.BigEndian.Uint32(dtb[0:4])
	if magic != fdtMagic {
		return fdtHeader{}, fmt.Errorf("bad FDT magic: 0x%08X", magic)
	}
	return fdtHeader{
		offDtStruct:   binary.BigEndian.Uint32(dtb[8:12]),
		offDtStrings:  binary.BigEndian.Uint32(dtb[12:16]),
		sizeDtStrings: binary.BigEndian.Uint32(dtb[32:36]),
		sizeDtStruct:  binary.BigEndian.Uint32(dtb[36:40]),
	}, nil
}

// align4 rounds up to the next 4-byte boundary.
func align4(n int) int {
	return (n + 3) &^ 3
}

// FindGPUFreqOffset walks the FDT struct block looking for a FDT_PROP where
// nameoff matches "qcom,gpu-freq" in the strings block and the 4-byte BE
// value equals targetHz. Returns the byte offset of the value within dtb,
// or -1 if not found.
func FindGPUFreqOffset(dtb []byte, targetHz uint32) int {
	hdr, err := parseFDTHeader(dtb)
	if err != nil {
		return -1
	}

	strBlock := dtb[hdr.offDtStrings : hdr.offDtStrings+hdr.sizeDtStrings]
	gpuFreqNameOff := findStringOffset(strBlock, "qcom,gpu-freq")
	if gpuFreqNameOff < 0 {
		return -1
	}

	structStart := int(hdr.offDtStruct)
	structEnd := structStart + int(hdr.sizeDtStruct)
	pos := structStart

	for pos+4 <= structEnd {
		token := binary.BigEndian.Uint32(dtb[pos : pos+4])

		switch token {
		case fdtBeginNode:
			pos += 4
			// Skip null-terminated node name, padded to 4-byte alignment
			nameStart := pos
			for pos < structEnd && dtb[pos] != 0 {
				pos++
			}
			pos++ // skip null terminator
			pos = structStart + align4(pos-structStart)
			_ = nameStart

		case fdtEndNode:
			pos += 4

		case fdtProp:
			if pos+12 > structEnd {
				return -1
			}
			propLen := binary.BigEndian.Uint32(dtb[pos+4 : pos+8])
			nameoff := binary.BigEndian.Uint32(dtb[pos+8 : pos+12])
			valStart := pos + 12

			if int(nameoff) == gpuFreqNameOff && propLen == 4 {
				if valStart+4 <= structEnd {
					val := binary.BigEndian.Uint32(dtb[valStart : valStart+4])
					if val == targetHz {
						return valStart
					}
				}
			}

			pos = structStart + align4(valStart+int(propLen)-structStart)

		case fdtNop:
			pos += 4

		case fdtEnd:
			return -1

		default:
			return -1
		}
	}
	return -1
}

// FindOPPOffsets walks the FDT struct block looking for an opp-hz property
// (8-byte BE uint64) matching targetHz and an opp-microvolt property (4-byte
// BE uint32) within the same node. Returns byte offsets within dtb, or -1.
func FindOPPOffsets(dtb []byte, targetHz uint32) (hzOff int, mvOff int) {
	hzOff = -1
	mvOff = -1

	hdr, err := parseFDTHeader(dtb)
	if err != nil {
		return
	}

	strBlock := dtb[hdr.offDtStrings : hdr.offDtStrings+hdr.sizeDtStrings]
	oppHzNameOff := findStringOffset(strBlock, "opp-hz")
	oppMvNameOff := findStringOffset(strBlock, "opp-microvolt")
	if oppHzNameOff < 0 || oppMvNameOff < 0 {
		return
	}

	structStart := int(hdr.offDtStruct)
	structEnd := structStart + int(hdr.sizeDtStruct)
	pos := structStart

	depth := 0
	nodeHzOff := -1
	nodeMvOff := -1
	nodeDepth := -1

	for pos+4 <= structEnd {
		token := binary.BigEndian.Uint32(dtb[pos : pos+4])

		switch token {
		case fdtBeginNode:
			depth++
			// If we're entering a new node at the same or lesser depth, reset
			if nodeDepth >= 0 && depth <= nodeDepth {
				// Check if we found both in previous node
				if nodeHzOff >= 0 && nodeMvOff >= 0 {
					return nodeHzOff, nodeMvOff
				}
				nodeHzOff = -1
				nodeMvOff = -1
				nodeDepth = -1
			}

			pos += 4
			for pos < structEnd && dtb[pos] != 0 {
				pos++
			}
			pos++
			pos = structStart + align4(pos-structStart)

		case fdtEndNode:
			if depth == nodeDepth {
				// Leaving the node where we found properties
				if nodeHzOff >= 0 && nodeMvOff >= 0 {
					return nodeHzOff, nodeMvOff
				}
				nodeHzOff = -1
				nodeMvOff = -1
				nodeDepth = -1
			}
			depth--
			pos += 4

		case fdtProp:
			if pos+12 > structEnd {
				return
			}
			propLen := binary.BigEndian.Uint32(dtb[pos+4 : pos+8])
			nameoff := binary.BigEndian.Uint32(dtb[pos+8 : pos+12])
			valStart := pos + 12

			if int(nameoff) == oppHzNameOff && propLen == 8 {
				if valStart+8 <= structEnd {
					hi := binary.BigEndian.Uint32(dtb[valStart : valStart+4])
					lo := binary.BigEndian.Uint32(dtb[valStart+4 : valStart+8])
					if hi == 0 && lo == targetHz {
						nodeHzOff = valStart
						nodeDepth = depth
					}
				}
			}

			if int(nameoff) == oppMvNameOff && propLen == 4 {
				if valStart+4 <= structEnd {
					if nodeDepth == depth {
						nodeMvOff = valStart
					}
				}
			}

			pos = structStart + align4(valStart+int(propLen)-structStart)

		case fdtNop:
			pos += 4

		case fdtEnd:
			if nodeHzOff >= 0 && nodeMvOff >= 0 {
				return nodeHzOff, nodeMvOff
			}
			return -1, -1

		default:
			return -1, -1
		}
	}

	if nodeHzOff >= 0 && nodeMvOff >= 0 {
		return nodeHzOff, nodeMvOff
	}
	return -1, -1
}

// PatchDTB copies the DTB, finds and patches gpu-freq (4 bytes), opp-hz
// (8 bytes), and opp-microvolt (4 bytes). Returns error if currentFreqHz
// is not found.
func PatchDTB(dtb []byte, currentFreqHz uint32, preset OCPreset) ([]byte, error) {
	gpuOff := FindGPUFreqOffset(dtb, currentFreqHz)
	if gpuOff < 0 {
		return nil, fmt.Errorf("gpu-freq %d Hz not found in DTB", currentFreqHz)
	}

	hzOff, mvOff := FindOPPOffsets(dtb, currentFreqHz)
	if hzOff < 0 || mvOff < 0 {
		return nil, fmt.Errorf("opp-hz/opp-microvolt for %d Hz not found in DTB", currentFreqHz)
	}

	patched := make([]byte, len(dtb))
	copy(patched, dtb)

	// Patch qcom,gpu-freq (4-byte big-endian)
	binary.BigEndian.PutUint32(patched[gpuOff:gpuOff+4], preset.FreqHz)

	// Patch opp-hz (8-byte big-endian uint64)
	binary.BigEndian.PutUint32(patched[hzOff:hzOff+4], 0)
	binary.BigEndian.PutUint32(patched[hzOff+4:hzOff+8], preset.FreqHz)

	// Patch opp-microvolt (4-byte big-endian)
	binary.BigEndian.PutUint32(patched[mvOff:mvOff+4], preset.Voltage)

	return patched, nil
}

// PatchBootImage parses a boot image, scans its DTB section, and patches each
// DTB that contains the target frequency. Returns the count of patched DTBs.
// DTBs without the target freq (e.g., kona v1) are skipped.
func PatchBootImage(data []byte, currentFreqHz uint32, preset OCPreset) (int, error) {
	hdr, err := ParseBootImgHeader(data)
	if err != nil {
		return 0, fmt.Errorf("parsing boot header: %w", err)
	}

	dtbOffset := DTBSectionOffset(hdr)
	if dtbOffset+int64(hdr.DTBSize) > int64(len(data)) {
		return 0, fmt.Errorf("DTB section extends beyond image (offset=%d, size=%d, image=%d)",
			dtbOffset, hdr.DTBSize, len(data))
	}

	dtbSection := data[dtbOffset : dtbOffset+int64(hdr.DTBSize)]
	dtbs := ScanDTBs(dtbSection)
	if len(dtbs) == 0 {
		return 0, fmt.Errorf("no DTBs found in boot image")
	}

	patched := 0
	for _, di := range dtbs {
		dtb := data[dtbOffset+int64(di.Offset) : dtbOffset+int64(di.Offset)+int64(di.Size)]

		// Check if this DTB has the target frequency
		if FindGPUFreqOffset(dtb, currentFreqHz) < 0 {
			continue // kona v1 or other variant — skip
		}

		patchedDTB, err := PatchDTB(dtb, currentFreqHz, preset)
		if err != nil {
			return patched, fmt.Errorf("patching DTB at offset %d: %w", di.Offset, err)
		}

		// Write patched DTB back into data in-place
		copy(data[dtbOffset+int64(di.Offset):], patchedDTB)
		patched++
	}

	return patched, nil
}

// ReadCurrentGPUFreq reads the current max GPU clock frequency from sysfs.
func ReadCurrentGPUFreq() (uint32, error) {
	out, err := adb.ShellSu(`cat /sys/class/kgsl/kgsl-3d0/max_gpuclk`)
	if err != nil {
		return 0, fmt.Errorf("reading max_gpuclk: %w", err)
	}
	val, err := strconv.ParseUint(strings.TrimSpace(out), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parsing max_gpuclk %q: %w", out, err)
	}
	return uint32(val), nil
}

// bootPartitionPath returns the block device path for boot_a.
func bootPartitionPath() string {
	return "/dev/block/by-name/" + bootPartition
}

// RunOverclock executes the full overclock pipeline: read current freq,
// backup boot, patch DTB, flash patched image.
func RunOverclock(ctx context.Context, preset OCPreset, send func(ProgressMsg)) error {
	send(ProgressMsg{Text: "Reading current GPU frequency...", Percent: 0})

	currentFreq, err := ReadCurrentGPUFreq()
	if err != nil {
		return fmt.Errorf("reading GPU freq: %w", err)
	}

	if currentFreq == preset.FreqHz {
		send(ProgressMsg{Text: fmt.Sprintf("GPU already at %s", preset.Name), Percent: 1.0})
		return nil
	}

	send(ProgressMsg{Text: "Backing up boot partition...", Percent: 0.05})

	_, err = adb.ShellSu(fmt.Sprintf("dd if=%s of=/sdcard/boot_backup.img", bootPartitionPath()))
	if err != nil {
		return fmt.Errorf("backing up boot: %w", err)
	}

	send(ProgressMsg{Text: "Pulling boot image...", Percent: 0.15})

	cacheDir, err := CacheDir()
	if err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}
	localBoot := filepath.Join(cacheDir, "boot_overclock.img")

	if err := adb.Pull("/sdcard/boot_backup.img", localBoot); err != nil {
		return fmt.Errorf("pulling boot image: %w", err)
	}

	send(ProgressMsg{Text: "Patching DTBs...", Percent: 0.35})

	data, err := os.ReadFile(localBoot)
	if err != nil {
		return fmt.Errorf("reading boot image: %w", err)
	}

	n, err := PatchBootImage(data, currentFreq, preset)
	if err != nil {
		return fmt.Errorf("patching boot image: %w", err)
	}
	send(ProgressMsg{Text: fmt.Sprintf("Patched %d DTBs", n), Percent: 0.5})

	patchedPath := filepath.Join(cacheDir, "boot_patched.img")
	if err := os.WriteFile(patchedPath, data, 0644); err != nil {
		return fmt.Errorf("writing patched image: %w", err)
	}

	send(ProgressMsg{Text: "Pushing patched boot image...", Percent: 0.6})

	remotePatchedPath := "/sdcard/boot_patched.img"
	if err := adb.Push(patchedPath, remotePatchedPath); err != nil {
		return fmt.Errorf("pushing patched image: %w", err)
	}

	send(ProgressMsg{Text: "Flashing patched boot...", Percent: 0.75})

	_, err = adb.ShellSu(fmt.Sprintf("dd if=%s of=%s", remotePatchedPath, bootPartitionPath()))
	if err != nil {
		return fmt.Errorf("flashing patched boot: %w", err)
	}

	send(ProgressMsg{Text: "Cleaning up...", Percent: 0.9})

	_, _ = adb.ShellSu(fmt.Sprintf("rm %s", remotePatchedPath))

	send(ProgressMsg{Text: fmt.Sprintf("GPU overclocked to %s — reboot to apply", preset.Name), Percent: 1.0})
	return nil
}

// RestoreStock reads the current boot partition, patches it back to stock
// 855 MHz, and flashes it.
func RestoreStock(ctx context.Context, send func(ProgressMsg)) error {
	stockPreset := Presets[0] // 855 MHz (Stock)

	send(ProgressMsg{Text: "Reading current GPU frequency", Percent: 0})
	currentFreq, err := ReadCurrentGPUFreq()
	if err != nil {
		return fmt.Errorf("reading GPU freq: %w", err)
	}
	if currentFreq == stockPreset.FreqHz {
		send(ProgressMsg{Text: "GPU already at stock 855 MHz", Percent: 1.0})
		return nil
	}

	// Use the same pipeline as overclock: pull, patch to stock, push, flash
	return RunOverclock(ctx, stockPreset, send)
}
