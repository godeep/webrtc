package rtcp

import (
	"encoding/binary"

	"github.com/pions/webrtc/internal/util"
)

// SDESType is the item type used in the RTCP SDES control packet.
type SDESType uint8

// RTP SDES item types registered with IANA. See: https://www.iana.org/assignments/rtp-parameters/rtp-parameters.xhtml#rtp-parameters-5
const (
	SDESEnd      SDESType = iota // end of SDES list                RFC 3550, 6.5
	SDESCNAME                    // canonical name                  RFC 3550, 6.5.1
	SDESName                     // user name                       RFC 3550, 6.5.2
	SDESEmail                    // user's electronic mail address  RFC 3550, 6.5.3
	SDESPhone                    // user's phone number             RFC 3550, 6.5.4
	SDESLocation                 // geographic user location        RFC 3550, 6.5.5
	SDESTool                     // name of application or tool     RFC 3550, 6.5.6
	SDESNote                     // notice about the source         RFC 3550, 6.5.7
	SDESPrivate                  // private extensions              RFC 3550, 6.5.8  (not implemented)
)

func (s SDESType) String() string {
	switch s {
	case SDESEnd:
		return "END"
	case SDESCNAME:
		return "CNAME"
	case SDESName:
		return "NAME"
	case SDESEmail:
		return "EMAIL"
	case SDESPhone:
		return "PHONE"
	case SDESLocation:
		return "LOC"
	case SDESTool:
		return "TOOL"
	case SDESNote:
		return "NOTE"
	case SDESPrivate:
		return "PRIV"
	default:
		return string(s)
	}
}

const (
	sdesSourceLen        = 4
	sdesTypeLen          = 1
	sdesTypeOffset       = 0
	sdesOctetCountLen    = 1
	sdesOctetCountOffset = 1
	sdesMaxOctetCount    = (1 << 8) - 1
	sdesTextOffset       = 2
)

// A SourceDescription (SDES) packet describes the sources in an RTP stream.
type SourceDescription struct {
	Chunks []SourceDescriptionChunk
}

