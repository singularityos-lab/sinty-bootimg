package bootimg

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"slices"

	"github.com/singularityos-lab/sinty-devices/profile"
)

type Inputs struct {
	Kernel    []byte
	DTB       []byte
	Initramfs []byte
	Key       *rsa.PrivateKey
}

// Android builds the boot images a profile's android-v4 boot section lists,
// keyed by image name.
func Android(b *profile.Boot, in Inputs) (map[string][]byte, error) {
	if b == nil || b.Format != "android-v4" {
		return nil, errors.New("the profile has no android-v4 boot section")
	}
	if len(in.Kernel) == 0 || len(in.DTB) == 0 || len(in.Initramfs) == 0 {
		return nil, errors.New("kernel, dtb and initramfs are all required")
	}
	has := func(name string) bool { return slices.Contains(b.Images, name) }
	kernel := in.Kernel
	if !IsLZ4Legacy(kernel) {
		var err error
		if kernel, err = LZ4Legacy(kernel); err != nil {
			return nil, fmt.Errorf("compressing the kernel: %w", err)
		}
	}
	dtb := in.DTB
	if b.Bootargs != "" {
		var err error
		if dtb, err = SetNodeString(dtb, "chosen", "bootargs", b.Bootargs); err != nil {
			return nil, err
		}
	}
	var dtbs []byte
	if len(b.SocIDs) == 0 {
		dtbs = dtb
	}
	for i, s := range b.SocIDs {
		id, err := profile.ParseHex32(s.ID)
		if err != nil {
			return nil, err
		}
		copyFor := dtb
		if i > 0 {
			if copyFor, err = SetRootCell(dtb, "soc_id", id); err != nil {
				return nil, fmt.Errorf("soc id %s: %w", s.Name, err)
			}
		} else if err := checkRootCell(dtb, "soc_id", id); err != nil {
			return nil, fmt.Errorf("soc id %s: %w", s.Name, err)
		}
		dtbs = append(dtbs, copyFor...)
	}
	ver, err := OSVersion(b.OSVersion, b.OSPatchLevel)
	if err != nil {
		return nil, err
	}
	empty := emptyCpio()
	out := map[string][]byte{}
	if has("init_boot") {
		if out["boot"], err = (Boot{Kernel: kernel}).Bytes(); err != nil {
			return nil, err
		}
		if out["init_boot"], err = (Boot{Ramdisk: empty, OSVersion: ver}).Bytes(); err != nil {
			return nil, err
		}
	} else if out["boot"], err = (Boot{Kernel: kernel, Ramdisk: empty, OSVersion: ver}).Bytes(); err != nil {
		return nil, err
	}
	vendor := Vendor{PageSize: b.VendorPageSize, Cmdline: b.VendorCmdline}
	payload := Vendor{PageSize: b.VendorPageSize, DTB: dtbs,
		Ramdisks: []VendorRamdisk{{Data: in.Initramfs, Type: RamdiskPlatform}}}
	if has("vendor_kernel_boot") {
		vendor.Ramdisks = []VendorRamdisk{{Data: empty, Type: RamdiskPlatform}}
		if out["vendor_kernel_boot"], err = payload.Bytes(); err != nil {
			return nil, err
		}
	} else {
		vendor.DTB, vendor.Ramdisks = payload.DTB, payload.Ramdisks
	}
	if out["vendor_boot"], err = vendor.Bytes(); err != nil {
		return nil, err
	}
	if b.DTBO != nil {
		id, _ := profile.ParseHex32(b.DTBO.BoardID)
		rev, _ := profile.ParseHex32(b.DTBO.BoardRev)
		out["dtbo"] = DTBO([]Overlay{{DTB: BoardOverlay(id, rev), ID: id, Rev: rev}}, uint32(b.DTBO.PageSize))
	}
	if b.AVB != nil {
		if in.Key == nil {
			return nil, errors.New("the profile asks for AVB footers and no key was given")
		}
		major := b.OSVersion
		if i := slices.Index([]byte(major), '.'); i >= 0 {
			major = major[:i]
		}
		for _, f := range b.AVB.Footers {
			size, _ := profile.ParseHex32(f.Size)
			img := out[f.Partition]
			if out[f.Partition], err = AddHashFooter(img, HashFooter{
				Partition: f.Partition, PartitionSize: int64(size), Salt: SaltOf(img),
				RollbackIndex: b.AVB.RollbackIndex, Key: in.Key,
				Props: []Prop{
					{"com.android.build." + f.Partition + ".os_version", major},
					{"com.android.build." + f.Partition + ".security_patch", b.AVB.SecurityPatch},
				},
			}); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func checkRootCell(dtb []byte, prop string, want uint32) error {
	f, err := findProp(dtb, "", prop)
	if err != nil {
		return err
	}
	if f.size < 4 {
		return fmt.Errorf("%s has no cell", prop)
	}
	if got := uint32(dtb[f.value])<<24 | uint32(dtb[f.value+1])<<16 | uint32(dtb[f.value+2])<<8 | uint32(dtb[f.value+3]); got != want {
		return fmt.Errorf("the DTB is built for %s %#x, the profile names %#x first", prop, got, want)
	}
	return nil
}

func emptyCpio() []byte {
	fields := make([]string, 13)
	for i := range fields {
		fields[i] = "00000000"
	}
	fields[4] = "00000001"
	fields[11] = "0000000B"
	b := []byte("070701")
	for _, f := range fields {
		b = append(b, f...)
	}
	b = append(b, "TRAILER!!!\x00"...)
	return padTo(b, 512)
}
