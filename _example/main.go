package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)
type wsConnection struct {
	Conn 			*websocket.Conn
	Send 			chan []byte
	wg 				sync.WaitGroup
	// heartbeat ping timer, if client haven't send any message to
	// server before next ping-timer comes, try to send a ping message
	// to make sure the client is still online
	pingTimer 		*time.Timer
	// heartbeat pong timer, server will send a ping message to
	// verify whether the client is still alive. if client haven't
	// response with a pong before next pong-timer comes, server will
	// close the connection
	pongTimer 		*time.Timer
}

type Message struct {
	Msg 		string 			`json:"msg"` // request type
	Method 		string 			`json:"method,omitempty"` // if request type is method, a valid method name is required
	Uid 		string 			`json:"uid"` // unique id
	Params 		interface{}		`json:"params"` // parameters
}
var(
	pingWait = 10*time.Second
	pongWait = 15*time.Second // should be larger than pingWait
)
func NewClient(c *websocket.Conn) *wsConnection{
	conn :=  &wsConnection{
		Conn:c,
		Send:make(chan []byte),
		wg:sync.WaitGroup{},
		pingTimer:time.NewTimer(pingWait),
		pongTimer:time.NewTimer(pongWait),
	}
	//sets the handler for close messages received from the peer
	conn.Conn.SetCloseHandler(func(code int, text string) error {
		closeMessage := websocket.FormatCloseMessage(code,"")
		return conn.Conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(time.Second))
	})
	return conn
}

func (wc *wsConnection) ReadMessage(){
	defer func(){
		wc.wg.Done()
		fmt.Println("read close")
	}()
	for {
		wc.pingTimer.Reset(pingWait)
		_, p, err := wc.Conn.ReadMessage()
		if err != nil {
			log.Println("read message error: ", err)
			return
		}
		fmt.Println(string(p))
		var msg Message
		err = json.Unmarshal(p, &msg)
		if err != nil {
			log.Println("Invalid request, could not Unmarshal: ", err)
			closeMessage := websocket.FormatCloseMessage(websocket.ClosePolicyViolation,"Invalid argument")
			_ = wc.Conn.WriteControl(websocket.CloseNormalClosure, closeMessage, time.Now().Add(time.Second))
			continue
		}

		if msg.Msg == "" {
			//TODO return error info
			log.Println("Invalid request, empty msg")
			continue
		}

		if msg.Msg == "pong" {
			wc.pongTimer.Stop()
			continue
		}
	}
}

func (wc *wsConnection) WriteMessage(){
	defer func() {
		wc.pingTimer.Stop()
		wc.pongTimer.Stop()
		wc.wg.Done()
		fmt.Println("write close")
	}()
	wc.pingTimer.Reset(pingWait)
	for {
		select {
			case <- wc.pingTimer.C:
				// time to send a ping message
				err := wc.Conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("{\"msg\":\"ping\"}")))
				if err != nil{
					log.Println("Failed to write message to client: ", err)
					return
				}
				wc.pingTimer.Stop()
				wc.pongTimer.Reset(pongWait)
			case <- wc.pongTimer.C:
				log.Println("No heartbeat detects from client, just close the connection")
				// send a close message to client
				closeMessage:=websocket.FormatCloseMessage(websocket.CloseNormalClosure,"No heartbeat detects: Normal closure")
				_ = wc.Conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(time.Second))
				wc.Conn.Close()
				return
			case msg := <- wc.Send:
				_ = wc.Conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
}

func WsHandler(w http.ResponseWriter, r *http.Request){
	//TODO add error handle details
	conn, err := (&websocket.Upgrader{}).Upgrade(w,r,nil)
	if err != nil{
		log.Println("Upgrader error: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Upgrader error :" + err.Error()))
		return
	}
	defer conn.Close()

	client := NewClient(conn)

	client.wg.Add(2)
	go client.ReadMessage()
	go client.WriteMessage()
	client.wg.Wait()
	fmt.Println("WsHandler exit.....")
}

func main(){
	http.HandleFunc("/websocket", WsHandler)
	log.Fatal(http.ListenAndServe(":8080",nil))
}
