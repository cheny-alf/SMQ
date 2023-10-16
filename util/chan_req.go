package util

type ChanReq struct {
	Variable interface{}
	RetChan  chan interface{}
}

type ChaneRet struct {
	Variable interface{}
	Err      error
}
