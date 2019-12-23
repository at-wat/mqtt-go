package mqtt

type pktUnsubAck struct {
	ID uint16
}

func (p *pktUnsubAck) parse(flag byte, contents []byte) (*pktUnsubAck, error) {
	if flag != 0 {
		return nil, ErrInvalidPacket
	}
	p.ID = uint16(contents[0])<<8 | uint16(contents[1])
	return p, nil
}
