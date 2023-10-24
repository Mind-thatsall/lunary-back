package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/Mind-thatsall/fiber-htmx/cmd/utils"
	"github.com/Mind-thatsall/fiber-htmx/public/protobuf"
	"github.com/gocql/gocql"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"google.golang.org/protobuf/proto"
)

type Invitation struct {
	Id   string      `json:"invitation_id"`
	User models.User `json:"user"`
}

type Deletion struct {
	Query string
	Args  []interface{}
}

var rollback []Deletion

func JoinServer(c *fiber.Ctx) error {
	db := database.DB
	var serverId string
	var serverInformations models.Server
	var channels []string
	var Invitation Invitation
	userId := c.Locals("user_id").(string)

	err := c.BodyParser(&Invitation)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when joining the server"})
	}

	queryCheckInvitation := "SELECT server_id FROM invitations WHERE id = ?"
	if err := db.Query(queryCheckInvitation, Invitation.Id).Scan(&serverId); err != nil {
		log.Error(err)
		return c.Status(404).JSON(fiber.Map{"error": "Invitation doesn't exist"})
	}

	queryGetAllChannels := "SELECT channel_id FROM channels WHERE server_id = ?"
	scanner := db.Query(queryGetAllChannels, serverId).Iter().Scanner()
	for scanner.Next() {
		var channelId string
		err := scanner.Scan(&channelId)
		if err != nil {
			log.Error(err)
		}
		channels = append(channels, channelId)
	}

	queryJoinChannel := "INSERT INTO channel_to_users (channel_id, user_id) VALUES (?, ?)"
	for _, channelId := range channels {
		if err := db.Query(queryJoinChannel, channelId, userId).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
		}
	}

	queryJoinServer := "UPDATE server_to_users SET users = users + ? WHERE server_id = ?"
	if err := db.Query(queryJoinServer, []string{userId}, serverId).Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	queryJoinUsersServerList := "UPDATE user_to_servers SET servers = servers + ? WHERE user_id = ?"
	if err := db.Query(queryJoinUsersServerList, []string{serverId}, userId).Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	queryAddUserServerState := "INSERT INTO user_to_server_state (user_id, server_id, last_channel_id) VALUES (?, ?, ?)"
	if err := db.Query(queryAddUserServerState, userId, serverId, channels[0]).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	queryServerInformations := "SELECT * FROM servers WHERE server_id = ?"
	if err := db.Query(queryServerInformations, serverId).Scan(&serverInformations.ServerId, &serverInformations.CreatedAt, &serverInformations.Banner, &serverInformations.Description, &serverInformations.Name, &serverInformations.Owner, &serverInformations.Status); err != nil {
		log.Error(err)
	}

	var users []gocql.UUID
	queryGetUsersOfServer := "SELECT users FROM server_to_users WHERE server_id = ?"
	if err := db.Query(queryGetUsersOfServer, serverId).Scan(&users); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	broadcastServerChanges(users, Options{UserId: &userId, Server: &serverInformations}, "server_join")

	return nil
}

func GetServersOfUser(c *fiber.Ctx) error {
	db := database.DB
	var serverIds []string
	var serversInformations []models.Server
	var userId gocql.UUID

	userClaim := c.Locals("user_id").(string)
	userId, err := gocql.ParseUUID(userClaim)
	if err != nil {
		log.Error("Error when parsing to UUID", err)
	}

	querySub := "SELECT servers FROM user_to_servers WHERE user_id = ?"
	if err := db.Query(querySub, userId).Scan(&serverIds); err != nil {
		log.Error(err)
	}

	for _, serverId := range serverIds {
		server, err := utils.GetServerInformations(serverId)
		if err != nil {
			log.Error(err)
		}
		serversInformations = append(serversInformations, server)
	}

	return c.JSON(serversInformations)
}

type channelMap struct {
	GroupId  gocql.UUID       `json:"groupId"`
	Channels []models.Channel `json:"channels"`
}

type ChannelsByCategory map[string]channelMap

type PrivateChannel struct {
	ChannelId string     `db:"channel_id"`
	Id        gocql.UUID `db:"id"`
	Type      string     `db:"type"`
}

