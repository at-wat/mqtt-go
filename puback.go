package mqtt

type pktPubAck struct {
	ID uint16
}

func (p *pktPubAck) parse(flag byte, contents []byte) *pktPubAck {
	_, p.ID = unpackUint16(contents)
	return p
}
