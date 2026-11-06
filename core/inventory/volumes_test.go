package inventory

import (
	"context"
	"errors"
	"testing"
	"unicode/utf16"
)

func TestVolumeRootsAndPartialErrors(t *testing.T) {
	roots, err := driveRoots(utf16.Encode([]rune("C:\\\x00D:\\\x00\x00")))
	if err != nil || len(roots) != 2 || roots[1] != "D:\\" {
		t.Fatalf("%v %v", roots, err)
	}
	if _, err = driveRoots([]uint16{'C'}); err == nil {
		t.Fatal("unterminated list accepted")
	}
	calls := 0
	volumes, err := collectVolumes(context.Background(), []Volume{{Root: "Z:\\", Kind: "network"}, {Root: "C:\\", Kind: "fixed"}, {Root: "D:\\", Kind: "fixed"}}, func(root string) (Volume, error) {
		calls++
		if root == "D:\\" {
			return Volume{}, errors.New("private diagnostic")
		}
		return Volume{TotalBytes: 1 << 40, FreeBytes: 1 << 39, AvailableBytes: 1 << 38}, nil
	})
	if err == nil || len(volumes) != 3 || calls != 2 || !volumes[0].CapacityKnown || volumes[1].Issue != "read volume capacity failed" || volumes[2].CapacityKnown {
		t.Fatalf("%+v %v", volumes, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = collectVolumes(ctx, []Volume{{Root: "C:\\", Kind: "fixed"}}, func(string) (Volume, error) { t.Fatal("queried after cancellation"); return Volume{}, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