func GetChannelsFromServer(c *fiber.Ctx) error {
	db := database.DB
	channels := make(ChannelsByCategory)
	serverId := c.Params("serverId")
	userId := c.Locals("user_id")

	queryAllCategory := "SELECT * FROM categories WHERE server_id = ?"
	scannerCategory := db.Query(queryAllCategory, serverId).Iter().Scanner()
	for scannerCategory.Next() {
		var category models.Category
		err := scannerCategory.Scan(&category.ServerId, &category.CategoryId, &category.Name)
		if err != nil {
			log.Error(err)
			return c.Status(404).JSON(fiber.Map{"error": "Error when fetching category"})
		}

		channels[category.Name] = channelMap{
			GroupId:  category.CategoryId,
			Channels: nil,
		}
	}

	if err := scannerCategory.Err(); err != nil {
		log.Error(err)
	}

	querySub := "SELECT * FROM channels WHERE server_id = ?"
	queryCategory := "SELECT * FROM categories WHERE server_id = ? AND category_id = ?"
	scanner := db.Query(querySub, serverId).Iter().Scanner()
	for scanner.Next() {
		var channel models.Channel
		err := scanner.Scan(&channel.ServerId, &channel.ChannelId, &channel.Category, &channel.Name, &channel.Status, &channel.Type)
		if err != nil {
			log.Error(err)
		}

		var category models.Category
		if err := db.Query(queryCategory, serverId, channel.Category).Scan(&category.ServerId, &category.CategoryId, &category.Name); err != nil {
			log.Error(err)
			return c.Status(404).JSON(fiber.Map{"error": "Error when fetching category"})
		}

		channelsCategory := channels[category.Name]

		if channel.Status == "private" {
			queryPrivateChannel := "SELECT * FROM private_channels WHERE channel_id = ? AND id = ?"
			var privateChannel PrivateChannel

			if err := db.Query(queryPrivateChannel, channel.ChannelId, userId).Scan(&privateChannel.ChannelId, &privateChannel.Id, &privateChannel.Type); err != nil {
				log.Errorf("User not authorized in this channel")
			} else {
				channelsCategory.Channels = append(channelsCategory.Channels, channel)
			}
		} else {
			channelsCategory.Channels = append(channelsCategory.Channels, channel)
		}

		channels[category.Name] = channelsCategory
	}

	if err := scanner.Err(); err != nil {
		log.Error(err)
	}

	return c.JSON(channels)
}

func UpdateServerState(c *fiber.Ctx) error {
	db := database.DB
	var servers_state models.ServerState
	user_id := c.Locals("user_id")

	err := c.BodyParser(&servers_state)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the servers state"})
	}

	for key, value := range servers_state {
		q := db.Query("UPDATE user_to_server_state SET last_channel_id = ? WHERE server_id = ? AND user_id = ?", value, key, user_id)
		if err := q.Exec(); err != nil {
			log.Errorf("Error when updating server_state: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Couldn't update your servers state"})
		}
	}

	return nil
}

func GetServerState(c *fiber.Ctx) error {
	db := database.DB
	servers_state := make(models.ServerState)
	user_id := c.Locals("user_id")

	var serverID, channelID string

	querySub := "SELECT server_id, last_channel_id FROM user_to_server_state WHERE user_id = ?"
	scanner := db.Query(querySub, user_id).Iter()

	for scanner.Scan(&serverID, &channelID) {
		servers_state[serverID] = channelID
	}

	if err := scanner.Close(); err != nil {
		log.Errorf("Error when fetching server_state: %v", err)
	}

	return c.JSON(servers_state)
}

