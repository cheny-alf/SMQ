package message

import (
	"SMQ/util"
	"errors"
	"log"
	"time"
)

type Consumer interface {
	Close()
}

type Channel struct {
	name             string
	addClientChan    chan util.ChanReq
	removeClientChan chan util.ChanReq
	clients          []Consumer

	msgChan             chan *Message
	incomingMessageChan chan *Message
	clientMessageChan   chan *Message

	exitChan chan util.ChanReq

	inFlightMessageChan chan *Message
	inFlightMessages    map[string]*Message

	finishMessageChan chan util.ChanReq

	requeueMessageChan chan util.ChanReq
}

func NewChannel(name string, inMemSize int) *Channel {
	channel := &Channel{
		name:                name,
		addClientChan:       make(chan util.ChanReq),
		removeClientChan:    make(chan util.ChanReq),
		clients:             make([]Consumer, 0, 5),
		incomingMessageChan: make(chan *Message, 5),
		msgChan:             make(chan *Message, inMemSize),
		clientMessageChan:   make(chan *Message),
		exitChan:            make(chan util.ChanReq),
		inFlightMessageChan: make(chan *Message),
		inFlightMessages:    make(map[string]*Message),
		requeueMessageChan:  make(chan util.ChanReq),
		finishMessageChan:   make(chan util.ChanReq),
	}
	go channel.Router()
	return channel
}

func (c *Channel) AddClient(client Consumer) {
	log.Printf("Channel[%s]: adding client...", c.name)
	doneChan := make(chan interface{})
	c.addClientChan <- util.ChanReq{
		Variable: client,
		RetChan:  doneChan,
	}
	<-doneChan
}

func (c *Channel) RemoveClient(client Consumer) {
	log.Printf("Channel[%s]: removing client...", c.name)
	doneChan := make(chan interface{})
	c.removeClientChan <- util.ChanReq{
		Variable: client,
		RetChan:  doneChan,
	}
	<-doneChan
}

func (c *Channel) Router() {
	var clientReq util.ChanReq
	closeChan := make(chan struct{})

	go c.RequeueRouter(closeChan)
	go c.MessagePump(closeChan)

	for {
		select {
		case clientReq = <-c.addClientChan:
			client := clientReq.Variable.(Consumer)
			c.clients = append(c.clients, client)
			log.Printf("Channle[%s] added client %#v", c.name, client)
			clientReq.RetChan <- struct{}{}
		case clientReq = <-c.removeClientChan:
			client := clientReq.Variable.(Consumer)
			indexToRemove := -1
			for k, v := range c.clients {
				if v == client {
					indexToRemove = k
					break
				}
			}
			if indexToRemove == -1 {
				log.Printf("Error: could not find client[%#v] in clients[%#v]", client, c.clients)
			} else {
				c.clients = append(c.clients[:indexToRemove], c.clients[indexToRemove+1:]...)
				log.Printf("Channel[%s] removed client %#v", c.name, client)
			}
			clientReq.RetChan <- struct{}{}
		case msg := <-c.incomingMessageChan:
			select {
			case c.msgChan <- msg:
				log.Printf("Channel[%s] wrote message", c.name)
			default:

			}
		case closeReq := <-c.exitChan:
			log.Printf("Channel[%s] is closing", c.name)
			close(closeChan)

			for _, consumer := range c.clients {
				consumer.Close()
			}

			closeReq.RetChan <- nil
		}
	}
}

func (c *Channel) PutMessage(msg *Message) {
	c.incomingMessageChan <- msg
}

func (c *Channel) PullMessage() *Message {
	return <-c.clientMessageChan
}

func (c *Channel) MessagePump(closeChan chan struct{}) {
	var msg *Message

	for {
		select {
		case msg = <-c.msgChan:
		case <-closeChan:
			return
		}
		if msg != nil {
			c.inFlightMessageChan <- msg
		}
		c.clientMessageChan <- msg
	}
}

func (c *Channel) Close() error {
	errChan := make(chan interface{})
	c.exitChan <- util.ChanReq{
		RetChan: errChan,
	}

	err, _ := (<-errChan).(error)
	return err
}

func (c *Channel) pushInFightMessage(msg *Message) {
	c.inFlightMessages[util.UUID2String(msg.UUID())] = msg
}

func (c *Channel) popInFlightMessage(uuidStr string) (*Message, error) {
	msg, ok := c.inFlightMessages[uuidStr]
	if !ok {
		return nil, errors.New("UUID not in flight")
	}
	delete(c.inFlightMessages, uuidStr)
	msg.EndTimer()
	return msg, nil
}

func (c *Channel) RequeueRouter(closeChan chan struct{}) {
	for {
		select {
		case msg := <-c.inFlightMessageChan:
			c.pushInFightMessage(msg)
			go func(msg *Message) {
				select {
				case <-time.After(60 * time.Second):
					log.Printf("Channel[%s]: auto requeue of message[%s]", c.name, util.UUID2String(msg.UUID()))
				case <-msg.timerChan:
					return
				}
				err := c.RequeueMessage(util.UUID2String(msg.UUID()))
				if err != nil {
					log.Printf("Error: Channel[%s] - %s", c.name, err.Error())
				}
			}(msg)
		case requeueReq := <-c.requeueMessageChan:
			uuidStr := requeueReq.Variable.(string)
			msg, err := c.popInFlightMessage(uuidStr)
			if err != nil {
				log.Printf("Error: failed to requeue message[%s] - %s", uuidStr, err)
			} else {
				go func(msg *Message) {
					c.PutMessage(msg)
				}(msg)
			}
			requeueReq.RetChan <- err
		case finishReq := <-c.finishMessageChan:
			uuidStr := finishReq.Variable.(string)
			_, err := c.popInFlightMessage(uuidStr)
			if err != nil {
				log.Printf("Error: failed to finish message[%s] - %s", uuidStr, err)
			}
			finishReq.RetChan <- err
		case <-closeChan:
			return
		}
	}
}

func (c *Channel) FinishMessage(uuidStr string) error {
	errChan := make(chan interface{})
	c.finishMessageChan <- util.ChanReq{
		Variable: uuidStr,
		RetChan:  errChan,
	}
	err, _ := (<-errChan).(error)
	return err
}

func (c *Channel) RequeueMessage(uuidStr string) error {
	errChan := make(chan interface{})
	c.requeueMessageChan <- util.ChanReq{
		Variable: uuidStr,
		RetChan:  errChan,
	}
	err, _ := (<-errChan).(error)
	return err
}
