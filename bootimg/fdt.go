package bootimg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	fdtMagic     = 0xd00dfeed
	fdtBeginNode = 1
	fdtEndNode   = 2
	fdtProp      = 3
	fdtNop       = 4
	fdtEnd       = 9
)

type fdtProperty struct {
	header int
	value  int
	size   int
	name   int
}

// findProp locates a property of the root node (node "") or of a node
// directly under it.
func findProp(dtb []byte, node, prop string) (fdtProperty, error) {
	be := binary.BigEndian
	if len(dtb) < 40 || be.Uint32(dtb) != fdtMagic {
		return fdtProperty{}, errors.New("not a flattened device tree")
	}
	structOff := int(be.Uint32(dtb[8:]))
	stringsOff := int(be.Uint32(dtb[12:]))
	structEnd := structOff + int(be.Uint32(dtb[36:]))
	if structEnd > len(dtb) || stringsOff > len(dtb) {
		return fdtProperty{}, errors.New("truncated device tree")
	}
	want := 1
	if node != "" {
		want = 2
	}
	depth, in := 0, false
	for p := structOff; p+4 <= structEnd; {
		tok := be.Uint32(dtb[p:])
		p += 4
		switch tok {
		case fdtBeginNode:
			i := bytes.IndexByte(dtb[p:structEnd], 0)
			if i < 0 {
				return fdtProperty{}, errors.New("unterminated node name")
			}
			name := string(dtb[p : p+i])
			p += align(i+1, 4)
			depth++
			in = depth == want && (depth == 1 || name == node)
		case fdtEndNode:
			depth--
			in = depth == 1 && want == 1
		case fdtProp:
			if p+8 > structEnd {
				return fdtProperty{}, errors.New("truncated property")
			}
			size := int(be.Uint32(dtb[p:]))
			nameOff := int(be.Uint32(dtb[p+4:]))
			f := fdtProperty{header: p - 4, value: p + 8, size: size, name: nameOff}
			p = f.value + align(size, 4)
			if p > structEnd {
				return fdtProperty{}, errors.New("truncated property")
			}
			if !in {
				continue
			}
			n := dtb[stringsOff+nameOff:]
			if i := bytes.IndexByte(n, 0); i >= 0 && string(n[:i]) == prop {
				return f, nil
			}
		case fdtNop:
		case fdtEnd:
			p = structEnd
		default:
			return fdtProperty{}, fmt.Errorf("bad token %#x", tok)
		}
	}
	where := "/"
	if node != "" {
		where = "/" + node
	}
	return fdtProperty{}, fmt.Errorf("no %s on %s", prop, where)
}

func SetRootCell(dtb []byte, prop string, value uint32) ([]byte, error) {
	f, err := findProp(dtb, "", prop)
	if err != nil {
		return nil, err
	}
	if f.size < 4 {
		return nil, fmt.Errorf("%s has no cell", prop)
	}
	out := bytes.Clone(dtb)
	binary.BigEndian.PutUint32(out[f.value:], value)
	return out, nil
}

func NodeString(dtb []byte, node, prop string) (string, error) {
	f, err := findProp(dtb, node, prop)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(dtb[f.value:f.value+f.size], "\x00")), nil
}

// SetNodeString sets a string property of the root or of a node directly
// under it, such as /chosen bootargs, growing or shrinking the tree.
func SetNodeString(dtb []byte, node, prop, value string) ([]byte, error) {
	be := binary.BigEndian
	f, err := findProp(dtb, node, prop)
	if err != nil {
		return nil, err
	}
	structOff := int(be.Uint32(dtb[8:]))
	structSize := int(be.Uint32(dtb[36:]))
	stringsOff := int(be.Uint32(dtb[12:]))
	if stringsOff < structOff+structSize {
		return nil, errors.New("the strings block comes before the end of the structure block")
	}
	val := padTo(append([]byte(value), 0), 4)
	end := f.value + align(f.size, 4)
	delta := len(val) - (end - f.value)
	out := make([]byte, 0, len(dtb)+delta)
	out = append(out, dtb[:f.header]...)
	out = be.AppendUint32(out, fdtProp)
	out = be.AppendUint32(out, uint32(len(value)+1))
	out = be.AppendUint32(out, uint32(f.name))
	out = append(out, val...)
	out = append(out, dtb[end:]...)
	be.PutUint32(out[4:], be.Uint32(dtb[4:])+uint32(delta))
	be.PutUint32(out[12:], uint32(stringsOff+delta))
	be.PutUint32(out[36:], uint32(structSize+delta))
	return out, nil
}

// BoardOverlay is an overlay that changes nothing and carries only the
// board id and revision a bootloader matches dtbo entries on.
func BoardOverlay(id, rev uint32) []byte {
	be := binary.BigEndian
	strs := []byte("board_id\x00board_rev\x00target-path\x00")
	var st []byte
	node := func(name string) {
		st = be.AppendUint32(st, fdtBeginNode)
		st = padTo(append(st, name+"\x00"...), 4)
	}
	prop := func(nameOff int, val []byte) {
		st = be.AppendUint32(st, fdtProp)
		st = be.AppendUint32(st, uint32(len(val)))
		st = be.AppendUint32(st, uint32(nameOff))
		st = padTo(append(st, val...), 4)
	}
	end := func() { st = be.AppendUint32(st, fdtEndNode) }
	node("")
	prop(0, be.AppendUint32(nil, id))
	prop(9, be.AppendUint32(nil, rev))
	node("fragment@0")
	prop(19, []byte("/\x00"))
	node("__overlay__")
	end()
	end()
	end()
	st = be.AppendUint32(st, fdtEnd)
	const headerSize, rsvSize = 40, 16
	h := make([]byte, headerSize+rsvSize)
	be.PutUint32(h[0:], fdtMagic)
	be.PutUint32(h[4:], uint32(headerSize+rsvSize+len(st)+len(strs)))
	be.PutUint32(h[8:], headerSize+rsvSize)
	be.PutUint32(h[12:], uint32(headerSize+rsvSize+len(st)))
	be.PutUint32(h[16:], headerSize)
	be.PutUint32(h[20:], 17)
	be.PutUint32(h[24:], 16)
	be.PutUint32(h[32:], uint32(len(strs)))
	be.PutUint32(h[36:], uint32(len(st)))
	out := append(h, st...)
	return append(out, strs...)
}
