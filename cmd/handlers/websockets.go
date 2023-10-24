package handlers

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/env"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/Mind-thatsall/fiber-htmx/public/protobuf"
	"github.com/gocql/gocql"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/log"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/proto"
)

var Connections = make(map[gocql.UUID]*websocket.Conn)

type receivedMessage struct {
	Type     string `json:"type"`
	ServerId string `json:"server_id"`
	Position string `json:"pos"`
}

func Connect(c *websocket.Conn) {
	// c.Locals is added to the *websocket.Conn
	//
	var newMsg receivedMessage

	var (
		msg          []byte
		errWebsocket error
	)

	userId, errWebsocket := CheckConnectionWebsocket(env.Variable("SECRET"), c)
	if errWebsocket != nil {
		c.Close()
		return
	}

	defer func() {
		c.Close()
	}()

	userUUID, _ := gocql.ParseUUID(userId.(string))
	Connections[userUUID] = c

	for {
		if _, msg, errWebsocket = c.ReadMessage(); errWebsocket != nil {
			log.Info("read:", errWebsocket)
			break
		}

		err := json.Unmarshal(msg, &newMsg)
		if err != nil {
			log.Errorf("error occurred while decoding message: %v", err)
			continue
		}
		log.Info("recv: ", newMsg)
		if newMsg.Type == "initial" {
			message, err := getInformations(userUUID, newMsg.Position)
			if err != nil {
				log.Error(err)
				return
			}

			messageToSend := &protobuf.ServerMessage{
				Type:    "initial",
				Payload: message,
			}
			data, err := proto.Marshal(messageToSend)

			if err != nil {
				fmt.Println("Error when transforming the message into protobuf", err)
			}
			c.WriteMessage(websocket.BinaryMessage, data)

		} else if newMsg.Type == "change_server" {
			channels, err := getInfos(newMsg.ServerId)
			if err != nil {
				log.Error(err)
			}

			messageToSend := &protobuf.ServerMessage{
				Type: "change_server",
				Payload: &protobuf.ServerMessage_ChangeServer{
					ChangeServer: &protobuf.ChangeServer{
						Server: channels,
					},
				},
			}

			data, err := proto.Marshal(messageToSend)
			if err != nil {
				fmt.Println("Error when transforming the message into protobuf", err)
			}

			c.WriteMessage(websocket.BinaryMessage, data)
		}
	}
}

func CheckConnectionWebsocket(secretKey string, c *websocket.Conn) (interface{}, error) {
	cookie := c.Cookies("session")
	db := database.DB
	var userId interface{}

	token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("Wrong signature")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		var session models.Session

		userId = claims["user_id"]
		querySession := "SELECT * FROM sessions WHERE session_id = ? AND user_id = ? AND timezone = ? AND user_agent = ?"
		if err := db.Query(querySession, claims["session_id"], claims["user_id"], claims["timezone"], claims["user_agent"]).Scan(&session.SessionId, &session.UserId, &session.Timezone, &session.UserAgent); err != nil {
			log.Error(err)
			return nil, fmt.Errorf("Session non existant")
		}
	} else {
		return nil, fmt.Errorf("Token have no claims")
	}

	return userId, nil
}

