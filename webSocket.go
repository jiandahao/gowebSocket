package webSocket

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
	// message handler, default set as defaultMessageHandler
	// in this handler function, you don't need to handle ping/pong message
	MessageHandler  func(message *Message)
	wg 				sync.WaitGroup
	// heartbeat ping timer, if client haven't send any message to
	// server before next ping-timer comes, try to send a ping message
	// to make sure the client is still online
	pingTimer 		*time.Timer
	pingWait 		time.Duration
	// if client haven't response with a pong to ping message from server
	// before next pong-timer comes, server will close the connection
	pongTimer 		*time.Timer
	pongWait 		time.Duration
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

func defaultMessageHandler(message *Message){
	fmt.Println(message)
}

func NewConnection(c *websocket.Conn) *wsConnection{
	conn :=  &wsConnection{
		Conn:c,
		Send:make(chan []byte),
		wg:sync.WaitGroup{},
		MessageHandler:defaultMessageHandler,
		pingTimer:time.NewTimer(pingWait),
		pongTimer:time.NewTimer(pongWait),
	}
	//sets the handler for close messages received from the peer
	conn.Conn.SetCloseHandler(func(code int, text string) error {
		closeMessage := websocket.FormatCloseMessage(code,text)
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
			//TODO send a error message to client
			continue
		}

		if msg.Msg == "" {
			//TODO send a error message to client
			log.Println("Invalid request, empty msg")
			continue
		}

		// receive a pong message, stop the pong timer,
		// in case the connection will be closed
		if msg.Msg == "pong" {
			wc.pongTimer.Stop()
			continue
		}

		go wc.MessageHandler(&msg)
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
	wc.pongTimer.Stop()
	for {
		select {
		case <- wc.pingTimer.C:
			// time to send a ping message
			fmt.Println("ping")
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
			return
		case msg := <- wc.Send:
			_ = wc.Conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
}
func (wc *wsConnection) SendMessage( message []byte){
	wc.Send <- message
}

type ConnectionManager struct {
	// handler for processing websocket received message
	messageHandler    func(message *Message)
	// pongWait: period to wait pong response from client side
	pongWait 		  time.Duration
	// pingWait: period to send ping message to client side,
	// to figure out whether client is still alive
	pingWait 		  time.Duration
}

func (cm *ConnectionManager) SetMessageHandler(handler func(message *Message)){
	cm.messageHandler = handler
}

func (cm *ConnectionManager) SetPingPeriod(period time.Duration){
	cm.pingWait = period
}

// make sure the pong period is larger than ping period, otherwise unexpected
// error would occur
func (cm *ConnectionManager) SetPongPeriod(period time.Duration){
	cm.pongWait = period
}

func (cm *ConnectionManager) HandleFunc(w http.ResponseWriter, r *http.Request){
	//TODO add error handle details
	conn, err := (&websocket.Upgrader{}).Upgrade(w,r,nil)
	if err != nil{
		log.Println("Upgrader error: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Upgrader error :" + err.Error()))
		return
	}
	defer conn.Close()

	wsrvConn := NewConnection(conn)
	if  cm.messageHandler != nil{
		wsrvConn.MessageHandler = cm.messageHandler
	}

	if cm.pingWait > 0{
		wsrvConn.pingWait = cm.pingWait
	}

	if cm.pongWait > 0{
		wsrvConn.pongWait = cm.pongWait
	}

	// make sure the pong period larger than ping period
	if wsrvConn.pongWait < wsrvConn.pingWait{
		wsrvConn.pongWait = 2*wsrvConn.pingWait
	}

	wsrvConn.wg.Add(2)
	go wsrvConn.ReadMessage()
	go wsrvConn.WriteMessage()
	wsrvConn.wg.Wait()
	fmt.Println("WsHandler exit.....")
}
