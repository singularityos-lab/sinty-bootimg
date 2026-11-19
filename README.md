# Sinty Bootimg

Packs a Sinty kernel, device tree and initramfs into the boot images a device's
bootloader loads, for devices whose bootloader does not read the kernel from a
filesystem, such as Android phones.

What goes into each image comes from the device's profile in
[sinty-devices](https://github.com/singularityos-lab/sinty-devices): which
images the device has, the device tree and the SoC revisions it is copied for,
the kernel command line, the OS version the bootloader checks, the board
overlay entry and the AVB footers. Adding a phone means adding its profile, not
changing this tool.

## Formats

`android-v4` writes the version 4 boot image formats:

- `boot`, the kernel, compressed with legacy lz4 when it is not already;
- `init_boot`, an empty generic ramdisk and the OS version;
- `vendor_boot` and `vendor_kernel_boot`, the vendor command line, the device
  tree copies and the initramfs;
- `dtbo`, a board overlay that changes nothing, so the bootloader finds an entry
  for the board without altering the device tree;
- AVB hash footers signed with the given key and padded to each partition.

The output is byte for byte what AOSP's `mkbootimg`, `mkdtboimg` and `avbtool`
produce from the same inputs. The tests check this when
`SINTY_BOOTIMG_REFERENCE` names a directory of such images and their inputs.

## Usage

```sh
sinty-bootimg android -device google/pixel-7-pro \
  -kernel Image -dts-dir arch/arm64/boot/dts -initramfs initrd.cpio.xz \
  -key avb.pem -out images/
```

`-device` reads the profile from the catalog built into the tool, and
`-profile` reads one from a file, for a profile that is not merged yet. `-dtb`
names the device tree directly instead of the kernel's `dts` directory. The
images are written as `<image>.img` with a `SHA256SUMS` file.

## Building

```sh
go build ./cmd/sinty-bootimg
go test ./...
```

## License

GPL-3.0-or-later.
