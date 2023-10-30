package handlers

import (
	"fmt"
	"time"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/Mind-thatsall/fiber-htmx/public/protobuf"
	"github.com/gocql/gocql"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewMessage(c *fiber.Ctx) error {
	var message models.Message
	db := database.DB
	var users []gocql.UUID

	err := c.BodyParser(&message)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "Error when sending the message"})
	}

	channelId := c.Params("channelId")
	serverId := c.Params("serverId")

	message.ChannelId = channelId
	message.ServerId = serverId
	users = getAllUsersFromChannel(message.ChannelId, db)

	message.MessageId = gocql.MustRandomUUID()
	t := time.Now()
	timestamp := timestamppb.New(t)
	message.CreatedAt = t

	q := db.Query("INSERT INTO messages (message_id, channel_id, content, mentions, mentions_roles, created_at, sender_id, server_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", message.MessageId, message.ChannelId, message.Content, message.Mentions, message.MentionsRoles, message.CreatedAt, message.User.Id, message.ServerId)
	if err := q.Exec(); err != nil {
		log.Errorf("Error when creating the user: %v", err)
	}

	broadcastMessage(message.User, users, message, timestamp)

	return nil
}

func NewDM(c *fiber.Ctx) error {
	var message models.Message
	db := database.DB
	var users []gocql.UUID

	err := c.BodyParser(&message)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "Error when sending the message"})
	}

	channelId := c.Params("channelId")
	querySub := "SELECT user_id FROM channel_to_users WHERE channel_id = ?"
	scanner := db.Query(querySub, channelId).Iter().Scanner()
	for scanner.Next() {
		var user_id gocql.UUID
		err := scanner.Scan(&user_id)
		if err != nil {
			log.Error(err)
		}

		if user_id != message.User.Id {
			users = append(users, user_id)
			err := isFriend(c, db, message.User.Id, user_id)
			if err != nil {
				return err
			}
		}
	}

	users = append(users, message.User.Id)

	message.ChannelId = channelId

	message.MessageId = gocql.MustRandomUUID()
	t := time.Now()
	timestamp := timestamppb.New(t)
	message.CreatedAt = t

	q := db.Query("INSERT INTO messages (message_id, channel_id, content, mentions, mentions_roles, created_at, sender_id, server_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", message.MessageId, message.ChannelId, message.Content, message.Mentions, message.MentionsRoles, message.CreatedAt, message.User.Id, message.ServerId)
	if err := q.Exec(); err != nil {
		log.Errorf("Error when creating the user: %v", err)
	}

	broadcastMessage(message.User, users, message, timestamp)

	return nil
}

func getAllUsersFromChannel(channelId string, db *gocql.Session) []gocql.UUID {
	var users []gocql.UUID

	querySub := "SELECT user_id FROM channel_to_users WHERE channel_id = ?"
	scanner := db.Query(querySub, channelId).Iter().Scanner()
	for scanner.Next() {
		var user_id gocql.UUID
		err := scanner.Scan(&user_id)
		if err != nil {
			log.Error(err)
		}
		users = append(users, user_id)
	}

	return users
}

func broadcastMessage(sender models.User, users []gocql.UUID, message models.Message, timestamp *timestamppb.Timestamp) {

	messageSender := &protobuf.User{
		Id:          sender.Id.String(),
		Email:       sender.Email,
		Username:    sender.Username,
		DisplayName: sender.DisplayName,
		Avatar:      sender.Avatar,
	}

	var mentions []string
	if len(message.Mentions) > 0 {
		for _, mentionUUID := range message.Mentions {
			mentions = append(mentions, mentionUUID.String())
		}
	}

	messageToSend := &protobuf.ServerMessage{
		Type: "message",
		Payload: &protobuf.ServerMessage_UserMessage{
			UserMessage: &protobuf.UserMessage{
				Id:            message.MessageId.String(),
				Content:       message.Content,
				Mentions:      mentions,
				MentionsRoles: message.MentionsRoles,
				ChannelId:     message.ChannelId,
				CreatedAt:     timestamp,
				Sender:        messageSender,
			},
		},
	}

	data, err := proto.Marshal(messageToSend)
	if err != nil {
		fmt.Println("Error when transforming the message into protobuf", err)
	}

	for _, user_id := range users {
		if conn := Connections[user_id]; conn != nil {
			conn.WriteMessage(websocket.BinaryMessage, data)
		}
	}
}

func GetMessageFromChannel(c *fiber.Ctx) error {
	db := database.DB
	var messages []models.Message

	channelId := c.Params("channelId")

	queryMessage := "SELECT * FROM messages WHERE channel_id = ?"
	queryUser := "SELECT * FROM users WHERE id = ?"

	scanner := db.Query(queryMessage, channelId).Iter().Scanner()
	for scanner.Next() {
		var message models.Message
		var user models.User

		err := scanner.Scan(&message.ChannelId, &message.CreatedAt, &message.MessageId, &message.Content, &message.Mentions, &message.MentionsRoles, &message.UserId, &message.ServerId)
		if err != nil {
			log.Error(err)
		}

		if err := db.Query(queryUser, message.UserId).Scan(&user.Id, &user.About, &user.Avatar, &user.Banner, &user.DisplayName, &user.Email, &user.Password, &user.Username); err != nil {
			log.Error(err)
			return c.Status(404).JSON(fiber.Map{"error": "Error when fetching matching user of message"})
		}

		message.User = user
		messages = append(messages, message)
	}

	return c.JSON(messages)
}

func isFriend(c *fiber.Ctx, db *gocql.Session, userId interface{}, friendId gocql.UUID) error {
	queryCheckFriend := db.Query("SELECT friend_id FROM friends WHERE user_id = ? AND friend_id = ?", userId, friendId)
	if err := queryCheckFriend.Scan(&friendId); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "The two users are not friends"})
	}

	return nil
}
