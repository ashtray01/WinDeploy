package inventory

import "testing"

func TestPhysicalMemoryValidation(t *testing.T) {
	total := uint64(64) << 30
	info, err := memoryInfo(total, total/2, 50)
	if err != nil || info.TotalPhysicalBytes != total || info.AvailablePhysicalBytes != total/2 {
		t.Fatalf("%+v %v", info, err)
	}
	for _, values := range [][3]uint64{{0, 0, 0}, {100, 101, 0}, {100, 50, 101}} {
		if _, err := memoryInfo(values[0], values[1], uint32(values[2])); err == nil {
			t.Fatal("invalid memory accepted")
		}
	}
}
