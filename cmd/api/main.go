package main

import (
	"fmt"
	"idempot/internal/config"
	"log"
)

func main(){
	config, err := config.Load()
	if err != nil {
        log.Fatal("failed to load configuration:", err)
    }

	fmt.Println(config)
}