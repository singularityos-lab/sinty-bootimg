package bootimg

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
)

const (
	avbBlock        = 4096
	avbFooterSize   = 64
	avbAlgSHA256RSA = 2
	avbRelease      = "avbtool 1.4.0"
)

type Prop struct {
	Key   string
	Value string
}

type HashFooter struct {
	Partition     string
	PartitionSize int64
	Salt          []byte
	RollbackIndex uint64
	Props         []Prop
	Key           *rsa.PrivateKey
}

func ParseKey(pemData []byte) (*rsa.PrivateKey, error) {
	b, _ := pem.Decode(pemData)
	if b == nil {
		return nil, errors.New("no PEM key")
	}
	if k, err := x509.ParsePKCS1PrivateKey(b.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(b.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA key")
	}
	return rk, nil
}

func pad8(b []byte) []byte { return padTo(b, 8) }

func hashDescriptor(f HashFooter, imageSize int, digest []byte) []byte {
	be := binary.BigEndian
	body := make([]byte, 116)
	be.PutUint64(body[0:], uint64(imageSize))
	copy(body[8:40], "sha256")
	be.PutUint32(body[40:], uint32(len(f.Partition)))
	be.PutUint32(body[44:], uint32(len(f.Salt)))
	be.PutUint32(body[48:], uint32(len(digest)))
	body = append(body, f.Partition...)
	body = append(body, f.Salt...)
	body = append(body, digest...)
	return descriptor(2, body)
}

func propDescriptor(p Prop) []byte {
	be := binary.BigEndian
	body := make([]byte, 16)
	be.PutUint64(body[0:], uint64(len(p.Key)))
	be.PutUint64(body[8:], uint64(len(p.Value)))
	body = append(body, p.Key...)
	body = append(body, 0)
	body = append(body, p.Value...)
	body = append(body, 0)
	return descriptor(0, body)
}

func descriptor(tag uint64, body []byte) []byte {
	body = pad8(body)
	h := make([]byte, 16)
	binary.BigEndian.PutUint64(h[0:], tag)
	binary.BigEndian.PutUint64(h[8:], uint64(len(body)))
	return append(h, body...)
}

func avbPublicKey(pub *rsa.PublicKey) []byte {
	bits := pub.N.BitLen()
	n := pub.N.FillBytes(make([]byte, bits/8))
	b32 := new(big.Int).Lsh(big.NewInt(1), 32)
	n0inv := new(big.Int).Sub(b32, new(big.Int).ModInverse(new(big.Int).Mod(pub.N, b32), b32))
	rr := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(2*bits)), pub.N).FillBytes(make([]byte, bits/8))
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out[0:], uint32(bits))
	binary.BigEndian.PutUint32(out[4:], uint32(n0inv.Uint64()))
	out = append(out, n...)
	return append(out, rr...)
}

func vbmeta(f HashFooter, descs []byte) ([]byte, error) {
	be := binary.BigEndian
	pub := avbPublicKey(&f.Key.PublicKey)
	sigSize := f.Key.Size()
	aux := padTo(append(bytes.Clone(descs), pub...), 64)
	authSize := align(sha256.Size+sigSize, 64)
	h := make([]byte, 256)
	copy(h, "AVB0")
	be.PutUint32(h[4:], 1)
	be.PutUint32(h[8:], 0)
	be.PutUint64(h[12:], uint64(authSize))
	be.PutUint64(h[20:], uint64(len(aux)))
	be.PutUint32(h[28:], avbAlgSHA256RSA)
	be.PutUint64(h[32:], 0)
	be.PutUint64(h[40:], sha256.Size)
	be.PutUint64(h[48:], sha256.Size)
	be.PutUint64(h[56:], uint64(sigSize))
	be.PutUint64(h[64:], uint64(len(descs)))
	be.PutUint64(h[72:], uint64(len(pub)))
	be.PutUint64(h[80:], uint64(len(descs)+len(pub)))
	be.PutUint64(h[88:], 0)
	be.PutUint64(h[96:], 0)
	be.PutUint64(h[104:], uint64(len(descs)))
	be.PutUint64(h[112:], f.RollbackIndex)
	copy(h[128:176], avbRelease)
	sum := sha256.Sum256(append(bytes.Clone(h), aux...))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.Key, crypto.SHA256, sum[:])
	if err != nil {
		return nil, err
	}
	auth := make([]byte, authSize)
	copy(auth, sum[:])
	copy(auth[sha256.Size:], sig)
	out := append(h, auth...)
	return append(out, aux...), nil
}

// AddHashFooter appends a signed vbmeta and an AVB footer to image, padding
// it to the partition size, as a chained boot partition needs to verify.
func AddHashFooter(image []byte, f HashFooter) ([]byte, error) {
	if f.Key == nil {
		return nil, errors.New("no signing key")
	}
	h := sha256.New()
	h.Write(f.Salt)
	h.Write(image)
	descs := hashDescriptor(f, len(image), h.Sum(nil))
	for _, p := range f.Props {
		descs = append(descs, propDescriptor(p)...)
	}
	meta, err := vbmeta(f, descs)
	if err != nil {
		return nil, err
	}
	out := padTo(bytes.Clone(image), avbBlock)
	offset := len(out)
	out = append(out, meta...)
	if int64(len(out)+avbFooterSize) > f.PartitionSize {
		return nil, fmt.Errorf("%s: image and vbmeta take %d bytes, the partition has %d", f.Partition, len(out)+avbFooterSize, f.PartitionSize)
	}
	out = append(out, make([]byte, int(f.PartitionSize)-len(out)-avbFooterSize)...)
	foot := make([]byte, avbFooterSize)
	copy(foot, "AVBf")
	binary.BigEndian.PutUint32(foot[4:], 1)
	binary.BigEndian.PutUint64(foot[12:], uint64(len(image)))
	binary.BigEndian.PutUint64(foot[20:], uint64(offset))
	binary.BigEndian.PutUint64(foot[28:], uint64(len(meta)))
	return append(out, foot...), nil
}

// SaltOf is the salt the footer takes from the image itself, so that the
// same image always gets the same footer.
func SaltOf(image []byte) []byte {
	s := sha256.Sum256(image)
	return s[:]
}
