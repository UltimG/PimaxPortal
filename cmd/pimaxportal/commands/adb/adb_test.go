package adb

import "testing"

func TestParseDevices_NoDevices(t *testing.T) {
	output := "List of devices attached\n\n"
	devs := parseDevices(output)
	if len(devs) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devs))
	}
}

func TestParseDevices_OneDevice(t *testing.T) {
	output := "List of devices attached\n3776f755\tdevice\n\n"
	devs := parseDevices(output)
	if len(devs) != 1 || devs[0] != "3776f755" {
		t.Fatalf("expected [3776f755], got %v", devs)
	}
}

func TestParseDevices_MultipleDevices(t *testing.T) {
	output := "List of devices attached\naaa\tdevice\nbbb\tdevice\n\n"
	devs := parseDevices(output)
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devs))
	}
}

func TestParseDevices_UnauthorizedSkipped(t *testing.T) {
	output := "List of devices attached\naaa\tunauthorized\nbbb\tdevice\n\n"
	devs := parseDevices(output)
	if len(devs) != 1 || devs[0] != "bbb" {
		t.Fatalf("expected [bbb], got %v", devs)
	}
}

func TestParseGLES(t *testing.T) {
	line := "GLES: Qualcomm, Adreno (TM) 650, OpenGL ES 3.2 V@0764.0 (GIT@e7c3ece80c, If78882a4bb, 1706176830) (Date:01/25/24)"
	info := parseGLES(line)
	if info.GPU != "Adreno (TM) 650" {
		t.Fatalf("expected 'Adreno (TM) 650', got '%s'", info.GPU)
	}
	if info.DriverVersion != "V@0764.0" {
		t.Fatalf("expected 'V@0764.0', got '%s'", info.DriverVersion)
	}
}
