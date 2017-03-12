// Code generated by protoc-gen-go.
// source: libri/librarian/server/storage/storage.proto
// DO NOT EDIT!

/*
Package storage is a generated protocol buffer package.

It is generated from these files:
	libri/librarian/server/storage/storage.proto

It has these top-level messages:
	ECID
	Address
	QueryOutcomes
	QueryTypeOutcomes
	Peer
	RoutingTable
*/
package storage

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

// ECID represents an ECDSA key-pair, whose public key x-value is used as the peer ID to outside
// world.
type ECID struct {
	// name of the curve (should always be P-256, but here for backward compatibility)
	Curve string `protobuf:"bytes,1,opt,name=curve" json:"curve,omitempty"`
	// private key
	D []byte `protobuf:"bytes,2,opt,name=D,proto3" json:"D,omitempty"`
	// x-value of public key
	X []byte `protobuf:"bytes,3,opt,name=X,proto3" json:"X,omitempty"`
	// y-value of public key
	Y []byte `protobuf:"bytes,4,opt,name=Y,proto3" json:"Y,omitempty"`
}

func (m *ECID) Reset()                    { *m = ECID{} }
func (m *ECID) String() string            { return proto.CompactTextString(m) }
func (*ECID) ProtoMessage()               {}
func (*ECID) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *ECID) GetCurve() string {
	if m != nil {
		return m.Curve
	}
	return ""
}

func (m *ECID) GetD() []byte {
	if m != nil {
		return m.D
	}
	return nil
}

func (m *ECID) GetX() []byte {
	if m != nil {
		return m.X
	}
	return nil
}

func (m *ECID) GetY() []byte {
	if m != nil {
		return m.Y
	}
	return nil
}

// Address is a IPv4 address.
type Address struct {
	// IP address
	Ip string `protobuf:"bytes,2,opt,name=ip" json:"ip,omitempty"`
	// TCP port
	Port uint32 `protobuf:"varint,3,opt,name=port" json:"port,omitempty"`
}

func (m *Address) Reset()                    { *m = Address{} }
func (m *Address) String() string            { return proto.CompactTextString(m) }
func (*Address) ProtoMessage()               {}
func (*Address) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *Address) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *Address) GetPort() uint32 {
	if m != nil {
		return m.Port
	}
	return 0
}

type QueryOutcomes struct {
	Requests  *QueryTypeOutcomes `protobuf:"bytes,1,opt,name=requests" json:"requests,omitempty"`
	Responses *QueryTypeOutcomes `protobuf:"bytes,2,opt,name=responses" json:"responses,omitempty"`
}

func (m *QueryOutcomes) Reset()                    { *m = QueryOutcomes{} }
func (m *QueryOutcomes) String() string            { return proto.CompactTextString(m) }
func (*QueryOutcomes) ProtoMessage()               {}
func (*QueryOutcomes) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *QueryOutcomes) GetRequests() *QueryTypeOutcomes {
	if m != nil {
		return m.Requests
	}
	return nil
}

func (m *QueryOutcomes) GetResponses() *QueryTypeOutcomes {
	if m != nil {
		return m.Responses
	}
	return nil
}

// Responses contains statistics about a Peer's query history.
type QueryTypeOutcomes struct {
	// epoch time (seconds since 1970 UTC) of the earliest response from the peer
	Earliest int64 `protobuf:"varint,1,opt,name=earliest" json:"earliest,omitempty"`
	// epoch time of the latest response from the peer
	Latest int64 `protobuf:"varint,2,opt,name=latest" json:"latest,omitempty"`
	// number of queries sent to the peer
	NQueries uint64 `protobuf:"varint,3,opt,name=n_queries,json=nQueries" json:"n_queries,omitempty"`
	// number of queries that errored
	NErrors uint64 `protobuf:"varint,4,opt,name=n_errors,json=nErrors" json:"n_errors,omitempty"`
}

func (m *QueryTypeOutcomes) Reset()                    { *m = QueryTypeOutcomes{} }
func (m *QueryTypeOutcomes) String() string            { return proto.CompactTextString(m) }
func (*QueryTypeOutcomes) ProtoMessage()               {}
func (*QueryTypeOutcomes) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *QueryTypeOutcomes) GetEarliest() int64 {
	if m != nil {
		return m.Earliest
	}
	return 0
}

