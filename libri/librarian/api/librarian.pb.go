// Code generated by protoc-gen-go.
// source: librarian/api/librarian.proto
// DO NOT EDIT!

package api

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type PutOperation int32

const (
	// new value was added
	PutOperation_STORED PutOperation = 0
	// value already existed
	PutOperation_LEFT_EXISTING PutOperation = 1
)

var PutOperation_name = map[int32]string{
	0: "STORED",
	1: "LEFT_EXISTING",
}
var PutOperation_value = map[string]int32{
	"STORED":        0,
	"LEFT_EXISTING": 1,
}

func (x PutOperation) String() string {
	return proto.EnumName(PutOperation_name, int32(x))
}
func (PutOperation) EnumDescriptor() ([]byte, []int) { return fileDescriptor1, []int{0} }

// RequestMetadata defines metadata associated with every request.
type RequestMetadata struct {
	// 32-byte unique request ID
	RequestId []byte `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	// peer's ECDSA public key
	PubKey []byte `protobuf:"bytes,2,opt,name=pub_key,json=pubKey,proto3" json:"pub_key,omitempty"`
}

func (m *RequestMetadata) Reset()                    { *m = RequestMetadata{} }
func (m *RequestMetadata) String() string            { return proto.CompactTextString(m) }
func (*RequestMetadata) ProtoMessage()               {}
func (*RequestMetadata) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{0} }

func (m *RequestMetadata) GetRequestId() []byte {
	if m != nil {
		return m.RequestId
	}
	return nil
}

func (m *RequestMetadata) GetPubKey() []byte {
	if m != nil {
		return m.PubKey
	}
	return nil
}

type ResponseMetadata struct {
	// 32-byte request ID that generated this response
	RequestId []byte `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	// peer's ECDSA public key
	PubKey []byte `protobuf:"bytes,2,opt,name=pub_key,json=pubKey,proto3" json:"pub_key,omitempty"`
}

func (m *ResponseMetadata) Reset()                    { *m = ResponseMetadata{} }
func (m *ResponseMetadata) String() string            { return proto.CompactTextString(m) }
func (*ResponseMetadata) ProtoMessage()               {}
func (*ResponseMetadata) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{1} }

func (m *ResponseMetadata) GetRequestId() []byte {
	if m != nil {
		return m.RequestId
	}
	return nil
}

func (m *ResponseMetadata) GetPubKey() []byte {
	if m != nil {
		return m.PubKey
	}
	return nil
}

type IntroduceRequest struct {
	Metadata *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// info about the peer making the introduction
	Self *PeerAddress `protobuf:"bytes,2,opt,name=self" json:"self,omitempty"`
	// number of peer librarians to request info for
	NumPeers uint32 `protobuf:"varint,3,opt,name=num_peers,json=numPeers" json:"num_peers,omitempty"`
}

func (m *IntroduceRequest) Reset()                    { *m = IntroduceRequest{} }
func (m *IntroduceRequest) String() string            { return proto.CompactTextString(m) }
func (*IntroduceRequest) ProtoMessage()               {}
func (*IntroduceRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{2} }

func (m *IntroduceRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *IntroduceRequest) GetSelf() *PeerAddress {
	if m != nil {
		return m.Self
	}
	return nil
}

func (m *IntroduceRequest) GetNumPeers() uint32 {
	if m != nil {
		return m.NumPeers
	}
	return 0
}

type IntroduceResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// info about the peer receiving the introduction
	Self *PeerAddress `protobuf:"bytes,2,opt,name=self" json:"self,omitempty"`
	// info about other peers
	Peers []*PeerAddress `protobuf:"bytes,3,rep,name=peers" json:"peers,omitempty"`
}

func (m *IntroduceResponse) Reset()                    { *m = IntroduceResponse{} }
func (m *IntroduceResponse) String() string            { return proto.CompactTextString(m) }
func (*IntroduceResponse) ProtoMessage()               {}
func (*IntroduceResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{3} }

func (m *IntroduceResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *IntroduceResponse) GetSelf() *PeerAddress {
	if m != nil {
		return m.Self
	}
	return nil
}

func (m *IntroduceResponse) GetPeers() []*PeerAddress {
	if m != nil {
		return m.Peers
	}
	return nil
}

type FindRequest struct {
	Metadata *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// 32-byte target to find peers around
	Key []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	// the number of closests peers to return
	NumPeers uint32 `protobuf:"varint,3,opt,name=num_peers,json=numPeers" json:"num_peers,omitempty"`
}