// Marshal encodes the SourceDescription in binary
func (s SourceDescription) Marshal() ([]byte, error) {
	/*
	 *         0                   1                   2                   3
	 *         0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * header |V=2|P|    SC   |  PT=SDES=202  |             length            |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_1                          |
	 *   1    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_2                          |
	 *   2    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	rawPacket := make([]byte, 0)
	for _, c := range s.Chunks {
		data, err := c.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	if len(s.Chunks) > countMax {
		return nil, errTooManyChunks
	}

	h := Header{
		Count:  uint8(len(s.Chunks)),
		Type:   TypeSourceDescription,
		Length: uint16(((headerLength + len(rawPacket)) / 4) - 1),
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	rawPacket = append(hData, rawPacket...)

	return rawPacket, nil
}

// Unmarshal decodes the SourceDescription from binary
func (s *SourceDescription) Unmarshal(rawPacket []byte) error {
	/*
	 *         0                   1                   2                   3
	 *         0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * header |V=2|P|    SC   |  PT=SDES=202  |             length            |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_1                          |
	 *   1    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_2                          |
	 *   2    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if h.Type != TypeSourceDescription {
		return errWrongType
	}

	for i := headerLength; i < len(rawPacket); {
		var chunk SourceDescriptionChunk
		if err := chunk.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		s.Chunks = append(s.Chunks, chunk)

		i += chunk.len()
	}

	if len(s.Chunks) != int(h.Count) {
		return errInvalidHeader
	}

	return nil
}

// A SourceDescriptionChunk contains items describing a single RTP source
type SourceDescriptionChunk struct {
	// The source (ssrc) or contributing source (csrc) identifier this packet describes
	Source uint32
	Items  []SourceDescriptionItem
}

// Marshal encodes the SourceDescriptionChunk in binary
func (s SourceDescriptionChunk) Marshal() ([]byte, error) {
	/*
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 *  |                          SSRC/CSRC_1                          |
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |                           SDES items                          |
	 *  |                              ...                              |
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	rawPacket := make([]byte, sdesSourceLen)
	binary.BigEndian.PutUint32(rawPacket, s.Source)

	for _, it := range s.Items {
		data, err := it.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	// The list of items in each chunk MUST be terminated by one or more null octets
	rawPacket = append(rawPacket, uint8(SDESEnd))

	// additional null octets MUST be included if needed to pad until the next 32-bit boundary
	rawPacket = append(rawPacket, make([]byte, util.GetPadding(len(rawPacket)))...)

	return rawPacket, nil
}

// Unmarshal decodes the SourceDescriptionChunk from binary
func (s *SourceDescriptionChunk) Unmarshal(rawPacket []byte) error {
	/*
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 *  |                          SSRC/CSRC_1                          |
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |                           SDES items                          |
	 *  |                              ...                              |
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	if len(rawPacket) < (sdesSourceLen + sdesTypeLen) {
		return errPacketTooShort
	}

	s.Source = binary.BigEndian.Uint32(rawPacket)

	for i := 4; i < len(rawPacket); {
		if pktType := SDESType(rawPacket[i]); pktType == SDESEnd {
			return nil
		}

		var it SourceDescriptionItem
		if err := it.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		s.Items = append(s.Items, it)
		i += it.len()
	}

	return errPacketTooShort
}

func (s SourceDescriptionChunk) len() int {
	len := sdesSourceLen
	for _, it := range s.Items {
		len += it.len()
	}
	len += sdesTypeLen // for terminating null octet

	// align to 32-bit boundary
	len += util.GetPadding(len)

	return len
}

// A SourceDescriptionItem is a part of a SourceDescription that describes a stream.
type SourceDescriptionItem struct {
	// The type identifier for this item. eg, SDESCNAME for canonical name description.
	//
	// Type zero or SDESEnd is interpreted as the end of an item list and cannot be used.
	Type SDESType
	// Text is a unicode text blob associated with the item. Its meaning varies based on the item's Type.
	Text string
}

func (s SourceDescriptionItem) len() int {
	/*
	 *   0                   1                   2                   3
	 *   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |    CNAME=1    |     length    | user and domain name        ...
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */
	return sdesTypeLen + sdesOctetCountLen + len([]byte(s.Text))
}

// Marshal encodes the SourceDescriptionItem in binary
func (s SourceDescriptionItem) Marshal() ([]byte, error) {
	/*
	 *   0                   1                   2                   3
	 *   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |    CNAME=1    |     length    | user and domain name        ...
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if s.Type == SDESEnd {
		return nil, errSDESMissingType
	}

	rawPacket := make([]byte, sdesTypeLen+sdesOctetCountLen)

	rawPacket[sdesTypeOffset] = uint8(s.Type)

	txtBytes := []byte(s.Text)
	octetCount := len(txtBytes)
	if octetCount > sdesMaxOctetCount {
		return nil, errSDESTextTooLong
	}
	rawPacket[sdesOctetCountOffset] = uint8(octetCount)

	rawPacket = append(rawPacket, txtBytes...)

	return rawPacket, nil
}

// Unmarshal decodes the SourceDescriptionItem from binary
func (s *SourceDescriptionItem) Unmarshal(rawPacket []byte) error {
	/*
	 *   0                   1                   2                   3
	 *   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |    CNAME=1    |     length    | user and domain name        ...
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if len(rawPacket) < (sdesTypeLen + sdesOctetCountLen) {
		return errPacketTooShort
	}

	s.Type = SDESType(rawPacket[sdesTypeOffset])

	octetCount := int(rawPacket[sdesOctetCountOffset])
	if sdesTextOffset+octetCount > len(rawPacket) {
		return errPacketTooShort
	}

	txtBytes := rawPacket[sdesTextOffset : sdesTextOffset+octetCount]
	s.Text = string(txtBytes)

	return nil
}
