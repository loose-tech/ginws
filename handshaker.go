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
