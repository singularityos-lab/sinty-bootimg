package bootimg

import (
	"bytes"
	"debug/pe"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// FromUKI takes the kernel, the initramfs and the device tree of the device
// with the given compatible string out of a unified kernel image, so the boot
// images carry the bytes of the signed release.
func FromUKI(uki []byte, compatible string) (kernel, initramfs, dtb []byte, err error) {
	f, err := pe.NewFile(bytes.NewReader(uki))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("not a unified kernel image: %w", err)
	}
	defer f.Close()
	var matches []string
	for _, s := range f.Sections {
		data, err := s.Data()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("section %s: %w", s.Name, err)
		}
		if s.VirtualSize != 0 && int(s.VirtualSize) < len(data) {
			data = data[:s.VirtualSize]
		}
		switch s.Name {
		case ".linux":
			kernel = data
		case ".initrd":
			initramfs = data
		case ".dtb", ".dtbauto":
			compat, err := NodeString(data, "", "compatible")
			if err != nil {
				continue
			}
			if slices.Contains(strings.Split(compat, "\x00"), compatible) {
				dtb = data
				model, _ := NodeString(data, "", "model")
				matches = append(matches, model)
			}
		}
	}
	switch {
	case kernel == nil:
		return nil, nil, nil, errors.New("the image has no .linux section")
	case initramfs == nil:
		return nil, nil, nil, errors.New("the image has no .initrd section")
	case len(matches) == 0:
		return nil, nil, nil, fmt.Errorf("the image has no device tree for %s", compatible)
	case len(matches) > 1:
		return nil, nil, nil, fmt.Errorf("the image has %d device trees for %s: %s", len(matches), compatible, strings.Join(matches, ", "))
	}
	return kernel, initramfs, dtb, nil
}