func (m *FindRequest) Reset()                    { *m = FindRequest{} }
func (m *FindRequest) String() string            { return proto.CompactTextString(m) }
func (*FindRequest) ProtoMessage()               {}
func (*FindRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{4} }

func (m *FindRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *FindRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *FindRequest) GetNumPeers() uint32 {
	if m != nil {
		return m.NumPeers
	}
	return 0
}

type FindResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// list of peers closest to target
	Peers []*PeerAddress `protobuf:"bytes,2,rep,name=peers" json:"peers,omitempty"`
	// value, if found
	Value *Document `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
}

func (m *FindResponse) Reset()                    { *m = FindResponse{} }
func (m *FindResponse) String() string            { return proto.CompactTextString(m) }
func (*FindResponse) ProtoMessage()               {}
func (*FindResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{5} }

func (m *FindResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *FindResponse) GetPeers() []*PeerAddress {
	if m != nil {
		return m.Peers
	}
	return nil
}

func (m *FindResponse) GetValue() *Document {
	if m != nil {
		return m.Value
	}
	return nil
}

type VerifyRequest struct {
	Metadata *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// 32-byte key of document to verify
	Key []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	// 32-byte key to use in HMAC-256 verification
	MacKey []byte `protobuf:"bytes,3,opt,name=mac_key,json=macKey,proto3" json:"mac_key,omitempty"`
	// the number of closests peers to return
	NumPeers uint32 `protobuf:"varint,4,opt,name=num_peers,json=numPeers" json:"num_peers,omitempty"`
}

func (m *VerifyRequest) Reset()                    { *m = VerifyRequest{} }
func (m *VerifyRequest) String() string            { return proto.CompactTextString(m) }
func (*VerifyRequest) ProtoMessage()               {}
func (*VerifyRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{6} }

func (m *VerifyRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *VerifyRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *VerifyRequest) GetMacKey() []byte {
	if m != nil {
		return m.MacKey
	}
	return nil
}

func (m *VerifyRequest) GetNumPeers() uint32 {
	if m != nil {
		return m.NumPeers
	}
	return 0
}

type VerifyResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// whether the peer has the the document with the given key
	Have bool `protobuf:"varint,2,opt,name=have" json:"have,omitempty"`
	// the HMAC-256 of the document's serialized bytes given the MAC key in the request
	Mac []byte `protobuf:"bytes,3,opt,name=mac,proto3" json:"mac,omitempty"`
	// list of peers closest to target
	Peers []*PeerAddress `protobuf:"bytes,4,rep,name=peers" json:"peers,omitempty"`
}

func (m *VerifyResponse) Reset()                    { *m = VerifyResponse{} }
func (m *VerifyResponse) String() string            { return proto.CompactTextString(m) }
func (*VerifyResponse) ProtoMessage()               {}
func (*VerifyResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{7} }

func (m *VerifyResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *VerifyResponse) GetHave() bool {
	if m != nil {
		return m.Have
	}
	return false
}

func (m *VerifyResponse) GetMac() []byte {
	if m != nil {
		return m.Mac
	}
	return nil
}

func (m *VerifyResponse) GetPeers() []*PeerAddress {
	if m != nil {
		return m.Peers
	}
	return nil
}

type PeerAddress struct {
	// 32-byte peer ID
	PeerId []byte `protobuf:"bytes,1,opt,name=peer_id,json=peerId,proto3" json:"peer_id,omitempty"`
	// self-reported name of the peer
	PeerName string `protobuf:"bytes,2,opt,name=peer_name,json=peerName" json:"peer_name,omitempty"`
	// public IP address
	Ip string `protobuf:"bytes,3,opt,name=ip" json:"ip,omitempty"`
	// public address TCP port
	Port uint32 `protobuf:"varint,4,opt,name=port" json:"port,omitempty"`
}

func (m *PeerAddress) Reset()                    { *m = PeerAddress{} }
func (m *PeerAddress) String() string            { return proto.CompactTextString(m) }
func (*PeerAddress) ProtoMessage()               {}
func (*PeerAddress) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{8} }

func (m *PeerAddress) GetPeerId() []byte {
	if m != nil {
		return m.PeerId
	}
	return nil
}

func (m *PeerAddress) GetPeerName() string {
	if m != nil {
		return m.PeerName
	}
	return ""
}

func (m *PeerAddress) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *PeerAddress) GetPort() uint32 {
	if m != nil {
		return m.Port
	}
	return 0
}

type StoreRequest struct {
	Metadata *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// key to store value under
	Key []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	// value to store for key
	Value *Document `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
}

