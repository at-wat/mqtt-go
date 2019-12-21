package mqtt

type pktPubRel struct {
	ID uint16
}

func (p *pktPubRel) parse(flag byte, contents []byte) *pktPubRel {
	_, p.ID = unpackUint16(contents)
	return p
}
