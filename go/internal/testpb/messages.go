package testpb

type HelloRequest struct {
	Name string
}

func (m *HelloRequest) Marshal() []byte {
	var e Encoder
	e.EncodeString(1, m.Name)
	return e.Bytes()
}

func (m *HelloRequest) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.Name, err = d.ReadString()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type HelloReply struct {
	Message string
}

func (m *HelloReply) Marshal() []byte {
	var e Encoder
	e.EncodeString(1, m.Message)
	return e.Bytes()
}

func (m *HelloReply) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.Message, err = d.ReadString()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type CalcRequest struct {
	A float64
	B float64
}

func (m *CalcRequest) Marshal() []byte {
	var e Encoder
	e.EncodeDouble(1, m.A)
	e.EncodeDouble(2, m.B)
	return e.Bytes()
}

func (m *CalcRequest) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.A, err = d.ReadDouble()
		case 2:
			m.B, err = d.ReadDouble()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type CalcResponse struct {
	Result float64
	Error  string
}

func (m *CalcResponse) Marshal() []byte {
	var e Encoder
	e.EncodeDouble(1, m.Result)
	e.EncodeString(2, m.Error)
	return e.Bytes()
}

func (m *CalcResponse) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.Result, err = d.ReadDouble()
		case 2:
			m.Error, err = d.ReadString()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type ChatMessage struct {
	Sender    string
	Text      string
	Timestamp uint64
}

func (m *ChatMessage) Marshal() []byte {
	var e Encoder
	e.EncodeString(1, m.Sender)
	e.EncodeString(2, m.Text)
	e.EncodeUint64(3, m.Timestamp)
	return e.Bytes()
}

func (m *ChatMessage) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.Sender, err = d.ReadString()
		case 2:
			m.Text, err = d.ReadString()
		case 3:
			m.Timestamp, err = d.ReadVarint()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type SendResult struct {
	Success   bool
	MessageID string
}

func (m *SendResult) Marshal() []byte {
	var e Encoder
	e.EncodeBool(1, m.Success)
	e.EncodeString(2, m.MessageID)
	return e.Bytes()
}

func (m *SendResult) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			var v uint64
			v, err = d.ReadVarint()
			m.Success = v != 0
		case 2:
			m.MessageID, err = d.ReadString()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type HistoryRequest struct {
	Limit uint32
}

func (m *HistoryRequest) Marshal() []byte {
	var e Encoder
	e.EncodeUint32(1, m.Limit)
	return e.Bytes()
}

func (m *HistoryRequest) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			var v uint64
			v, err = d.ReadVarint()
			m.Limit = uint32(v)
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type HistoryResponse struct {
	Messages []ChatMessage
	Total    uint32
}

func (m *HistoryResponse) Marshal() []byte {
	var e Encoder
	for i := range m.Messages {
		e.EncodeBytes(1, m.Messages[i].Marshal())
	}
	e.EncodeUint32(2, m.Total)
	return e.Bytes()
}

func (m *HistoryResponse) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			var msgData []byte
			msgData, err = d.ReadBytes()
			if err != nil {
				return err
			}
			var msg ChatMessage
			if err = msg.Unmarshal(msgData); err != nil {
				return err
			}
			m.Messages = append(m.Messages, msg)
		case 2:
			var v uint64
			v, err = d.ReadVarint()
			m.Total = uint32(v)
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type PingRequest struct {
	SentAt uint64
}

func (m *PingRequest) Marshal() []byte {
	var e Encoder
	e.EncodeUint64(1, m.SentAt)
	return e.Bytes()
}

func (m *PingRequest) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.SentAt, err = d.ReadVarint()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type PongResponse struct {
	SentAt     uint64
	ReceivedAt uint64
}

func (m *PongResponse) Marshal() []byte {
	var e Encoder
	e.EncodeUint64(1, m.SentAt)
	e.EncodeUint64(2, m.ReceivedAt)
	return e.Bytes()
}

func (m *PongResponse) Unmarshal(data []byte) error {
	d := NewDecoder(data)
	for !d.Done() {
		num, wt, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch num {
		case 1:
			m.SentAt, err = d.ReadVarint()
		case 2:
			m.ReceivedAt, err = d.ReadVarint()
		default:
			err = d.Skip(wt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