func getInformations(userId gocql.UUID, pos string) (*protobuf.ServerMessage_InitialLoad, error) {
	db := database.DB
	var serversId []string
	var servers []*protobuf.Server
	var user protobuf.User
	message := &protobuf.ServerMessage_InitialLoad{
		InitialLoad: &protobuf.InitialLoad{},
	}

	queryGetUser := "SELECT id, about, avatar, banner, displayname, email, username FROM users WHERE id = ?"
	errUser := db.Query(queryGetUser, userId).Scan(&user.Id, &user.About, &user.Avatar, &user.Banner, &user.DisplayName, &user.Email, &user.Username)
	if errUser != nil {
		log.Error(errUser)
		return nil, fmt.Errorf("Impossible to fetch servers")
	}

	queryGetServers := "SELECT servers FROM user_to_servers WHERE user_id = ?"
	errServers := db.Query(queryGetServers, userId).Scan(&serversId)
	if errServers != nil {
		log.Error(errServers)
		return nil, fmt.Errorf("Impossible to fetch servers")
	}

	queryGetServer := "SELECT server_id, banner, description, name, owner, status FROM servers WHERE server_id = ?"
	for _, id := range serversId {
		var server protobuf.Server
		err := db.Query(queryGetServer, id).Scan(&server.ServerId, &server.Banner, &server.Description, &server.Name, &server.Owner, &server.Status)
		if err != nil {
			log.Error(err)
			return nil, fmt.Errorf("Impossible to the server")
		}

		servers = append(servers, &server)
	}

	message.InitialLoad.User = &user
	message.InitialLoad.Servers = servers
	if pos != "me" {
		channels, err := getInfos(pos)
		if err != nil {
			return nil, err
		}
		message.InitialLoad.Server = channels
	}

	return message, nil

}

type ChannelTest struct {
	ServerId       string `db:"server_id" json:"serverId"`
	ChannelId      string `db:"channel_id" json:"channelId"`
	Category       string `db:"group" json:"group"`
	Name           string `db:"name" json:"name"`
	ParentId       string `db:"parent_id" json:"parentId"`
	ParentPosition int    `db:"parent_position" json:"parentPosition"`
	Position       int    `db:"position" json:"position"`
	Status         string `db:"status" json:"status"`
	Type           string `db:"type" json:"type"`
}

type Category struct {
	Name     string        `json:"groupName"`
	Channels []ChannelTest `json:"channels"`
}

func getInfos(serverId string) (*protobuf.ServerInfos, error) {
	db := database.DB
	var sortedChannels []*protobuf.Channel
	message := &protobuf.ServerInfos{
		Categories: []*protobuf.Categories{},
	}

	queryAllChannels := "SELECT * FROM channels_test WHERE server_id = ?"
	scanner := db.Query(queryAllChannels, serverId).Iter().Scanner()
	for scanner.Next() {
		channel := &protobuf.Channel{}
		err := scanner.Scan(&channel.ServerId, &channel.ChannelId, &channel.Category, &channel.Name, &channel.ParentId, &channel.ParentPosition, &channel.Position, &channel.Status, &channel.Type)
		if err != nil {
			log.Error(err)
			return nil, fmt.Errorf("Error when fetching the channels")
		}

		sortedChannels = append(sortedChannels, channel)
	}

	sort.Slice(sortedChannels, func(i, j int) bool {
		if sortedChannels[i].ParentPosition == sortedChannels[j].ParentPosition {
			return sortedChannels[i].Position < sortedChannels[j].Position
		}
		return sortedChannels[i].ParentPosition < sortedChannels[j].ParentPosition
	})

	var channels []*protobuf.Channel
	if len(sortedChannels) > 0 {
		currentCat := sortedChannels[0].Category
		for i := range sortedChannels {
			// If new category starts, append existing one to categories and start a new one.
			if sortedChannels[i].Category != currentCat {
				message.Categories = append(message.Categories, &protobuf.Categories{GroupName: currentCat, Channels: channels})
				currentCat = sortedChannels[i].Category
				channels = nil // Reset channels slice
			}
			channels = append(channels, sortedChannels[i])
		}
		message.Categories = append(message.Categories, &protobuf.Categories{GroupName: currentCat, Channels: channels})
	}

	return message, nil
}

//			if channel.Status == "private" {
//				queryPrivateChannel := "SELECT * FROM private_channels WHERE channel_id = ? AND id = ?"
//				var privateChannel PrivateChannel
//
//				if err := db.Query(queryPrivateChannel, channel.ChannelId, userId).Scan(&privateChannel.ChannelId, &privateChannel.Id, &privateChannel.Type); err != nil {
//					log.Errorf("User not authorized in this channel")
//				} else {
//					channelsCategory.Channels = append(channelsCategory.Channels, &channel)
//				}
//			} else {
//				channelsCategory.Channels = append(channelsCategory.Channels, &channel)
//			}
