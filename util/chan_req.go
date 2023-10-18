package util

type ChanReq struct {
	Variable interface{}
	RetChan  chan interface{}
}

type ChanRet struct {
	Variable interface{}
	Err      error
}
