package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"webSocket"
)

func main(){
	wh := webSocket.NewConnectionManager()
	wh.SetMessageHandler(func(message *webSocket.Message){
		fmt.Println(message)
		return
	})
	wh.SetPingPeriod(10*time.Second)
	wh.SetPongPeriod(15*time.Second)
	http.HandleFunc("/websocket", wh.HandleFunc)
	log.Fatal(http.ListenAndServe(":8080",nil))
}
