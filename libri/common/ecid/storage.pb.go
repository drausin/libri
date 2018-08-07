// Code generated by protoc-gen-go. DO NOT EDIT.
// source: libri/common/ecid/storage.proto

/*
Package ecid is a generated protocol buffer package.

It is generated from these files:
	libri/common/ecid/storage.proto

It has these top-level messages:
	ECDSAPrivateKey
*/
package ecid

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// ECDSAPrivateKey represents an ECDSA key-pair, whose public key x-value is used as the peer ID
// to outside world.
type ECDSAPrivateKey struct {
	// name of the curve used
	Curve string `protobuf:"bytes,1,opt,name=curve" json:"curve,omitempty"`
	// private key
	D []byte `protobuf:"bytes,2,opt,name=D,proto3" json:"D,omitempty"`
	// x-value of public key
	X []byte `protobuf:"bytes,3,opt,name=X,proto3" json:"X,omitempty"`
	// y-value of public key
	Y []byte `protobuf:"bytes,4,opt,name=Y,proto3" json:"Y,omitempty"`
}

func (m *ECDSAPrivateKey) Reset()                    { *m = ECDSAPrivateKey{} }
func (m *ECDSAPrivateKey) String() string            { return proto.CompactTextString(m) }
func (*ECDSAPrivateKey) ProtoMessage()               {}
func (*ECDSAPrivateKey) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *ECDSAPrivateKey) GetCurve() string {
	if m != nil {
		return m.Curve
	}
	return ""
}

func (m *ECDSAPrivateKey) GetD() []byte {
	if m != nil {
		return m.D
	}
	return nil
}

func (m *ECDSAPrivateKey) GetX() []byte {
	if m != nil {
		return m.X
	}
	return nil
}

func (m *ECDSAPrivateKey) GetY() []byte {
	if m != nil {
		return m.Y
	}
	return nil
}

func init() {
	proto.RegisterType((*ECDSAPrivateKey)(nil), "ecid.ECDSAPrivateKey")
}

func init() { proto.RegisterFile("libri/common/ecid/storage.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 132 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0xcf, 0xc9, 0x4c, 0x2a,
	0xca, 0xd4, 0x4f, 0xce, 0xcf, 0xcd, 0xcd, 0xcf, 0xd3, 0x4f, 0x4d, 0xce, 0x4c, 0xd1, 0x2f, 0x2e,
	0xc9, 0x2f, 0x4a, 0x4c, 0x4f, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x01, 0x89, 0x29,
	0x05, 0x72, 0xf1, 0xbb, 0x3a, 0xbb, 0x04, 0x3b, 0x06, 0x14, 0x65, 0x96, 0x25, 0x96, 0xa4, 0x7a,
	0xa7, 0x56, 0x0a, 0x89, 0x70, 0xb1, 0x26, 0x97, 0x16, 0x95, 0xa5, 0x4a, 0x30, 0x2a, 0x30, 0x6a,
	0x70, 0x06, 0x41, 0x38, 0x42, 0x3c, 0x5c, 0x8c, 0x2e, 0x12, 0x4c, 0x0a, 0x8c, 0x1a, 0x3c, 0x41,
	0x8c, 0x2e, 0x20, 0x5e, 0x84, 0x04, 0x33, 0x84, 0x17, 0x01, 0xe2, 0x45, 0x4a, 0xb0, 0x40, 0x78,
	0x91, 0x49, 0x6c, 0x60, 0xf3, 0x8d, 0x01, 0x01, 0x00, 0x00, 0xff, 0xff, 0xb6, 0x4a, 0x7c, 0x21,
	0x82, 0x00, 0x00, 0x00,
}
