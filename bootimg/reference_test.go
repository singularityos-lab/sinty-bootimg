package bootimg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func reference(t *testing.T) string {
	dir := os.Getenv("SINTY_BOOTIMG_REFERENCE")
	if dir == "" {
		t.Skip("SINTY_BOOTIMG_REFERENCE names no directory of images made by AOSP's tools")
	}
	return dir
}

func read(t *testing.T, dir, name string) []byte {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func same(t *testing.T, name string, got, want []byte) {
	t.Helper()
	if bytes.Equal(got, want) {
		return
	}
	n := min(len(got), len(want))
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			t.Fatalf("%s: first difference at byte %d (%#x), sizes %d and %d", name, i, i, len(got), len(want))
		}
	}
	t.Fatalf("%s: sizes differ, %d and %d", name, len(got), len(want))
}

func TestMatchesAOSPTools(t *testing.T) {
	dir := reference(t)
	key, err := ParseKey(read(t, dir, "testkey_rsa4096.pem"))
	if err != nil {
		t.Fatal(err)
	}
	ver, err := OSVersion("17.0.0", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	empty := read(t, dir, "empty.cpio")
	footer := func(name string, size int64, img []byte) []byte {
		out, err := AddHashFooter(img, HashFooter{
			Partition: name, PartitionSize: size, Salt: SaltOf(img), RollbackIndex: 1783209600, Key: key,
			Props: []Prop{
				{"com.android.build." + name + ".os_version", "17"},
				{"com.android.build." + name + ".security_patch", "2026-07-05"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	boot, err := Boot{Kernel: read(t, dir, "Image.lz4")}.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "boot.img", footer("boot", 0x4000000, boot), read(t, dir, "boot.img"))

	initBoot, err := Boot{Ramdisk: empty, OSVersion: ver}.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "init_boot.img", footer("init_boot", 0x800000, initBoot), read(t, dir, "init_boot.img"))

	vendor, err := Vendor{
		PageSize: 0x800,
		Cmdline:  "console=tty0 earlycon=exynos4210,0x10A00000 clk_ignore_unused pd_ignore_unused printk.devkmsg=on bootconfig",
		Ramdisks: []VendorRamdisk{{Data: empty, Type: RamdiskPlatform}},
	}.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "vendor_boot.img", vendor, read(t, dir, "vendor_boot.img"))

	vkb, err := Vendor{
		PageSize: 0x800,
		DTB:      read(t, dir, "dtb.img"),
		Ramdisks: []VendorRamdisk{{Data: read(t, dir, "initramfs.cpio.lz4"), Type: RamdiskPlatform}},
	}.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	same(t, "vendor_kernel_boot.img", vkb, read(t, dir, "vendor_kernel_boot.img"))

	same(t, "cheetah-m1-overlay.dtbo", BoardOverlay(0x30306, 0x10000), read(t, dir, "cheetah-m1-overlay.dtbo"))
	dtbo := DTBO([]Overlay{{DTB: BoardOverlay(0x30306, 0x10000), ID: 0x30306, Rev: 0x10000}}, 4096)
	same(t, "dtbo.img", dtbo, read(t, dir, "dtbo.img"))

	b0 := read(t, dir, "cheetah-b0.dtb")
	bootargs, err := NodeString(b0, "chosen", "bootargs")
	if err != nil || bootargs == "" {
		t.Fatalf("bootargs %q: %v", bootargs, err)
	}
	same(t, "bootargs round trip", mustSet(t, b0, bootargs), b0)
	a0, err := SetRootCell(b0, "soc_id", 0x9855000)
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	for i := range b0 {
		if a0[i] != b0[i] {
			changed++
		}
	}
	if len(a0) != len(b0) || changed != 1 {
		t.Fatalf("soc_id rewrite touched %d bytes, sizes %d and %d", changed, len(a0), len(b0))
	}
}

func mustSet(t *testing.T, dtb []byte, bootargs string) []byte {
	out, err := SetNodeString(dtb, "chosen", "bootargs", bootargs)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
