package apperr

type UsageError struct {
	Message string
}

func (e UsageError) Error() string {
	return e.Message
}
