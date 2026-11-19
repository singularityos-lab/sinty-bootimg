package bootimg

import (
	"bytes"
	"crypto/rand"
	"testing"

	devices "github.com/singularityos-lab/sinty-devices"
	"github.com/singularityos-lab/sinty-devices/profile"
)

func pixel(t *testing.T) *profile.Profile {
	ps, err := devices.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if p.ID == "google/pixel-7-pro" {
			return p
		}
	}
	t.Fatal("no Pixel 7 Pro in the catalog")
	return nil
}

func TestLZ4Legacy(t *testing.T) {
	data := make([]byte, lz4LegacyChunk+12345)
	rand.Read(data[:4096])
	z, err := LZ4Legacy(data)
	if err != nil {
		t.Fatal(err)
	}
	back, err := unLZ4Legacy(z)
	if err != nil || !bytes.Equal(back, data) {
		t.Fatalf("round trip lost data: %v", err)
	}
}

func TestAndroidFromProfile(t *testing.T) {
	dir := reference(t)
	key, err := ParseKey(read(t, dir, "testkey_rsa4096.pem"))
	if err != nil {
		t.Fatal(err)
	}
	p := pixel(t)
	b0 := read(t, dir, "cheetah-b0.dtb")
	imgs, err := Android(p.Boot, Inputs{
		Kernel: read(t, dir, "Image.lz4"), DTB: b0,
		Initramfs: read(t, dir, "initramfs.cpio.lz4"), Key: key,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"boot", "init_boot", "vendor_boot", "dtbo"} {
		same(t, name, imgs[name], read(t, dir, name+".img"))
	}
	vkb := imgs["vendor_kernel_boot"]
	if !bytes.Contains(vkb, []byte(p.Boot.Bootargs)) {
		t.Fatal("vendor_kernel_boot does not carry the profile's bootargs")
	}
	own, err := NodeString(b0, "chosen", "bootargs")
	if err != nil {
		t.Fatal(err)
	}
	if own != p.Boot.Bootargs && bytes.Contains(vkb, []byte(own)) {
		t.Fatal("the DTB's own bootargs survived")
	}
	if _, err := Android(p.Boot, Inputs{Kernel: []byte{1}, DTB: b0, Initramfs: []byte{1}}); err == nil {
		t.Fatal("AVB footers without a key must fail")
	}
	wrong, _ := SetRootCell(b0, "soc_id", 0x1234)
	if _, err := Android(p.Boot, Inputs{Kernel: []byte{1}, DTB: wrong, Initramfs: []byte{1}, Key: key}); err == nil {
		t.Fatal("a DTB built for another soc_id must fail")
	}
}
