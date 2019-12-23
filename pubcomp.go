package mqtt

type pktPubComp struct {
	ID uint16
}

func (p *pktPubComp) parse(flag byte, contents []byte) (*pktPubComp, error) {
	if flag != 0 {
		return nil, ErrInvalidPacket
	}
	_, p.ID = unpackUint16(contents)
	return p, nil
}
