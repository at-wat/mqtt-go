package mqtt

type pktPubRec struct {
	ID uint16
}

func (p *pktPubRec) parse(flag byte, contents []byte) (*pktPubRec, error) {
	if flag != 0 {
		return nil, ErrInvalidPacket
	}
	_, p.ID = unpackUint16(contents)
	return p, nil
}
