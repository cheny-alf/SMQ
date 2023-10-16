package message

import (
	"SMQ/util"
	"log"
)

type Topic struct {
	name                string
	newChannelChan      chan util.ChanReq
	channelMap          map[string]*Channel
	incomingMessageChan chan *Message
	msgChan             chan *Message
	readSyncChan        chan struct{}
	routerSyncChan      chan struct{}
	exitChan            chan util.ChanReq
	channelWriteStarted bool
}

var (
	TopicMap     = make(map[string]*Topic)
	newTopicChan = make(chan util.ChanReq)
)

func NewTopic(name string, inMemSize int) *Topic {
	topic := &Topic{
		name:                name,
		newChannelChan:      make(chan util.ChanReq),
		channelMap:          make(map[string]*Channel),
		incomingMessageChan: make(chan *Message),
		msgChan:             make(chan *Message, inMemSize),
		readSyncChan:        make(chan struct{}),
		routerSyncChan:      make(chan struct{}),
		exitChan:            make(chan util.ChanReq),
	}
	go topic.Router(inMemSize)
	return topic
}

func GetTopic(name string) *Topic {
	topicChan := make(chan interface{})
	newTopicChan <- util.ChanReq{
		Variable: name,
		RetChan:  topicChan,
	}
	return (<-topicChan).(*Topic)
}

func TopicFactory(inMemSize int) {
	for {
		topicReq := <-newTopicChan
		name := topicReq.Variable.(string)
		if topic, ok := TopicMap[name]; !ok {
			topic = NewTopic(name, inMemSize)
			TopicMap[name] = topic
			log.Printf("Topic %s created", name)
		}
		topicReq.RetChan <- topicReq
	}
}

func (t *Topic) Router(inMemSize int) {
	var msg *Message
	closeChan := make(chan struct{})
	for {
		select {
		case channelReq := <-t.newChannelChan:
			channelName := channelReq.Variable.(string)
			channel, ok := t.channelMap[channelName]
			if !ok {
				channel = NewChannel(channelName, inMemSize)
				t.channelMap[channelName] = channel
				log.Printf("Topic[%s]: new channel[%s]", t.name, channel.name)
			}
			channelReq.RetChan <- channel
			if !t.channelWriteStarted {
				go t.MessagePump(closeChan)
				t.channelWriteStarted = true
			}
		case msg = <-t.incomingMessageChan:
			select {
			case t.msgChan <- msg:
				log.Printf("Topic[%s] wrote message", t.name)
			default:
			}
		case <-t.readSyncChan:
			<-t.routerSyncChan
		case closeReq := <-t.exitChan:
			log.Printf("Topic[%s]: closeing", t.name)
			for _, channel := range t.channelMap {
				err := channel.Close()
				if err != nil {
					log.Printf("Error: channel[%s] close - %s", channel.name, err.Error())
				}
			}
			close(closeChan)
			closeReq.RetChan <- nil
		}
	}

}

func (t *Topic) PutMessage(msg *Message) {
	t.incomingMessageChan <- msg
}

func (t *Topic) MessagePump(closeChan <-chan struct{}) {
	var msg *Message
	for {
		select {
		case msg = <-t.msgChan:
		case <-closeChan:
			return
		}
		t.readSyncChan <- struct{}{}
		for _, channel := range t.channelMap {
			go func(ch *Channel) {
				ch.PutMessage(msg)
			}(channel)
		}
		t.routerSyncChan <- struct{}{}
	}
}

func (t *Topic) Close() error {
	errChan := make(chan interface{})
	t.exitChan <- util.ChanReq{
		RetChan: errChan,
	}
	err, _ := (<-errChan).(error)
	return err
}
