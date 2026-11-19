package bootimg

import (
	"os"
	"testing"
)

func TestFromUKI(t *testing.T) {
	path, compat := os.Getenv("SINTY_BOOTIMG_UKI"), os.Getenv("SINTY_BOOTIMG_UKI_COMPATIBLE")
	if path == "" || compat == "" {
		t.Skip("SINTY_BOOTIMG_UKI and SINTY_BOOTIMG_UKI_COMPATIBLE name no unified kernel image")
	}
	uki, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	kernel, initramfs, dtb, err := FromUKI(uki, compat)
	if err != nil {
		t.Fatal(err)
	}
	if string(kernel[56:60]) != "ARM\x64" {
		t.Fatalf("the kernel is not an arm64 Image")
	}
	if len(initramfs) < 1024 {
		t.Fatalf("initramfs is %d bytes", len(initramfs))
	}
	if _, err := NodeString(dtb, "", "model"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := FromUKI(uki, "vendor,no-such-board"); err == nil {
		t.Fatal("an unknown board must fail")
	}
	if _, _, _, err := FromUKI([]byte("MZ not really"), compat); err == nil {
		t.Fatal("a non-PE file must fail")
	}
}
