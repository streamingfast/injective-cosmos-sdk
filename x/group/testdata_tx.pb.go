// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: cosmos/group/v1beta1/testdata_tx.proto

package group

import (
	fmt "fmt"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type MsgAuthenticated struct {
	Signers []github_com_cosmos_cosmos_sdk_types.AccAddress `protobuf:"bytes,1,rep,name=signers,proto3,casttype=github.com/cosmos/cosmos-sdk/types.AccAddress" json:"signers,omitempty"`
}

func (m *MsgAuthenticated) Reset()         { *m = MsgAuthenticated{} }
func (m *MsgAuthenticated) String() string { return proto.CompactTextString(m) }
func (*MsgAuthenticated) ProtoMessage()    {}
func (*MsgAuthenticated) Descriptor() ([]byte, []int) {
	return fileDescriptor_576ee3acc74f67ba, []int{0}
}
func (m *MsgAuthenticated) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MsgAuthenticated) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgAuthenticated.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MsgAuthenticated) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgAuthenticated.Merge(m, src)
}
func (m *MsgAuthenticated) XXX_Size() int {
	return m.Size()
}
func (m *MsgAuthenticated) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgAuthenticated.DiscardUnknown(m)
}

var xxx_messageInfo_MsgAuthenticated proto.InternalMessageInfo

func (m *MsgAuthenticated) GetSigners() []github_com_cosmos_cosmos_sdk_types.AccAddress {
	if m != nil {
		return m.Signers
	}
	return nil
}

func init() {
	proto.RegisterType((*MsgAuthenticated)(nil), "cosmos.group.v1beta1.MsgAuthenticated")
}

func init() {
	proto.RegisterFile("cosmos/group/v1beta1/testdata_tx.proto", fileDescriptor_576ee3acc74f67ba)
}

var fileDescriptor_576ee3acc74f67ba = []byte{
	// 209 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x52, 0x4b, 0xce, 0x2f, 0xce,
	0xcd, 0x2f, 0xd6, 0x4f, 0x2f, 0xca, 0x2f, 0x2d, 0xd0, 0x2f, 0x33, 0x4c, 0x4a, 0x2d, 0x49, 0x34,
	0xd4, 0x2f, 0x49, 0x2d, 0x2e, 0x49, 0x49, 0x2c, 0x49, 0x8c, 0x2f, 0xa9, 0xd0, 0x2b, 0x28, 0xca,
	0x2f, 0xc9, 0x17, 0x12, 0x81, 0xa8, 0xd3, 0x03, 0xab, 0xd3, 0x83, 0xaa, 0x93, 0x12, 0x49, 0xcf,
	0x4f, 0xcf, 0x07, 0x2b, 0xd0, 0x07, 0xb1, 0x20, 0x6a, 0x95, 0xe2, 0xb9, 0x04, 0x7c, 0x8b, 0xd3,
	0x1d, 0x4b, 0x4b, 0x32, 0x52, 0xf3, 0x4a, 0x32, 0x93, 0x13, 0x4b, 0x52, 0x53, 0x84, 0xbc, 0xb9,
	0xd8, 0x8b, 0x33, 0xd3, 0xf3, 0x52, 0x8b, 0x8a, 0x25, 0x18, 0x15, 0x98, 0x35, 0x78, 0x9c, 0x0c,
	0x7f, 0xdd, 0x93, 0xd7, 0x4d, 0xcf, 0x2c, 0xc9, 0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5, 0x87,
	0xba, 0x03, 0x42, 0xe9, 0x16, 0xa7, 0x64, 0xeb, 0x97, 0x54, 0x16, 0xa4, 0x16, 0xeb, 0x39, 0x26,
	0x27, 0x3b, 0xa6, 0xa4, 0x14, 0xa5, 0x16, 0x17, 0x07, 0xc1, 0x4c, 0x70, 0xb2, 0x3b, 0xf1, 0x48,
	0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6, 0x07, 0x8f, 0xe4, 0x18, 0x27, 0x3c, 0x96, 0x63, 0xb8, 0xf0,
	0x58, 0x8e, 0xe1, 0xc6, 0x63, 0x39, 0x86, 0x28, 0x15, 0xbc, 0x26, 0x56, 0x40, 0xbc, 0x99, 0xc4,
	0x06, 0x76, 0xa7, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0xad, 0x0b, 0x8c, 0x0d, 0xfd, 0x00, 0x00,
	0x00,
}

func (m *MsgAuthenticated) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgAuthenticated) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgAuthenticated) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Signers) > 0 {
		for iNdEx := len(m.Signers) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Signers[iNdEx])
			copy(dAtA[i:], m.Signers[iNdEx])
			i = encodeVarintTestdataTx(dAtA, i, uint64(len(m.Signers[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintTestdataTx(dAtA []byte, offset int, v uint64) int {
	offset -= sovTestdataTx(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *MsgAuthenticated) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Signers) > 0 {
		for _, b := range m.Signers {
			l = len(b)
			n += 1 + l + sovTestdataTx(uint64(l))
		}
	}
	return n
}

func sovTestdataTx(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozTestdataTx(x uint64) (n int) {
	return sovTestdataTx(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *MsgAuthenticated) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTestdataTx
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MsgAuthenticated: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgAuthenticated: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Signers", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTestdataTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTestdataTx
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTestdataTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Signers = append(m.Signers, make([]byte, postIndex-iNdEx))
			copy(m.Signers[len(m.Signers)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTestdataTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTestdataTx
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipTestdataTx(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTestdataTx
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTestdataTx
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTestdataTx
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthTestdataTx
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupTestdataTx
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthTestdataTx
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthTestdataTx        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTestdataTx          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupTestdataTx = fmt.Errorf("proto: unexpected end of group")
)
