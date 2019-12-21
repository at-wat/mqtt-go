package mqtt

type PubRel struct {
	ID uint16
}

func (p *PubRel) parse(flag byte, contents []byte) *PubRel {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
