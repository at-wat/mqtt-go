package mqtt

type PubRel struct {
	ID uint16
}

func (p *PubRel) parse(flag byte, contents []byte) *PubRel {
	_, p.ID = unpackUint16(contents)
	return p
}
