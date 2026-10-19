package ws

import "github.com/gin-gonic/gin"

type Handshaker interface {
	// claims is the content of the auth
	Claims() any
	// handshake is how you wanna auth a user
	HandShake() error
	// get the id
	Id() string
}

type Auther interface {
	Check(ctx *gin.Context) (Handshaker, error)
}

var AuthHandler Auther

// DefaultHandshaker provides a basic handshaker for anonymous or unauthenticated connections.
type DefaultHandshaker struct {
	id string
}

func NewDefaultHandshaker(id string) *DefaultHandshaker {
	return &DefaultHandshaker{id: id}
}

func (d *DefaultHandshaker) Claims() any {
	return nil
}

func (d *DefaultHandshaker) HandShake() error {
	return nil
}

func (d *DefaultHandshaker) Id() string {
	return d.id
}