func CreateServer(c *fiber.Ctx) error {
	db := database.DB
	userId := c.Locals("user_id").(string)
	var server models.Server

	err := c.BodyParser(&server)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the server's informations"})
	}

	channelId := utils.GenerateNanoid()
	serverId := utils.GenerateNanoid()

	server.ServerId = serverId
	server.Banner = "https://d2b2cq6cks3j1i.cloudfront.net/server_banner/banner_" + serverId + "_v1.webp"

	t := time.Now()

	queryCreateServer := "INSERT INTO servers (server_id, created_at, banner, description, name, owner, status) VALUES (?, ?, ?, ?, ?, ?, ?)"
	if err := db.Query(queryCreateServer, server.ServerId, t, server.Banner, server.Description, server.Name, userId, server.Status).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create the server"})
	} else {
		rollback = append(rollback, Deletion{Query: "DELETE FROM servers WHERE server_id = ?", Args: []interface{}{
			server.ServerId,
		}})
	}

	categoryId, err := gocql.RandomUUID()
	if err != nil {
		log.Error(err)
	}

	queryCreateCategory := "INSERT INTO categories (server_id, category_id, name) VALUES (?, ?, ?)"
	if err := db.Query(queryCreateCategory, serverId, categoryId, "Home").Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create a new channel"})
	} else {
		rollback = append(rollback, Deletion{Query: "DELETE FROM categories WHERE server_id = ?", Args: []interface{}{
			server.ServerId,
		}})
	}

	queryCreateChannel := "INSERT INTO channels (server_id, channel_id, group, name, status, type) VALUES (?, ?, ?, ?, ?, ?)"
	if err := db.Query(queryCreateChannel, serverId, channelId, categoryId, "General", "public", "textual").Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create a new channel"})
	} else {
		rollback = append(rollback, Deletion{Query: "DELETE FROM channels WHERE server_id = ?", Args: []interface{}{
			server.ServerId,
		}})
	}

	queryJoinChannel := "INSERT INTO channel_to_users (channel_id, user_id) VALUES (?, ?)"
	if err := db.Query(queryJoinChannel, channelId, userId).Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	} else {
		rollback = append(rollback, Deletion{Query: "DELETE FROM channel_to_users WHERE channel_id = ?", Args: []interface{}{
			channelId,
		}})
	}

	queryJoinServer := "INSERT INTO server_to_users (server_id, users) VALUES (?, ?)"
	if err := db.Query(queryJoinServer, server.ServerId, []string{userId}).Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	} else {
		rollback = append(rollback, Deletion{Query: "DELETE FROM server_to_users WHERE server_id = ?", Args: []interface{}{
			server.ServerId,
		}})
	}

	queryJoinUserServerList := "UPDATE user_to_servers SET servers = servers + ? WHERE user_id = ?"
	if err := db.Query(queryJoinUserServerList, []string{server.ServerId}, userId).Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	} else {
		rollback = append(rollback, Deletion{Query: "UPDATE users_to_servers SET servers = servers - ? WHERE user_id = ?", Args: []interface{}{
			[]string{server.ServerId},
			userId,
		}})
	}

	queryAddUserServerState := "INSERT INTO user_to_server_state (user_id, server_id, last_channel_id) VALUES (?, ?, ?)"
	if err := db.Query(queryAddUserServerState, userId, server.ServerId, channelId).Exec(); err != nil {
		RollbackQueries(db)
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	return c.JSON(serverId)
}

func RollbackQueries(db *gocql.Session) {
	for _, deletion := range rollback {
		db.Query(deletion.Query, deletion.Args...).Exec()
	}
}

// DELETE SERVER

func DeleteServer(c *fiber.Ctx) error {
	db := database.DB
	type BodyRequest struct {
		Id string `json:"server_id"`
	}

	var serverId BodyRequest
	var server models.Server
	var channels []models.Channel
	var users []gocql.UUID

	err := c.BodyParser(&serverId)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the server's informations"})
	}

	queryGetServer := "SELECT * FROM servers WHERE server_id = ?"
	if err := db.Query(queryGetServer, serverId.Id).Scan(&server.ServerId, &server.CreatedAt, &server.Banner, &server.Description, &server.Name, &server.Owner, &server.Status); err != nil {
		log.Error(err)
		return c.Status(404).JSON(fiber.Map{"error": "The server you're trying to delete does not exist."})
	}

	queryGetChannels := "SELECT * FROM channels WHERE server_id = ?"
	scanner := db.Query(queryGetChannels, serverId.Id).Iter().Scanner()
	for scanner.Next() {
		var channel models.Channel
		err := scanner.Scan(&channel.ServerId, &channel.ChannelId, &channel.Category, &channel.Name, &channel.Status, &channel.Type)
		if err != nil {
			log.Error(err)
			return c.Status(404).JSON(fiber.Map{"error": "Can't find any channels in this server."})
		}
		channels = append(channels, channel)
	}

	queryDeleteServer := "DELETE FROM servers WHERE server_id = ?"
	if err := db.Query(queryDeleteServer, serverId.Id).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete the server"})
	}

	queryDeleteChannels := "DELETE FROM channels WHERE server_id = ?"
	if err := db.Query(queryDeleteChannels, serverId.Id).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete a new channel"})
	}

	queryLeaveChannel := "DELETE FROM channel_to_users WHERE channel_id = ?"
	for _, channel := range channels {
		if err := db.Query(queryLeaveChannel, channel.ChannelId).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
		}
	}

	queryGetUsersOfServer := "SELECT users FROM server_to_users WHERE server_id = ?"
	if err := db.Query(queryGetUsersOfServer, serverId.Id).Scan(&users); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	queryDeleteServerListOfUsers := "DELETE FROM server_to_users WHERE server_id = ?"
	if err := db.Query(queryDeleteServerListOfUsers, serverId.Id).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete the list of users related to this server."})
	}

	queryDeleteServerFromUsersList := "UPDATE user_to_servers SET servers = servers - ? WHERE user_id = ?"
	queryDeleteUserState := "DELETE FROM user_to_server_state WHERE user_id = ? AND server_id = ?"
	for _, userId := range users {
		if err := db.Query(queryDeleteServerFromUsersList, []string{serverId.Id}, userId).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "Can't delete user from server."})
		}

		if err := db.Query(queryDeleteUserState, userId, serverId.Id).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
		}
	}

	broadcastServerChanges(users, Options{ServerId: &serverId.Id}, "server_deletion")

	return nil
}

