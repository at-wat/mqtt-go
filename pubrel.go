package mqtt

type pktPubRel struct {
	ID uint16
}

func (p *pktPubRel) parse(flag byte, contents []byte) (*pktPubRel, error) {
	if flag != 0x02 {
		return nil, ErrInvalidPacket
	}
	_, p.ID = unpackUint16(contents)
	return p, nil
}
