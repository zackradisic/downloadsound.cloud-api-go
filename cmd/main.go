package main

import "github.com/zackradisic/downloadsound.cloud-api-go/server"

func main() {
	s := server.New()

	s.Run(":8080")
}
