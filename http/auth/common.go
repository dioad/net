package auth

import (
	"net/http"
)

type ClientAuth interface {
	AddAuth(*http.Request) error
}
