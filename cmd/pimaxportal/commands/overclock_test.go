package commands

import (
	"encoding/binary"
	"os"
	"testing"
)

// buildStringsBlock constructs a DTB strings block with null-terminated
// property names. Returns the block and the offset of each name.
func buildStringsBlock() ([]byte, int, int, int) {
	// "qcom,gpu-freq\x00opp-hz\x00opp-microvolt\x00"
	s := "qcom,gpu-freq\x00opp-hz\x00opp-microvolt\x00"
	gpuFreqOff := 0
	oppHzOff := 14  // len("qcom,gpu-freq") + 1
	oppMvOff := 21  // oppHzOff + len("opp-hz") + 1
	return []byte(s), gpuFreqOff, oppHzOff, oppMvOff
}

// buildMinimalDTB constructs a valid FDT with a root node containing
// a "qcom,gpu-pwrlevel@0" child (with qcom,gpu-freq property) and an
// "opp-855000000" child (with opp-hz and opp-microvolt properties).
func buildMinimalDTB(freqHz uint32, voltage uint32) []byte {
	strBlock, gpuFreqOff, oppHzOff, oppMvOff := buildStringsBlock()

	// Build the struct block
	var st []byte

	// Helper: append big-endian uint32
	appendU32 := func(v uint32) {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		st = append(st, b...)
	}

	// Helper: append null-terminated string with padding to 4-byte alignment
	appendStr := func(s string) {
		b := append([]byte(s), 0)
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		st = append(st, b...)
	}

	// Root node: FDT_BEGIN_NODE + empty name
	appendU32(fdtBeginNode)
	appendStr("") // root has empty name

	// Child node: qcom,gpu-pwrlevel@0
	appendU32(fdtBeginNode)
	appendStr("qcom,gpu-pwrlevel@0")

	// Property: qcom,gpu-freq = freqHz (4 bytes, big-endian)
	appendU32(fdtProp)
	appendU32(4)                    // len
	appendU32(uint32(gpuFreqOff))   // nameoff
	appendU32(freqHz)               // value

	appendU32(fdtEndNode) // end qcom,gpu-pwrlevel@0

	// Child node: opp-855000000
	appendU32(fdtBeginNode)
	appendStr("opp-855000000")

	// Property: opp-hz = freqHz as uint64 (8 bytes, big-endian)
	appendU32(fdtProp)
	appendU32(8)                  // len
	appendU32(uint32(oppHzOff))   // nameoff
	appendU32(0)                  // high 32 bits of uint64
	appendU32(freqHz)             // low 32 bits of uint64

	// Property: opp-microvolt = voltage (4 bytes, big-endian)
	appendU32(fdtProp)
	appendU32(4)                  // len
	appendU32(uint32(oppMvOff))   // nameoff
	appendU32(voltage)            // value

	appendU32(fdtEndNode) // end opp-855000000

	appendU32(fdtEndNode) // end root
	appendU32(fdtEnd)     // FDT_END

	// Build header (40 bytes)
	// Header layout:
	//   0:  magic (0xD00DFEED)
	//   4:  totalsize
	//   8:  off_dt_struct
	//  12:  off_dt_strings
	//  16:  off_mem_rsvmap
	//  20:  version
	//  24:  last_comp_version
	//  28:  boot_cpuid_phys
	//  32:  size_dt_strings
	//  36:  size_dt_struct
	headerSize := 40
	// Memory reservation block: just one entry of 16 zero bytes (terminator)
	memRsvMap := make([]byte, 16)
	offMemRsvMap := headerSize
	offDtStruct := headerSize + len(memRsvMap)
	offDtStrings := offDtStruct + len(st)
	totalSize := offDtStrings + len(strBlock)

	hdr := make([]byte, headerSize)
	binary.BigEndian.PutUint32(hdr[0:4], fdtMagic)
	binary.BigEndian.PutUint32(hdr[4:8], uint32(totalSize))
	binary.BigEndian.PutUint32(hdr[8:12], uint32(offDtStruct))
	binary.BigEndian.PutUint32(hdr[12:16], uint32(offDtStrings))
	binary.BigEndian.PutUint32(hdr[16:20], uint32(offMemRsvMap))
	binary.BigEndian.PutUint32(hdr[20:24], 17) // version
	binary.BigEndian.PutUint32(hdr[24:28], 16) // last_comp_version
	binary.BigEndian.PutUint32(hdr[28:32], 0)  // boot_cpuid_phys
	binary.BigEndian.PutUint32(hdr[32:36], uint32(len(strBlock)))
	binary.BigEndian.PutUint32(hdr[36:40], uint32(len(st)))

	dtb := make([]byte, 0, totalSize)
	dtb = append(dtb, hdr...)
	dtb = append(dtb, memRsvMap...)
	dtb = append(dtb, st...)
	dtb = append(dtb, strBlock...)
	return dtb
}

