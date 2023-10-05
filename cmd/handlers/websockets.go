package handlers

import (
	"log"

	"github.com/gocql/gocql"
	"github.com/gofiber/contrib/websocket"
)

var Connections = make(map[gocql.UUID]*websocket.Conn)

func Connect(c *websocket.Conn) {
	// c.Locals is added to the *websocket.Conn
	userIdString := c.Params("userId")
	userIdUUID, _ := gocql.ParseUUID(userIdString)

	Connections[userIdUUID] = c

	var (
		msg []byte
		err error
	)

	defer func() {
		c.Close()
		delete(Connections, userIdUUID)
	}()

	//message := &protobuf.MyMessage{
	//	Id:       1,
	//	Username: "John",
	//	Text:     "tezsjol",
	//}

	//data, err := proto.Marshal(message)
	//if err != nil {
	//	fmt.Println("error")
	//}

	//c.WriteMessage(websocket.BinaryMessage, data)

	for {
		if _, msg, err = c.ReadMessage(); err != nil {
			log.Println("read:", err)
			break
		}

		log.Printf("recv: %s", msg)
	}
}
