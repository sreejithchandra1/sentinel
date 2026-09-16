package main

import "testing"

func TestDiskBytes(t *testing.T) {
	total, used, free := diskBytes(100, 25, 1024)
	if total != 102400 || free != 25600 || used != 76800 {
		t.Fatalf("got total=%d used=%d free=%d", total, used, free)
	}
	if t0, _, _ := diskBytes(0, 0, 4096); t0 != 0 {
		t.Fatal("zero blocks")
	}
	if t0, _, _ := diskBytes(^uint64(0), 1, 4096); t0 != 0 {
		t.Fatal("overflow should yield zeros")
	}
}