func TestFindStringOffset(t *testing.T) {
	strBlock, _, _, _ := buildStringsBlock()

	tests := []struct {
		name   string
		want   int
	}{
		{"qcom,gpu-freq", 0},
		{"opp-hz", 14},
		{"opp-microvolt", 21},
		{"nonexistent", -1},
	}

	for _, tt := range tests {
		got := findStringOffset(strBlock, tt.name)
		if got != tt.want {
			t.Errorf("findStringOffset(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestFindStringOffset_Substring(t *testing.T) {
	// Ensure we don't match a substring in the middle of another string.
	// "gpu-freq" should NOT match at offset 5 within "qcom,gpu-freq"
	strBlock, _, _, _ := buildStringsBlock()
	got := findStringOffset(strBlock, "gpu-freq")
	if got != -1 {
		t.Errorf("findStringOffset(%q) = %d, want -1 (should not match substring)", "gpu-freq", got)
	}
}

func TestScanDTBs(t *testing.T) {
	dtb1 := buildMinimalDTB(855000000, 320)
	dtb2 := buildMinimalDTB(587000000, 256)

	combined := make([]byte, 0, len(dtb1)+len(dtb2))
	combined = append(combined, dtb1...)
	combined = append(combined, dtb2...)

	dtbs := ScanDTBs(combined)
	if len(dtbs) != 2 {
		t.Fatalf("ScanDTBs found %d DTBs, want 2", len(dtbs))
	}
	if dtbs[0].Offset != 0 {
		t.Errorf("DTB[0].Offset = %d, want 0", dtbs[0].Offset)
	}
	if dtbs[0].Size != uint32(len(dtb1)) {
		t.Errorf("DTB[0].Size = %d, want %d", dtbs[0].Size, len(dtb1))
	}
	if dtbs[1].Offset != uint32(len(dtb1)) {
		t.Errorf("DTB[1].Offset = %d, want %d", dtbs[1].Offset, len(dtb1))
	}
	if dtbs[1].Size != uint32(len(dtb2)) {
		t.Errorf("DTB[1].Size = %d, want %d", dtbs[1].Size, len(dtb2))
	}
}

func TestFindGPUFreqOffset(t *testing.T) {
	dtb := buildMinimalDTB(855000000, 320)
	off := FindGPUFreqOffset(dtb, 855000000)
	if off < 0 {
		t.Fatal("FindGPUFreqOffset returned -1, expected valid offset")
	}
	// Verify the value at that offset is 855000000 big-endian
	val := binary.BigEndian.Uint32(dtb[off : off+4])
	if val != 855000000 {
		t.Fatalf("value at offset %d = %d, want 855000000", off, val)
	}
}

func TestFindGPUFreqOffset_NotFound(t *testing.T) {
	dtb := buildMinimalDTB(855000000, 320)
	off := FindGPUFreqOffset(dtb, 480000000) // freq not in DTB
	if off != -1 {
		t.Fatalf("FindGPUFreqOffset returned %d, expected -1 for non-matching freq", off)
	}
}

func TestFindOPPOffsets(t *testing.T) {
	dtb := buildMinimalDTB(855000000, 320)
	hzOff, mvOff := FindOPPOffsets(dtb, 855000000)
	if hzOff < 0 {
		t.Fatal("FindOPPOffsets hzOff returned -1")
	}
	if mvOff < 0 {
		t.Fatal("FindOPPOffsets mvOff returned -1")
	}

	// Verify opp-hz value: 8 bytes, high=0, low=855000000
	hi := binary.BigEndian.Uint32(dtb[hzOff : hzOff+4])
	lo := binary.BigEndian.Uint32(dtb[hzOff+4 : hzOff+8])
	if hi != 0 || lo != 855000000 {
		t.Fatalf("opp-hz at offset %d = 0x%08X%08X, want 855000000", hzOff, hi, lo)
	}

	// Verify opp-microvolt value
	val := binary.BigEndian.Uint32(dtb[mvOff : mvOff+4])
	if val != 320 {
		t.Fatalf("opp-microvolt at offset %d = %d, want 320", mvOff, val)
	}
}

func TestPatchDTB(t *testing.T) {
	dtb := buildMinimalDTB(855000000, 320)
	preset := OCPreset{Name: "940 MHz", FreqHz: 940000000, Voltage: 416}

	patched, err := PatchDTB(dtb, 855000000, preset)
	if err != nil {
		t.Fatalf("PatchDTB: %v", err)
	}

	// Original should be unchanged
	gpuOff := FindGPUFreqOffset(dtb, 855000000)
	if gpuOff < 0 {
		t.Fatal("original DTB was modified")
	}

	// Verify patched values
	newGPUOff := FindGPUFreqOffset(patched, 940000000)
	if newGPUOff < 0 {
		t.Fatal("patched DTB does not contain new gpu-freq")
	}

	hzOff, mvOff := FindOPPOffsets(patched, 940000000)
	if hzOff < 0 {
		t.Fatal("patched DTB does not contain new opp-hz")
	}
	if mvOff < 0 {
		t.Fatal("patched DTB does not contain new opp-microvolt")
	}

	mv := binary.BigEndian.Uint32(patched[mvOff : mvOff+4])
	if mv != 416 {
		t.Fatalf("patched opp-microvolt = %d, want 416", mv)
	}
}

func TestPatchDTB_SkipsNonMatching(t *testing.T) {
	dtb := buildMinimalDTB(480000000, 256) // DTB with 480 MHz
	preset := OCPreset{Name: "940 MHz", FreqHz: 940000000, Voltage: 416}

	_, err := PatchDTB(dtb, 855000000, preset) // looking for 855 MHz
	if err == nil {
		t.Fatal("PatchDTB should return error when currentFreqHz not found")
	}
}

func TestPatchBootImage_RealImage(t *testing.T) {
	const imgPath = "originals/boot_a_stock.img"

	// We need to resolve relative to the project root.
	// The tests run from cmd/pimaxportal/commands/, so walk up.
	projectRoot := "../../../"
	fullPath := projectRoot + imgPath

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Skipf("skipping: %s not available", imgPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("reading boot image: %v", err)
	}

	preset := OCPreset{Name: "940 MHz", FreqHz: 940000000, Voltage: 416}
	n, err := PatchBootImage(data, 855000000, preset)
	if err != nil {
		t.Fatalf("PatchBootImage: %v", err)
	}

	// The real Pimax boot image should have 5 DTBs with 855 MHz
	// (kona v2) and 2 without (kona v1) = 7 total, 5 patched.
	if n != 5 {
		t.Fatalf("PatchBootImage patched %d DTBs, want 5", n)
	}
}
