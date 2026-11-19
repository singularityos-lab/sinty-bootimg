package bootimg

import "encoding/binary"

const dtTableMagic = 0xd7b7ab1e

// Overlay is one entry of a DTBO image, matched by the bootloader on its
// board id and revision.
type Overlay struct {
	DTB []byte
	ID  uint32
	Rev uint32
}

// DTBO builds the dt_table image a dtbo partition carries.
func DTBO(overlays []Overlay, pageSize uint32) []byte {
	be := binary.BigEndian
	const headerSize, entrySize = 32, 32
	offset := headerSize + entrySize*len(overlays)
	total := offset
	for _, o := range overlays {
		total += len(o.DTB)
	}
	out := make([]byte, headerSize, total)
	be.PutUint32(out[0:], dtTableMagic)
	be.PutUint32(out[4:], uint32(total))
	be.PutUint32(out[8:], headerSize)
	be.PutUint32(out[12:], entrySize)
	be.PutUint32(out[16:], uint32(len(overlays)))
	be.PutUint32(out[20:], headerSize)
	be.PutUint32(out[24:], pageSize)
	for _, o := range overlays {
		e := make([]byte, entrySize)
		be.PutUint32(e[0:], uint32(len(o.DTB)))
		be.PutUint32(e[4:], uint32(offset))
		be.PutUint32(e[8:], o.ID)
		be.PutUint32(e[12:], o.Rev)
		out = append(out, e...)
		offset += len(o.DTB)
	}
	for _, o := range overlays {
		out = append(out, o.DTB...)
	}
	return padTo(out, int(pageSize))
}
