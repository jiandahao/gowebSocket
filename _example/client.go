package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"sync"
)

func main(){
	client,_, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/websocket",nil)
	if err != nil{
		log.Fatal(err)
		return
	}
	defer client.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)
	//t := time.NewTimer(30*time.Second) // simulate error event
	go func(){
		for{
				select {
				//case <- t.C:
				//	wg.Done()
				//	return
				default:
				_,p,err := client.ReadMessage()
				if err != nil{
					log.Println("Read error:", err)
					wg.Done()
					return
				}
				fmt.Println(string(p))
				var msg map[string]interface{}
				_ = json.Unmarshal(p, &msg)
				if msg["msg"] == "ping"{
					//_ = client.WriteMessage(websocket.TextMessage, []byte(`{"msg":"pong"}`))
					continue
				}
			}
		}
	}()

	wg.Wait()
	fmt.Println("Read close")
}
