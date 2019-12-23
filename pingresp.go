package mqtt

type pktPingResp struct {
}

func (p *pktPingResp) parse(flag byte, contents []byte) (*pktPingResp, error) {
	if flag != 0 {
		return nil, ErrInvalidPacket
	}
	return p, nil
}
