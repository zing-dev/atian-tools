package main

import (
	"github.com/zing-dev/atian-tools/protocol/tcp/haosen"
	"testing"
)

func TestServer(t *testing.T) {
	server := haosen.NewServer()
	server.Run()
}
