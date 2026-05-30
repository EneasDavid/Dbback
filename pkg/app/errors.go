package app

import "fmt"

type HTTPError struct {
	Status  int
	Message string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", e.Status, e.Message)
}

func NewHTTPError(status int, message string) HTTPError {
	return HTTPError{Status: status, Message: message}
}
