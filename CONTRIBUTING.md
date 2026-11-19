# Contributing to sinty-bootimg

Thanks for your interest in contributing!

## Quick Start

```bash
git clone https://github.com/singularityos-lab/sinty-bootimg
cd sinty-bootimg
go build ./...
go test ./...
```

## Guidelines

- The images this tool writes are flashed onto real devices, and a bootloader
  that rejects one leaves the device in its bootloader. Every format change
  keeps the byte-for-byte check against images made by AOSP's tools passing:
  point `SINTY_BOOTIMG_REFERENCE` at a directory of such images and their
  inputs and run `go test ./...`.
- Device settings belong in the device's profile in
  [sinty-devices](https://github.com/singularityos-lab/sinty-devices), not in
  this tool.
- Match the surrounding style. Comments explain WHY, not WHAT.
- By submitting a contribution you agree to the [CLA](CLA.md).

## License

GPL-3.0-or-later.

## Device- and environment-specific code

Code that exists only because of a specific device, architecture, firmware or environment must be marked on the line above it:

```
DEVICE-FIX(<scope>): <what breaks without this>
```

- `<scope>` is one of: `arm64`, `x86_64`, `uno-q`, `jetson`, `msm`, `nvidia`, `intel`, `amd`, `no-tpm`, `vm`, `u-boot`, `uefi`. A new scope is added to this list in the same pull request that first uses it.
- The text states what fails on that device without the code, not what the code does.
- Use the comment syntax of the file's language.
- This covers driver names, hardware probes, architecture constants, firmware quirks, boot-chain differences and anything gated on a specific environment.
- A pull request that adds or changes such code without the marker is not merged.
- `git grep -n "DEVICE-FIX("` lists every marker in the repository.
