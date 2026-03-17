package apperr

type BaseError struct {
	ErrorType ErrorType
	Reason    string
	HINT      string
	Cause     error
}

func (e *BaseError) Error() string {
	return e.Reason
}

func (e *BaseError) Type() ErrorType {
	return e.ErrorType
}

func (e *BaseError) Hint() string {
	return e.HINT
}

func (e *BaseError) Unwrap() error {
	return e.Cause
}
