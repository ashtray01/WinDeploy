package inventory

import "testing"

func TestCPUValidation(t *testing.T) {
	info, err := cpuInfo(" Model ", " Vendor ", 128)
	if err != nil || info.Model != "Model" || info.Vendor != "Vendor" || info.LogicalProcessors != 128 {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err = cpuInfo(" ", "", 1); err == nil {
		t.Fatal("missing model accepted")
	}
	if _, err = cpuInfo("Model", "", 0); err == nil {
		t.Fatal("missing count accepted")
	}
}
