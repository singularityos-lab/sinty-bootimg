package bootimg

import (
	"encoding/binary"
	"testing"
)

func TestSetNodeString(t *testing.T) {
	dtb := BoardOverlay(0x30306, 0x10000)
	for _, v := range []string{"/a/longer/path/than/before", "", "/"} {
		out, err := SetNodeString(dtb, "fragment@0", "target-path", v)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := NodeString(out, "fragment@0", "target-path"); err != nil || got != v {
			t.Fatalf("target-path = %q, %v; want %q", got, err, v)
		}
		f, err := findProp(out, "", "board_rev")
		if err != nil || binary.BigEndian.Uint32(out[f.value:]) != 0x10000 {
			t.Fatalf("board_rev lost after setting %q: %v", v, err)
		}
		if int(binary.BigEndian.Uint32(out[4:])) != len(out) {
			t.Fatalf("totalsize %d, blob %d", binary.BigEndian.Uint32(out[4:]), len(out))
		}
	}
	if _, err := SetNodeString(dtb, "chosen", "bootargs", "x"); err == nil {
		t.Fatal("a missing node must fail")
	}
}

func TestSetRootCell(t *testing.T) {
	out, err := SetRootCell(BoardOverlay(1, 2), "board_id", 7)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := findProp(out, "", "board_id")
	if binary.BigEndian.Uint32(out[f.value:]) != 7 {
		t.Fatal("board_id not set")
	}
}