func (m *StoreRequest) Reset()                    { *m = StoreRequest{} }
func (m *StoreRequest) String() string            { return proto.CompactTextString(m) }
func (*StoreRequest) ProtoMessage()               {}
func (*StoreRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{9} }

func (m *StoreRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *StoreRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *StoreRequest) GetValue() *Document {
	if m != nil {
		return m.Value
	}
	return nil
}

type StoreResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
}

func (m *StoreResponse) Reset()                    { *m = StoreResponse{} }
func (m *StoreResponse) String() string            { return proto.CompactTextString(m) }
func (*StoreResponse) ProtoMessage()               {}
func (*StoreResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{10} }

func (m *StoreResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

type GetRequest struct {
	Metadata *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// 32-byte key of document to get
	Key []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
}

func (m *GetRequest) Reset()                    { *m = GetRequest{} }
func (m *GetRequest) String() string            { return proto.CompactTextString(m) }
func (*GetRequest) ProtoMessage()               {}
func (*GetRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{11} }

func (m *GetRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *GetRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

type GetResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// value to store for key
	Value *Document `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
}

func (m *GetResponse) Reset()                    { *m = GetResponse{} }
func (m *GetResponse) String() string            { return proto.CompactTextString(m) }
func (*GetResponse) ProtoMessage()               {}
func (*GetResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{12} }

func (m *GetResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *GetResponse) GetValue() *Document {
	if m != nil {
		return m.Value
	}
	return nil
}

type PutRequest struct {
	Metadata *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// key to store value under
	Key []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	// value to store for key
	Value *Document `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
}

func (m *PutRequest) Reset()                    { *m = PutRequest{} }
func (m *PutRequest) String() string            { return proto.CompactTextString(m) }
func (*PutRequest) ProtoMessage()               {}
func (*PutRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{13} }

func (m *PutRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *PutRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *PutRequest) GetValue() *Document {
	if m != nil {
		return m.Value
	}
	return nil
}

type PutResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// result of the put operation
	Operation PutOperation `protobuf:"varint,2,opt,name=operation,enum=api.PutOperation" json:"operation,omitempty"`
	// number of replicas of the stored value; only populated for operation = STORED
	NReplicas uint32 `protobuf:"varint,3,opt,name=n_replicas,json=nReplicas" json:"n_replicas,omitempty"`
}

func (m *PutResponse) Reset()                    { *m = PutResponse{} }
func (m *PutResponse) String() string            { return proto.CompactTextString(m) }
func (*PutResponse) ProtoMessage()               {}
func (*PutResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{14} }

func (m *PutResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *PutResponse) GetOperation() PutOperation {
	if m != nil {
		return m.Operation
	}
	return PutOperation_STORED
}

func (m *PutResponse) GetNReplicas() uint32 {
	if m != nil {
		return m.NReplicas
	}
	return 0
}

type SubscribeRequest struct {
	Metadata     *RequestMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	Subscription *Subscription    `protobuf:"bytes,2,opt,name=subscription" json:"subscription,omitempty"`
}

func (m *SubscribeRequest) Reset()                    { *m = SubscribeRequest{} }
func (m *SubscribeRequest) String() string            { return proto.CompactTextString(m) }
func (*SubscribeRequest) ProtoMessage()               {}
func (*SubscribeRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{15} }

func (m *SubscribeRequest) GetMetadata() *RequestMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *SubscribeRequest) GetSubscription() *Subscription {
	if m != nil {
		return m.Subscription
	}
	return nil
}

type SubscribeResponse struct {
	Metadata *ResponseMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	Key      []byte            `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	Value    *Publication      `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
}

func (m *SubscribeResponse) Reset()                    { *m = SubscribeResponse{} }
func (m *SubscribeResponse) String() string            { return proto.CompactTextString(m) }
func (*SubscribeResponse) ProtoMessage()               {}
func (*SubscribeResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{16} }

func (m *SubscribeResponse) GetMetadata() *ResponseMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *SubscribeResponse) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *SubscribeResponse) GetValue() *Publication {
	if m != nil {
		return m.Value
	}
	return nil
}

type Publication struct {
	EnvelopeKey     []byte `protobuf:"bytes,1,opt,name=envelope_key,json=envelopeKey,proto3" json:"envelope_key,omitempty"`
	EntryKey        []byte `protobuf:"bytes,2,opt,name=entry_key,json=entryKey,proto3" json:"entry_key,omitempty"`
	AuthorPublicKey []byte `protobuf:"bytes,3,opt,name=author_public_key,json=authorPublicKey,proto3" json:"author_public_key,omitempty"`
	ReaderPublicKey []byte `protobuf:"bytes,4,opt,name=reader_public_key,json=readerPublicKey,proto3" json:"reader_public_key,omitempty"`
}