func LeaveServer(c *fiber.Ctx) error {
	userId := c.Locals("user_id").(string)
	db := database.DB

	type BodyRequest struct {
		Id string `json:"server_id"`
	}

	var server BodyRequest

	err := c.BodyParser(&server)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the server's informations"})
	}

	serverId := server.Id

	var channels []models.Channel
	queryGetChannels := "SELECT * FROM channels WHERE server_id = ?"
	scanner := db.Query(queryGetChannels, serverId).Iter().Scanner()
	for scanner.Next() {
		var channel models.Channel
		err := scanner.Scan(&channel.ServerId, &channel.ChannelId, &channel.Category, &channel.Name, &channel.Status, &channel.Type)
		if err != nil {
			log.Error(err)
			return c.Status(404).JSON(fiber.Map{"error": "Can't find any channels in this server."})
		}
		channels = append(channels, channel)
	}
	fmt.Println(channels)

	if err := scanner.Err(); err != nil {
		log.Error(err)
	}

	queryLeaveChannel := "DELETE FROM channel_to_users WHERE channel_id = ? AND user_id = ?"
	for _, channel := range channels {
		if err := db.Query(queryLeaveChannel, channel.ChannelId, userId).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
		}
	}

	var users []gocql.UUID
	queryGetUsersOfServer := "SELECT users FROM server_to_users WHERE server_id = ?"
	if err := db.Query(queryGetUsersOfServer, serverId).Scan(&users); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	queryLeaveServer := "UPDATE server_to_users SET users = users - ? WHERE server_id = ?"
	if err := db.Query(queryLeaveServer, []string{userId}, serverId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	queryDeleteServerFromUsersList := "UPDATE user_to_servers SET servers = servers - ? WHERE user_id = ?"
	if err := db.Query(queryDeleteServerFromUsersList, []string{serverId}, userId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	queryDeleteUserState := "DELETE FROM user_to_server_state WHERE user_id = ? AND server_id = ?"
	if err := db.Query(queryDeleteUserState, userId, serverId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	broadcastServerChanges(users, Options{ServerId: &serverId}, "user_leaving")

	return nil
}

func CreateChannel(c *fiber.Ctx) error {
	db := database.DB
	type bodyRequest struct {
		Group   string         `json:"category"`
		Channel models.Channel `json:"channel"`
	}
	var body bodyRequest

	err := c.BodyParser(&body)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the server's informations"})
	}

	newChannel := body.Channel
	newChannel.ChannelId = utils.GenerateNanoid()

	queryCreateChannel := "INSERT INTO channels_test (server_id, channel_id, group, name, parent_id, parent_position, position, status, type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	if err := db.Query(queryCreateChannel, newChannel.ServerId, newChannel.ChannelId, newChannel.Category, newChannel.Name, newChannel.ParentId, newChannel.ParentPosition, newChannel.Position, newChannel.Status, newChannel.Type).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create the new channel"})
	}

	var users []gocql.UUID
	queryGetUsersOfServer := "SELECT users FROM server_to_users WHERE server_id = ?"
	if err := db.Query(queryGetUsersOfServer, newChannel.ServerId).Scan(&users); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "An error occured while leaving the server"})
	}

	for _, user := range users {
		queryConnectUsersToNewChannel := "INSERT INTO channel_to_users (channel_id, user_id) VALUES (?, ?)"
		if err := db.Query(queryConnectUsersToNewChannel, newChannel.ChannelId, user).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "Couldn't make the user join"})
		}
	}

	broadcastServerChanges(users, Options{Group: &body.Group, Channel: &newChannel}, "channel_creation")

	return nil
}

