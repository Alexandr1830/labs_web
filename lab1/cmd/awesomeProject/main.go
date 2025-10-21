package main

import (
	"log"
	"lab1/internal/api"
)

func main() {
	log.Print("Application start")
	api.StartServer()
	log.Println("Application terminated")
}
