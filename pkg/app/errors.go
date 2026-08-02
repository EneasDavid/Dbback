package app

import (
	"fmt"
	"strings"
)

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

// MissingHeaderError sinaliza que uma coluna obrigatoria nao foi encontrada
// na aba indicada - usado no lugar de adivinhar a coluna por posicao ou por
// aproximacao de texto.
func MissingHeaderError(sheetName string, headerNames ...string) HTTPError {
	return NewHTTPError(500, "cabeçalho obrigatório ausente: "+strings.Join(headerNames, " ou ")+" na aba "+sheetName)
}
