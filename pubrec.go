package mqtt

type pktPubRec struct {
	ID uint16
}

func (p *pktPubRec) parse(flag byte, contents []byte) *pktPubRec {
	_, p.ID = unpackUint16(contents)
	return p
}
