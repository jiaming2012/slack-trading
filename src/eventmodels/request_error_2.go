package eventmodels

// todo: change to RequestError
type RequestError2 struct {
	Request interface{}
	Error   error
}

func NewRequestError2(req interface{}, err error) RequestError2 {
	return RequestError2{
		Request: req,
		Error:   err,
	}
}
