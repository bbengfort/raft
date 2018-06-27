package main

import (
	"log"

	"github.com/bbengfort/raft"
)

func main() {
	replica, err := raft.New(&raft.Config{LogLevel: 2})
	if err != nil {
		log.Fatal(err)
	}
	if err = replica.Listen(); err != nil {
		log.Fatal(err)
	}
}