func (m *Publication) Reset()                    { *m = Publication{} }
func (m *Publication) String() string            { return proto.CompactTextString(m) }
func (*Publication) ProtoMessage()               {}
func (*Publication) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{17} }

func (m *Publication) GetEnvelopeKey() []byte {
	if m != nil {
		return m.EnvelopeKey
	}
	return nil
}

func (m *Publication) GetEntryKey() []byte {
	if m != nil {
		return m.EntryKey
	}
	return nil
}

func (m *Publication) GetAuthorPublicKey() []byte {
	if m != nil {
		return m.AuthorPublicKey
	}
	return nil
}

func (m *Publication) GetReaderPublicKey() []byte {
	if m != nil {
		return m.ReaderPublicKey
	}
	return nil
}

type Subscription struct {
	AuthorPublicKeys *BloomFilter `protobuf:"bytes,1,opt,name=author_public_keys,json=authorPublicKeys" json:"author_public_keys,omitempty"`
	ReaderPublicKeys *BloomFilter `protobuf:"bytes,2,opt,name=reader_public_keys,json=readerPublicKeys" json:"reader_public_keys,omitempty"`
}

func (m *Subscription) Reset()                    { *m = Subscription{} }
func (m *Subscription) String() string            { return proto.CompactTextString(m) }
func (*Subscription) ProtoMessage()               {}
func (*Subscription) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{18} }

func (m *Subscription) GetAuthorPublicKeys() *BloomFilter {
	if m != nil {
		return m.AuthorPublicKeys
	}
	return nil
}

func (m *Subscription) GetReaderPublicKeys() *BloomFilter {
	if m != nil {
		return m.ReaderPublicKeys
	}
	return nil
}

type BloomFilter struct {
	// using https://godoc.org/github.com/willf/bloom#BloomFilter.GobEncode
	Encoded []byte `protobuf:"bytes,1,opt,name=encoded,proto3" json:"encoded,omitempty"`
}

func (m *BloomFilter) Reset()                    { *m = BloomFilter{} }
func (m *BloomFilter) String() string            { return proto.CompactTextString(m) }
func (*BloomFilter) ProtoMessage()               {}
func (*BloomFilter) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{19} }

func (m *BloomFilter) GetEncoded() []byte {
	if m != nil {
		return m.Encoded
	}
	return nil
}

func init() {
	proto.RegisterType((*RequestMetadata)(nil), "api.RequestMetadata")
	proto.RegisterType((*ResponseMetadata)(nil), "api.ResponseMetadata")
	proto.RegisterType((*IntroduceRequest)(nil), "api.IntroduceRequest")
	proto.RegisterType((*IntroduceResponse)(nil), "api.IntroduceResponse")
	proto.RegisterType((*FindRequest)(nil), "api.FindRequest")
	proto.RegisterType((*FindResponse)(nil), "api.FindResponse")
	proto.RegisterType((*VerifyRequest)(nil), "api.VerifyRequest")
	proto.RegisterType((*VerifyResponse)(nil), "api.VerifyResponse")
	proto.RegisterType((*PeerAddress)(nil), "api.PeerAddress")
	proto.RegisterType((*StoreRequest)(nil), "api.StoreRequest")
	proto.RegisterType((*StoreResponse)(nil), "api.StoreResponse")
	proto.RegisterType((*GetRequest)(nil), "api.GetRequest")
	proto.RegisterType((*GetResponse)(nil), "api.GetResponse")
	proto.RegisterType((*PutRequest)(nil), "api.PutRequest")
	proto.RegisterType((*PutResponse)(nil), "api.PutResponse")
	proto.RegisterType((*SubscribeRequest)(nil), "api.SubscribeRequest")
	proto.RegisterType((*SubscribeResponse)(nil), "api.SubscribeResponse")
	proto.RegisterType((*Publication)(nil), "api.Publication")
	proto.RegisterType((*Subscription)(nil), "api.Subscription")
	proto.RegisterType((*BloomFilter)(nil), "api.BloomFilter")
	proto.RegisterEnum("api.PutOperation", PutOperation_name, PutOperation_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Librarian service

type LibrarianClient interface {
	// Introduce identifies the node by name and ID.
	Introduce(ctx context.Context, in *IntroduceRequest, opts ...grpc.CallOption) (*IntroduceResponse, error)
	// Find returns the value for a key or the closest peers to it.
	Find(ctx context.Context, in *FindRequest, opts ...grpc.CallOption) (*FindResponse, error)
	// Verify checks that a peer has the value for a given key or returns the closest peers to
	// that value.
	Verify(ctx context.Context, in *VerifyRequest, opts ...grpc.CallOption) (*VerifyResponse, error)
	// Store stores a value in a given key.
	Store(ctx context.Context, in *StoreRequest, opts ...grpc.CallOption) (*StoreResponse, error)
	// Get retrieves a value, if it exists.
	Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error)
	// Put stores a value.
	Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error)
	// Subscribe streams Publications to the client per a subscription filter.
	Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (Librarian_SubscribeClient, error)
}

