package webSocket

import "github.com/gorilla/websocket"

type wsServer struct {
	Addr 		string 		// websocket server address
	upgrader 	*websocket.Upgrader
}