func (m *QueryTypeOutcomes) GetLatest() int64 {
	if m != nil {
		return m.Latest
	}
	return 0
}

func (m *QueryTypeOutcomes) GetNQueries() uint64 {
	if m != nil {
		return m.NQueries
	}
	return 0
}

func (m *QueryTypeOutcomes) GetNErrors() uint64 {
	if m != nil {
		return m.NErrors
	}
	return 0
}

// Peer is the basic information associated with each peer in the network.
type Peer struct {
	// big-endian byte representation of 32-byte ID
	Id []byte `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// self-reported name of the peer
	Name string `protobuf:"bytes,2,opt,name=name" json:"name,omitempty"`
	// public IP address
	PublicAddress *Address `protobuf:"bytes,3,opt,name=public_address,json=publicAddress" json:"public_address,omitempty"`
	// response history
	QueryOutcomes *QueryOutcomes `protobuf:"bytes,4,opt,name=query_outcomes,json=queryOutcomes" json:"query_outcomes,omitempty"`
}

func (m *Peer) Reset()                    { *m = Peer{} }
func (m *Peer) String() string            { return proto.CompactTextString(m) }
func (*Peer) ProtoMessage()               {}
func (*Peer) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *Peer) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

func (m *Peer) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Peer) GetPublicAddress() *Address {
	if m != nil {
		return m.PublicAddress
	}
	return nil
}

func (m *Peer) GetQueryOutcomes() *QueryOutcomes {
	if m != nil {
		return m.QueryOutcomes
	}
	return nil
}

// StoredRoutingTable contains the essential information associated with a routing table.
type RoutingTable struct {
	// big-endian byte representation of 32-byte self ID
	SelfId []byte `protobuf:"bytes,1,opt,name=self_id,json=selfId,proto3" json:"self_id,omitempty"`
	// array of peers in table
	Peers []*Peer `protobuf:"bytes,2,rep,name=peers" json:"peers,omitempty"`
}

func (m *RoutingTable) Reset()                    { *m = RoutingTable{} }
func (m *RoutingTable) String() string            { return proto.CompactTextString(m) }
func (*RoutingTable) ProtoMessage()               {}
func (*RoutingTable) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *RoutingTable) GetSelfId() []byte {
	if m != nil {
		return m.SelfId
	}
	return nil
}

func (m *RoutingTable) GetPeers() []*Peer {
	if m != nil {
		return m.Peers
	}
	return nil
}

func init() {
	proto.RegisterType((*ECID)(nil), "storage.ECID")
	proto.RegisterType((*Address)(nil), "storage.Address")
	proto.RegisterType((*QueryOutcomes)(nil), "storage.QueryOutcomes")
	proto.RegisterType((*QueryTypeOutcomes)(nil), "storage.QueryTypeOutcomes")
	proto.RegisterType((*Peer)(nil), "storage.Peer")
	proto.RegisterType((*RoutingTable)(nil), "storage.RoutingTable")
}

func init() { proto.RegisterFile("libri/librarian/server/storage/storage.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 416 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x7c, 0x92, 0x4f, 0x8b, 0x13, 0x41,
	0x10, 0xc5, 0x99, 0xc9, 0x6c, 0xfe, 0x54, 0x32, 0x41, 0x1b, 0x59, 0xe3, 0x7a, 0x59, 0xc6, 0x4b,
	0x0e, 0xba, 0x81, 0x08, 0xea, 0xc5, 0x83, 0x98, 0x15, 0x16, 0x04, 0xdd, 0x66, 0x0f, 0xbb, 0xa7,
	0x61, 0x92, 0x29, 0x97, 0x86, 0xd9, 0xee, 0x4e, 0x75, 0x4f, 0x20, 0x27, 0xf1, 0xab, 0xf8, 0x49,
	0xa5, 0x6b, 0xfe, 0xa8, 0x08, 0x5e, 0x66, 0xfa, 0x47, 0xbd, 0x9a, 0x7a, 0xf5, 0x7a, 0xe0, 0x65,
	0xa5, 0xb6, 0xa4, 0x56, 0xe1, 0x59, 0x90, 0x2a, 0xf4, 0xca, 0x21, 0x1d, 0x90, 0x56, 0xce, 0x1b,
	0x2a, 0xee, 0xb1, 0x7b, 0x5f, 0x58, 0x32, 0xde, 0x88, 0x51, 0x8b, 0xd9, 0x27, 0x48, 0x2e, 0x3f,
	0x5e, 0x6d, 0xc4, 0x13, 0x38, 0xd9, 0xd5, 0x74, 0xc0, 0x45, 0x74, 0x1e, 0x2d, 0x27, 0xb2, 0x01,
	0x31, 0x83, 0x68, 0xb3, 0x88, 0xcf, 0xa3, 0xe5, 0x4c, 0x46, 0x9b, 0x40, 0xb7, 0x8b, 0x41, 0x43,
	0xb7, 0x81, 0xee, 0x16, 0x49, 0x43, 0x77, 0xd9, 0x2b, 0x18, 0x7d, 0x28, 0x4b, 0x42, 0xe7, 0xc4,
	0x1c, 0x62, 0x65, 0xb9, 0x6b, 0x22, 0x63, 0x65, 0x85, 0x80, 0xc4, 0x1a, 0xf2, 0xdc, 0x99, 0x4a,
	0x3e, 0x67, 0x3f, 0x22, 0x48, 0xaf, 0x6b, 0xa4, 0xe3, 0x97, 0xda, 0xef, 0xcc, 0x03, 0x3a, 0xf1,
	0x06, 0xc6, 0x84, 0xfb, 0x1a, 0x9d, 0x77, 0xec, 0x61, 0xba, 0x3e, 0xbb, 0xe8, 0x3c, 0xb3, 0xf2,
	0xe6, 0x68, 0xb1, 0x53, 0xcb, 0x5e, 0x2b, 0xde, 0xc1, 0x84, 0xd0, 0x59, 0xa3, 0x1d, 0x3a, 0x1e,
	0xfa, 0xff, 0xc6, 0xdf, 0xe2, 0xec, 0x3b, 0x3c, 0xfe, 0xa7, 0x2e, 0xce, 0x60, 0x8c, 0x05, 0x55,
	0x0a, 0x9d, 0x67, 0x1b, 0x03, 0xd9, 0xb3, 0x38, 0x85, 0x61, 0x55, 0xf8, 0x50, 0x89, 0xb9, 0xd2,
	0x92, 0x78, 0x0e, 0x13, 0x9d, 0xef, 0x6b, 0x24, 0x85, 0x8e, 0xb7, 0x4c, 0xe4, 0x58, 0x5f, 0x37,
	0x2c, 0x9e, 0xc1, 0x58, 0xe7, 0x48, 0x64, 0xc8, 0x71, 0x5a, 0x89, 0x1c, 0xe9, 0x4b, 0xc6, 0xec,
	0x67, 0x04, 0xc9, 0x57, 0x44, 0xe2, 0xc4, 0x4a, 0x1e, 0x37, 0x93, 0xb1, 0x2a, 0x43, 0x62, 0xba,
	0x78, 0xc0, 0x36, 0x43, 0x3e, 0x8b, 0xb7, 0x30, 0xb7, 0xf5, 0xb6, 0x52, 0xbb, 0xbc, 0x68, 0x72,
	0xe6, 0x49, 0xd3, 0xf5, 0xa3, 0x7e, 0xd9, 0x36, 0x7f, 0x99, 0x36, 0xba, 0xee, 0x3a, 0xde, 0xc3,
	0x3c, 0x78, 0x3b, 0xe6, 0xa6, 0xdd, 0x91, 0x6d, 0x4c, 0xd7, 0xa7, 0x7f, 0xa7, 0xd4, 0x27, 0x94,
	0xee, 0xff, 0xc4, 0xec, 0x33, 0xcc, 0xa4, 0xa9, 0xbd, 0xd2, 0xf7, 0x37, 0xc5, 0xb6, 0x42, 0xf1,
	0x14, 0x46, 0x0e, 0xab, 0x6f, 0x79, 0x6f, 0x78, 0x18, 0xf0, 0xaa, 0x14, 0x2f, 0xe0, 0xc4, 0x22,
	0x52, 0xb8, 0x84, 0xc1, 0x72, 0xba, 0x4e, 0xfb, 0xcf, 0x87, 0x15, 0x65, 0x53, 0xdb, 0x0e, 0xf9,
	0xf7, 0x7b, 0xfd, 0x2b, 0x00, 0x00, 0xff, 0xff, 0x23, 0x9c, 0xd3, 0x41, 0xae, 0x02, 0x00, 0x00,
}
