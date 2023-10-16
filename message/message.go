package message

type Message struct {
	data      []byte // 前16位位uuid，后面为body
	timerChan chan struct{}
}

func NewMessage(data []byte) *Message {
	return &Message{data: data}
}

func (m *Message) UUID() []byte {
	return m.data[:16]
}

func (m *Message) Body() []byte {
	return m.data[16:]
}

func (m *Message) Data() []byte {
	return m.data
}

func (m *Message) EndTimer() {
	select {
	case m.timerChan <- struct{}{}:
	default:

	}
}
