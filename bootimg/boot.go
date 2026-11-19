package bootimg

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const (
	bootPage         = 4096
	bootHeaderSize   = 1584
	vendorHeaderSize = 2128
	ramdiskEntrySize = 108

	baseAddr      = 0x10000000
	kernelOffset  = 0x00008000
	ramdiskOffset = 0x01000000
	tagsOffset    = 0x00000100
	dtbOffset     = 0x01f00000
)

type errTooLong int

func (e errTooLong) Error() string { return fmt.Sprintf("does not fit in %d bytes", int(e)) }

// OSVersion packs an Android version such as "17.0.0" and a security patch
// level such as "2026-07" the way the boot header stores them.
func OSVersion(version, patch string) (uint32, error) {
	if version == "" && patch == "" {
		return 0, nil
	}
	var v [3]uint32
	for i, part := range strings.SplitN(version, ".", 3) {
		n, err := strconv.ParseUint(part, 10, 7)
		if err != nil {
			return 0, fmt.Errorf("os version %q: %w", version, err)
		}
		v[i] = uint32(n)
	}
	var y, m uint64
	if patch != "" {
		ys, ms, ok := strings.Cut(patch, "-")
		if !ok {
			return 0, fmt.Errorf("patch level %q is not YYYY-MM", patch)
		}
		var err error
		if y, err = strconv.ParseUint(ys, 10, 16); err != nil || y < 2000 || y > 2127 {
			return 0, fmt.Errorf("patch level %q: bad year", patch)
		}
		if m, err = strconv.ParseUint(ms, 10, 8); err != nil || m < 1 || m > 12 {
			return 0, fmt.Errorf("patch level %q: bad month", patch)
		}
		y -= 2000
	}
	return (v[0]<<14|v[1]<<7|v[2])<<11 | uint32(y)<<4 | uint32(m), nil
}

// Boot is a version 4 boot image: the kernel, an optional generic ramdisk
// and the OS version, as boot and init_boot carry them.
type Boot struct {
	Kernel    []byte
	Ramdisk   []byte
	Cmdline   string
	OSVersion uint32
}

func (b Boot) Bytes() ([]byte, error) {
	h := make([]byte, bootHeaderSize)
	copy(h, "ANDROID!")
	le := binary.LittleEndian
	le.PutUint32(h[8:], uint32(len(b.Kernel)))
	le.PutUint32(h[12:], uint32(len(b.Ramdisk)))
	le.PutUint32(h[16:], b.OSVersion)
	le.PutUint32(h[20:], bootHeaderSize)
	le.PutUint32(h[40:], 4)
	if err := putString(h[44:44+1536], b.Cmdline); err != nil {
		return nil, fmt.Errorf("boot cmdline %w", err)
	}
	out := padTo(h, bootPage)
	out = padTo(append(out, b.Kernel...), bootPage)
	out = padTo(append(out, b.Ramdisk...), bootPage)
	return out, nil
}

// Ramdisk types of a vendor ramdisk table entry.
const (
	RamdiskNone     = 0
	RamdiskPlatform = 1
	RamdiskRecovery = 2
	RamdiskDLKM     = 3
)

type VendorRamdisk struct {
	Data []byte
	Type uint32
	Name string
}

// Vendor is a version 4 vendor boot image, the format of both vendor_boot
// and vendor_kernel_boot: vendor ramdisk fragments, the DTB and the vendor
// command line.
type Vendor struct {
	PageSize   int
	Cmdline    string
	Name       string
	DTB        []byte
	Ramdisks   []VendorRamdisk
	Bootconfig []byte
}

func (v Vendor) Bytes() ([]byte, error) {
	page := v.PageSize
	if page == 0 {
		page = bootPage
	}
	le := binary.LittleEndian
	var ramdisks []byte
	table := make([]byte, 0, len(v.Ramdisks)*ramdiskEntrySize)
	for _, r := range v.Ramdisks {
		e := make([]byte, ramdiskEntrySize)
		le.PutUint32(e[0:], uint32(len(r.Data)))
		le.PutUint32(e[4:], uint32(len(ramdisks)))
		le.PutUint32(e[8:], r.Type)
		if err := putString(e[12:44], r.Name); err != nil {
			return nil, fmt.Errorf("vendor ramdisk name %w", err)
		}
		table = append(table, e...)
		ramdisks = append(ramdisks, r.Data...)
	}
	h := make([]byte, vendorHeaderSize)
	copy(h, "VNDRBOOT")
	le.PutUint32(h[8:], 4)
	le.PutUint32(h[12:], uint32(page))
	le.PutUint32(h[16:], baseAddr+kernelOffset)
	le.PutUint32(h[20:], baseAddr+ramdiskOffset)
	le.PutUint32(h[24:], uint32(len(ramdisks)))
	if err := putString(h[28:28+2048], v.Cmdline); err != nil {
		return nil, fmt.Errorf("vendor cmdline %w", err)
	}
	le.PutUint32(h[2076:], baseAddr+tagsOffset)
	if err := putString(h[2080:2096], v.Name); err != nil {
		return nil, fmt.Errorf("vendor name %w", err)
	}
	le.PutUint32(h[2096:], vendorHeaderSize)
	le.PutUint32(h[2100:], uint32(len(v.DTB)))
	le.PutUint64(h[2104:], baseAddr+dtbOffset)
	le.PutUint32(h[2112:], uint32(len(table)))
	le.PutUint32(h[2116:], uint32(len(v.Ramdisks)))
	le.PutUint32(h[2120:], ramdiskEntrySize)
	le.PutUint32(h[2124:], uint32(len(v.Bootconfig)))
	out := padTo(h, page)
	for _, part := range [][]byte{ramdisks, v.DTB, table, v.Bootconfig} {
		out = padTo(append(out, part...), page)
	}
	return out, nil
}