func DeleteChannel(c *fiber.Ctx) error {
	db := database.DB
	type BodyRequest struct {
		ServerId  string `json:"server_id"`
		ChannelId string `json:"channel_id"`
		Group     string `json:"category"`
	}

	var body BodyRequest
	err := c.BodyParser(&body)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the server's informations"})
	}

	queryDeleteChannel := "DELETE FROM channels_test WHERE server_id = ? AND channel_id = ?"
	if err := db.Query(queryDeleteChannel, body.ServerId, body.ChannelId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete the channel"})
	}

	queryDeleteMessages := "DELETE FROM messages WHERE channel_id = ?"
	if err := db.Query(queryDeleteMessages, body.ChannelId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete the messages of this channel"})
	}

	var users []gocql.UUID
	queryGetAllUsersFromChannel := "SELECT user_id FROM channel_to_users WHERE channel_id = ?"
	scanner := db.Query(queryGetAllUsersFromChannel, body.ChannelId).Iter().Scanner()
	for scanner.Next() {
		var userId gocql.UUID
		err := scanner.Scan(&userId)
		if err != nil {
			log.Error(err)
		}
		users = append(users, userId)
	}

	queryRemoveConnectionToChannel := "DELETE FROM channel_to_users WHERE channel_id = ?"
	if err := db.Query(queryRemoveConnectionToChannel, body.ChannelId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete the channel"})
	}

	broadcastServerChanges(users, Options{ServerId: &body.ServerId, ChannelId: &body.ChannelId, Group: &body.Group}, "channel_deletion")

	return nil
}

type Options struct {
	ServerId  *string
	UserId    *string
	ChannelId *string
	Group     *string
	Channel   *models.Channel
	Server    *models.Server
}

func broadcastServerChanges(users []gocql.UUID, opts Options, typeOfMessage string) {
	var data []byte
	var err error

	switch typeOfMessage {
	case "server_deletion", "user_leaving":
		messageToSend := &protobuf.ServerMessage{
			Type: typeOfMessage,
			Payload: &protobuf.ServerMessage_ServerDeletion{
				ServerDeletion: &protobuf.ServerDeletion{
					Id: *opts.ServerId,
				},
			},
		}
		data, err = proto.Marshal(messageToSend)
	case "channel_creation":
		messageToSend := &protobuf.ServerMessage{
			Type: typeOfMessage,
			Payload: &protobuf.ServerMessage_NewChannel{
				NewChannel: &protobuf.NewChannel{
					Group: *opts.Group,
					Channel: &protobuf.Channel{
						ServerId:       opts.Channel.ServerId,
						ChannelId:      opts.Channel.ChannelId,
						Category:       opts.Channel.Category,
						Name:           opts.Channel.Name,
						ParentId:       opts.Channel.ParentId,
						ParentPosition: strconv.Itoa(opts.Channel.ParentPosition),
						Position:       strconv.Itoa(opts.Channel.Position),
						Status:         opts.Channel.Status,
						Type:           opts.Channel.Type,
					},
				},
			},
		}
		data, err = proto.Marshal(messageToSend)
	case "channel_deletion":
		messageToSend := &protobuf.ServerMessage{
			Type: typeOfMessage,
			Payload: &protobuf.ServerMessage_ChannelDeletion{
				ChannelDeletion: &protobuf.ChannelDeletion{
					ChannelId: *opts.ChannelId,
					Category:  *opts.Group,
				},
			},
		}
		data, err = proto.Marshal(messageToSend)
	case "server_join":
		messageToSend := &protobuf.ServerMessage{
			Type: typeOfMessage,
			Payload: &protobuf.ServerMessage_ServerJoin{
				ServerJoin: &protobuf.ServerJoin{
					UserId: *opts.UserId,
					Server: &protobuf.Server{
						ServerId:    *&opts.Server.ServerId,
						Banner:      opts.Server.Banner,
						Description: opts.Server.Description,
						Name:        opts.Server.Name,
						Owner:       opts.Server.Owner.String(),
						Status:      opts.Server.Status,
					},
				},
			},
		}
		data, err = proto.Marshal(messageToSend)
	}

	if err != nil {
		fmt.Println("Error when transforming the message into protobuf", err)
	}

	for _, user_id := range users {
		if conn := Connections[user_id]; conn != nil {
			conn.WriteMessage(websocket.BinaryMessage, data)
		}
	}
}
