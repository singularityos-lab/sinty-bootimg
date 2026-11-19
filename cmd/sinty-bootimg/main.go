package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	devices "github.com/singularityos-lab/sinty-devices"
	"github.com/singularityos-lab/sinty-devices/profile"

	"github.com/singularityos-lab/sinty-bootimg/bootimg"
)

var version = "dev"

const usage = `sinty-bootimg packs a Sinty kernel into the boot images a device's
bootloader loads, the way the device's profile describes them.

usage:
  sinty-bootimg android [flags]   build the images of an android-v4 device
  sinty-bootimg version           print the version

android flags:
  -device <id>        a device from the embedded catalog, such as google/pixel-7-pro
  -profile <file>     a profile file instead of a catalog device
  -kernel <file>      the kernel Image, raw or legacy lz4
  -dtb <file>         the device tree, or
  -dts-dir <dir>      the kernel's arch/<arch>/boot/dts, where the profile's dtb is
  -initramfs <file>   the initramfs, in any compression the kernel unpacks
  -key <file>         the PEM RSA key for the AVB footers
  -out <dir>          where to write <image>.img and SHA256SUMS
`

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "android":
		err = cmdAndroid(args[1:])
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sinty-bootimg:", err)
		return 1
	}
	return 0
}

func loadProfile(id, file string) (*profile.Profile, error) {
	switch {
	case id != "" && file != "":
		return nil, errors.New("give -device or -profile, not both")
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return profile.Parse(data)
	case id != "":
		ps, err := devices.Profiles()
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if p.ID == id {
				return p, nil
			}
		}
		return nil, fmt.Errorf("no device %q in the catalog", id)
	}
	return nil, errors.New("-device or -profile is required")
}

func cmdAndroid(args []string) error {
	fl := flag.NewFlagSet("android", flag.ContinueOnError)
	id := fl.String("device", "", "")
	file := fl.String("profile", "", "")
	kernel := fl.String("kernel", "", "")
	dtb := fl.String("dtb", "", "")
	dtsDir := fl.String("dts-dir", "", "")
	initramfs := fl.String("initramfs", "", "")
	keyFile := fl.String("key", "", "")
	out := fl.String("out", "", "")
	fl.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fl.Parse(args); err != nil {
		return err
	}
	p, err := loadProfile(*id, *file)
	if err != nil {
		return err
	}
	if p.Boot == nil {
		return fmt.Errorf("%s has no boot section", p.ID)
	}
	if *out == "" || *kernel == "" || *initramfs == "" {
		return errors.New("-kernel, -initramfs and -out are required")
	}
	switch {
	case *dtb != "" && *dtsDir != "":
		return errors.New("give -dtb or -dts-dir, not both")
	case *dtsDir != "":
		*dtb = filepath.Join(*dtsDir, filepath.FromSlash(p.Boot.DTB))
	case *dtb == "":
		return errors.New("-dtb or -dts-dir is required")
	}
	in := bootimg.Inputs{}
	for _, f := range []struct {
		dst  *[]byte
		path string
	}{{&in.Kernel, *kernel}, {&in.DTB, *dtb}, {&in.Initramfs, *initramfs}} {
		if *f.dst, err = os.ReadFile(f.path); err != nil {
			return err
		}
	}
	if *keyFile != "" {
		pem, err := os.ReadFile(*keyFile)
		if err != nil {
			return err
		}
		if in.Key, err = bootimg.ParseKey(pem); err != nil {
			return fmt.Errorf("%s: %w", *keyFile, err)
		}
	}
	imgs, err := bootimg.Android(p.Boot, in)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(imgs))
	for name := range imgs {
		names = append(names, name)
	}
	slices.Sort(names)
	var sums strings.Builder
	for _, name := range names {
		path := filepath.Join(*out, name+".img")
		if err := os.WriteFile(path, imgs[name], 0o644); err != nil {
			return err
		}
		sum := sha256.Sum256(imgs[name])
		fmt.Fprintf(&sums, "%s  %s.img\n", hex.EncodeToString(sum[:]), name)
		fmt.Printf("%s.img  %d bytes\n", name, len(imgs[name]))
	}
	return os.WriteFile(filepath.Join(*out, "SHA256SUMS"), []byte(sums.String()), 0o644)
}
