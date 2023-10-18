package protocol

import "io"

const (
	ClientInit = iota
	ClientWaitGet
	ClientWaitResponse
)

type StatefulReadWriter interface {
	io.ReadWriteCloser
	GetState() int
	SetState(state int)
	String() string
}

type ClientError struct {
	errStr string
}

func (e ClientError) Error() string {
	return e.errStr
}

var (
	ClientErrInvalid    = ClientError{errStr: "E_INVALID"}
	ClientErrBadTopic   = ClientError{errStr: "E_BAD_TOPIC"}
	ClientErrBadChannel = ClientError{errStr: "E_BAD_CHANNEL"}
	ClientErrBadMessage = ClientError{errStr: "E_BAD_MESSAGE"}
)