type librarianClient struct {
	cc *grpc.ClientConn
}

func NewLibrarianClient(cc *grpc.ClientConn) LibrarianClient {
	return &librarianClient{cc}
}

func (c *librarianClient) Introduce(ctx context.Context, in *IntroduceRequest, opts ...grpc.CallOption) (*IntroduceResponse, error) {
	out := new(IntroduceResponse)
	err := grpc.Invoke(ctx, "/api.Librarian/Introduce", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *librarianClient) Find(ctx context.Context, in *FindRequest, opts ...grpc.CallOption) (*FindResponse, error) {
	out := new(FindResponse)
	err := grpc.Invoke(ctx, "/api.Librarian/Find", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *librarianClient) Verify(ctx context.Context, in *VerifyRequest, opts ...grpc.CallOption) (*VerifyResponse, error) {
	out := new(VerifyResponse)
	err := grpc.Invoke(ctx, "/api.Librarian/Verify", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *librarianClient) Store(ctx context.Context, in *StoreRequest, opts ...grpc.CallOption) (*StoreResponse, error) {
	out := new(StoreResponse)
	err := grpc.Invoke(ctx, "/api.Librarian/Store", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *librarianClient) Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error) {
	out := new(GetResponse)
	err := grpc.Invoke(ctx, "/api.Librarian/Get", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *librarianClient) Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error) {
	out := new(PutResponse)
	err := grpc.Invoke(ctx, "/api.Librarian/Put", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *librarianClient) Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (Librarian_SubscribeClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_Librarian_serviceDesc.Streams[0], c.cc, "/api.Librarian/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &librarianSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Librarian_SubscribeClient interface {
	Recv() (*SubscribeResponse, error)
	grpc.ClientStream
}

type librarianSubscribeClient struct {
	grpc.ClientStream
}

func (x *librarianSubscribeClient) Recv() (*SubscribeResponse, error) {
	m := new(SubscribeResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for Librarian service

type LibrarianServer interface {
	// Introduce identifies the node by name and ID.
	Introduce(context.Context, *IntroduceRequest) (*IntroduceResponse, error)
	// Find returns the value for a key or the closest peers to it.
	Find(context.Context, *FindRequest) (*FindResponse, error)
	// Verify checks that a peer has the value for a given key or returns the closest peers to
	// that value.
	Verify(context.Context, *VerifyRequest) (*VerifyResponse, error)
	// Store stores a value in a given key.
	Store(context.Context, *StoreRequest) (*StoreResponse, error)
	// Get retrieves a value, if it exists.
	Get(context.Context, *GetRequest) (*GetResponse, error)
	// Put stores a value.
	Put(context.Context, *PutRequest) (*PutResponse, error)
	// Subscribe streams Publications to the client per a subscription filter.
	Subscribe(*SubscribeRequest, Librarian_SubscribeServer) error
}

func RegisterLibrarianServer(s *grpc.Server, srv LibrarianServer) {
	s.RegisterService(&_Librarian_serviceDesc, srv)
}

func _Librarian_Introduce_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IntroduceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LibrarianServer).Introduce(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Librarian/Introduce",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LibrarianServer).Introduce(ctx, req.(*IntroduceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Librarian_Find_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FindRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LibrarianServer).Find(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Librarian/Find",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LibrarianServer).Find(ctx, req.(*FindRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Librarian_Verify_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(VerifyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LibrarianServer).Verify(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Librarian/Verify",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LibrarianServer).Verify(ctx, req.(*VerifyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Librarian_Store_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StoreRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LibrarianServer).Store(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Librarian/Store",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LibrarianServer).Store(ctx, req.(*StoreRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Librarian_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LibrarianServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Librarian/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LibrarianServer).Get(ctx, req.(*GetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Librarian_Put_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PutRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LibrarianServer).Put(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Librarian/Put",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LibrarianServer).Put(ctx, req.(*PutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Librarian_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LibrarianServer).Subscribe(m, &librarianSubscribeServer{stream})
}

type Librarian_SubscribeServer interface {
	Send(*SubscribeResponse) error
	grpc.ServerStream
}

type librarianSubscribeServer struct {
	grpc.ServerStream
}

func (x *librarianSubscribeServer) Send(m *SubscribeResponse) error {
	return x.ServerStream.SendMsg(m)
}

var _Librarian_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.Librarian",
	HandlerType: (*LibrarianServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Introduce",
			Handler:    _Librarian_Introduce_Handler,
		},
		{
			MethodName: "Find",
			Handler:    _Librarian_Find_Handler,
		},
		{
			MethodName: "Verify",
			Handler:    _Librarian_Verify_Handler,
		},
		{
			MethodName: "Store",
			Handler:    _Librarian_Store_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _Librarian_Get_Handler,
		},
		{
			MethodName: "Put",
			Handler:    _Librarian_Put_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _Librarian_Subscribe_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "librarian/api/librarian.proto",
}

func init() { proto.RegisterFile("librarian/api/librarian.proto", fileDescriptor1) }

var fileDescriptor1 = []byte{
	// 870 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xbc, 0x56, 0xdf, 0x6f, 0xe3, 0x44,
	0x10, 0xae, 0x93, 0x34, 0x8d, 0xc7, 0x49, 0xeb, 0x2c, 0x3f, 0x2e, 0x0a, 0x3a, 0xe9, 0x30, 0xe8,
	0x38, 0x55, 0xba, 0xb6, 0xe4, 0xc4, 0x1b, 0x3a, 0x89, 0xd3, 0xb5, 0x55, 0xb8, 0xe3, 0x2e, 0x72,
	0x2a, 0xc4, 0x5b, 0xb4, 0xb1, 0xa7, 0xd4, 0x22, 0xb6, 0x97, 0xf5, 0xba, 0x28, 0xe2, 0x85, 0x37,
	0xc4, 0x0b, 0x02, 0x89, 0x7f, 0x81, 0xbf, 0x91, 0x57, 0xe4, 0xdd, 0xf5, 0xcf, 0xa0, 0x08, 0xd2,
	0x8a, 0x37, 0x7b, 0xe6, 0xdb, 0x9d, 0x6f, 0xbe, 0x9d, 0x9d, 0x59, 0x78, 0xb8, 0x0a, 0x96, 0x9c,
	0xf2, 0x80, 0x46, 0xa7, 0x94, 0x05, 0xa7, 0xc5, 0xdf, 0x09, 0xe3, 0xb1, 0x88, 0x49, 0x9b, 0xb2,
	0x60, 0xdc, 0xc0, 0xf8, 0xb1, 0x97, 0x86, 0x18, 0x89, 0x44, 0x61, 0x9c, 0x29, 0x1c, 0xb9, 0xf8,
	0x7d, 0x8a, 0x89, 0xf8, 0x0a, 0x05, 0xf5, 0xa9, 0xa0, 0xe4, 0x21, 0x00, 0x57, 0xa6, 0x45, 0xe0,
	0x8f, 0x8c, 0x47, 0xc6, 0x93, 0xbe, 0x6b, 0x6a, 0xcb, 0xd4, 0x27, 0x0f, 0xe0, 0x80, 0xa5, 0xcb,
	0xc5, 0x77, 0xb8, 0x1e, 0xb5, 0xa4, 0xaf, 0xcb, 0xd2, 0xe5, 0x2b, 0x5c, 0x3b, 0x5f, 0x82, 0xed,
	0x62, 0xc2, 0xe2, 0x28, 0xc1, 0x3b, 0xef, 0xf5, 0xb3, 0x01, 0xf6, 0x34, 0x12, 0x3c, 0xf6, 0x53,
	0x0f, 0x35, 0x41, 0x72, 0x06, 0xbd, 0x50, 0x6f, 0x2c, 0xb7, 0xb2, 0x26, 0xef, 0x9e, 0x50, 0x16,
	0x9c, 0x34, 0x12, 0x70, 0x0b, 0x14, 0xf9, 0x18, 0x3a, 0x09, 0xae, 0xae, 0xe5, 0xe6, 0xd6, 0xc4,
	0x96, 0xe8, 0x19, 0x22, 0xff, 0xc2, 0xf7, 0x39, 0x26, 0x89, 0x2b, 0xbd, 0xe4, 0x03, 0x30, 0xa3,
	0x34, 0x5c, 0x30, 0x44, 0x9e, 0x8c, 0xda, 0x8f, 0x8c, 0x27, 0x03, 0xb7, 0x17, 0xa5, 0x61, 0x06,
	0x4c, 0x9c, 0x3f, 0x0c, 0x18, 0x56, 0x98, 0xa8, 0xfc, 0xc8, 0xa7, 0x1b, 0x54, 0xde, 0xd3, 0x54,
	0xea, 0x02, 0xfc, 0x67, 0x2e, 0x8f, 0x61, 0x3f, 0xe7, 0xd1, 0xfe, 0x47, 0x98, 0x72, 0x3b, 0x11,
	0x58, 0x17, 0x41, 0xe4, 0xef, 0x2e, 0x8d, 0x0d, 0xed, 0x52, 0xf6, 0xec, 0x73, 0xbb, 0x0c, 0xbf,
	0x1a, 0xd0, 0x57, 0x01, 0x77, 0x57, 0xa0, 0xc8, 0xad, 0xb5, 0x35, 0x37, 0xf2, 0x11, 0xec, 0xdf,
	0xd2, 0x55, 0x8a, 0x92, 0x84, 0x35, 0x19, 0x48, 0xdc, 0x4b, 0x5d, 0xb8, 0xae, 0xf2, 0x39, 0xbf,
	0x18, 0x30, 0xf8, 0x1a, 0x79, 0x70, 0xbd, 0xbe, 0x4f, 0x0d, 0x1e, 0xc0, 0x41, 0x48, 0x3d, 0x59,
	0x90, 0x6d, 0x55, 0x90, 0x21, 0xf5, 0x5e, 0x35, 0xc5, 0xe9, 0x34, 0xc4, 0xf9, 0xdd, 0x80, 0xc3,
	0x9c, 0xcb, 0xee, 0xf2, 0x10, 0xe8, 0xdc, 0xd0, 0x5b, 0x94, 0x74, 0x7a, 0xae, 0xfc, 0xce, 0x18,
	0x86, 0xd4, 0xd3, 0x5c, 0xb2, 0xcf, 0x52, 0xc4, 0xce, 0xf6, 0x02, 0xf9, 0x16, 0xac, 0x8a, 0x55,
	0xde, 0x34, 0x44, 0x5e, 0xde, 0xc2, 0x6e, 0xf6, 0x3b, 0xf5, 0xb3, 0xc4, 0xa4, 0x23, 0xa2, 0xa1,
	0x0a, 0x6d, 0xba, 0xbd, 0xcc, 0xf0, 0x86, 0x86, 0x48, 0x0e, 0xa1, 0x15, 0x30, 0x19, 0xdd, 0x74,
	0x5b, 0x01, 0xcb, 0x28, 0xb2, 0x98, 0x0b, 0x2d, 0x80, 0xfc, 0x76, 0x7e, 0x80, 0xfe, 0x5c, 0xc4,
	0x1c, 0xef, 0xf3, 0x18, 0xfe, 0x55, 0x05, 0xbc, 0x80, 0x81, 0x0e, 0xbc, 0xb3, 0xe6, 0xce, 0x0c,
	0xe0, 0x12, 0xc5, 0x3d, 0x52, 0x77, 0x10, 0x2c, 0xb9, 0xe3, 0xee, 0x75, 0x50, 0x24, 0xdf, 0xda,
	0x92, 0x7c, 0x0a, 0x30, 0x4b, 0xc5, 0xff, 0xae, 0xf9, 0x6f, 0x06, 0x58, 0x32, 0xee, 0xee, 0xe9,
	0x9d, 0x82, 0x19, 0x33, 0xe4, 0x54, 0x04, 0x71, 0x24, 0xe3, 0x1f, 0x4e, 0x86, 0xaa, 0x88, 0x53,
	0xf1, 0x36, 0x77, 0xb8, 0x25, 0x26, 0x9b, 0x21, 0xd1, 0x82, 0x23, 0x5b, 0x05, 0x1e, 0xcd, 0x1b,
	0x93, 0x19, 0xb9, 0xda, 0xe0, 0xfc, 0x08, 0xf6, 0x3c, 0x5d, 0x26, 0x1e, 0x0f, 0x96, 0x77, 0xa8,
	0xc1, 0xcf, 0xa0, 0x9f, 0xa8, 0x5d, 0x58, 0x41, 0xcc, 0xd2, 0xc4, 0xe6, 0x15, 0x87, 0x5b, 0x83,
	0x39, 0x3f, 0x19, 0x30, 0xac, 0x44, 0xdf, 0x5d, 0x95, 0xcd, 0xf3, 0x78, 0x5c, 0x3f, 0x0f, 0x7d,
	0xd1, 0xd3, 0x65, 0x96, 0xb5, 0x64, 0xa2, 0x8f, 0xe4, 0x4f, 0x79, 0x24, 0x85, 0x99, 0x7c, 0x08,
	0x7d, 0x8c, 0x6e, 0x71, 0x15, 0x33, 0x94, 0x7d, 0x4c, 0x5d, 0x77, 0x2b, 0xb7, 0xe9, 0x66, 0x86,
	0x91, 0xe0, 0xeb, 0xca, 0xe0, 0xed, 0x49, 0x43, 0xe6, 0x3c, 0x86, 0x21, 0x4d, 0xc5, 0x4d, 0xcc,
	0x17, 0x4c, 0xee, 0x5a, 0x69, 0x86, 0x47, 0xca, 0xa1, 0xa2, 0x69, 0x2c, 0x47, 0xea, 0x63, 0x0d,
	0xdb, 0x51, 0x58, 0xe5, 0x28, 0xb0, 0x72, 0x82, 0x54, 0x95, 0x24, 0xcf, 0x81, 0x6c, 0x04, 0x4a,
	0xb4, 0x5e, 0x2a, 0xdb, 0x17, 0xab, 0x38, 0x0e, 0x2f, 0x82, 0x95, 0x40, 0xee, 0xda, 0x8d, 0xd8,
	0x49, 0xb6, 0x7e, 0x23, 0x78, 0x52, 0x1b, 0xaf, 0xb5, 0xf5, 0x0d, 0x3e, 0x89, 0xf3, 0x09, 0x58,
	0x15, 0x00, 0x19, 0xc1, 0x01, 0x46, 0x5e, 0xec, 0x63, 0xde, 0x21, 0xf3, 0xdf, 0xe3, 0xa7, 0xd0,
	0xaf, 0xd6, 0x26, 0x01, 0xe8, 0xce, 0xaf, 0xde, 0xba, 0xe7, 0x2f, 0xed, 0x3d, 0x32, 0x84, 0xc1,
	0xeb, 0xf3, 0x8b, 0xab, 0xc5, 0xf9, 0x37, 0xd3, 0xf9, 0xd5, 0xf4, 0xcd, 0xa5, 0x6d, 0x4c, 0xfe,
	0x6a, 0x81, 0xf9, 0x3a, 0x7f, 0x74, 0x91, 0xcf, 0xc1, 0x2c, 0x9e, 0x0f, 0x44, 0x95, 0x41, 0xf3,
	0x61, 0x33, 0x7e, 0xbf, 0x69, 0x56, 0x65, 0xe2, 0xec, 0x91, 0xa7, 0xd0, 0xc9, 0xa6, 0x2e, 0x51,
	0xf9, 0x54, 0x26, 0xfe, 0x78, 0x58, 0xb1, 0x14, 0xf0, 0x67, 0xd0, 0x55, 0x73, 0x88, 0x10, 0xe9,
	0xae, 0x0d, 0xc8, 0xf1, 0x3b, 0x35, 0x5b, 0xb1, 0xe8, 0x0c, 0xf6, 0x65, 0x1f, 0x25, 0xba, 0xda,
	0x2b, 0xcd, 0x7c, 0x4c, 0xaa, 0xa6, 0x62, 0xc5, 0x31, 0xb4, 0x2f, 0x51, 0x90, 0x23, 0xe9, 0x2c,
	0xfb, 0xe7, 0xd8, 0x2e, 0x0d, 0x55, 0xec, 0x2c, 0xcd, 0xb1, 0x65, 0xcb, 0x1a, 0xdb, 0xa5, 0xa1,
	0xc0, 0x3e, 0x07, 0xb3, 0xb8, 0x4c, 0x5a, 0xab, 0xe6, 0xd5, 0xd6, 0x5a, 0x6d, 0xdc, 0x39, 0x67,
	0xef, 0xcc, 0x58, 0x76, 0xe5, 0x9b, 0xf6, 0xd9, 0xdf, 0x01, 0x00, 0x00, 0xff, 0xff, 0xd0, 0xfe,
	0xf7, 0xbe, 0x18, 0x0b, 0x00, 0x00,
}
