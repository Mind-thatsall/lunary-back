package handlers

import (
	"fmt"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/Mind-thatsall/fiber-htmx/public/protobuf"
	"github.com/gocql/gocql"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"google.golang.org/protobuf/proto"
)

func NewMessage(c *fiber.Ctx) error {
	var message models.Message
	db := database.DB

	err := c.BodyParser(&message)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "Error when sending the message"})
	}

	channelId := c.Params("channelId")
	serverId := c.Params("serverId")

	message.MessageId = gocql.MustRandomUUID()
	message.ChannelId = channelId
	message.ServerId = serverId

	q := db.Query("INSERT INTO messages (message_id, channel_id, content, created_at, sender_id, server_id) VALUES (?, ?, ?, ?, ?, ?)", message.MessageId, message.ChannelId, message.Content, message.CreatedAt, message.User.Id, message.ServerId)
	if err := q.Exec(); err != nil {
		log.Errorf("Error when creating the user: %v", err)
	}

	users := getAllUsersFromChannel(message.ChannelId, db)

	broadcastMessage(message.User, users, message)

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

func broadcastMessage(sender models.User, users []gocql.UUID, message models.Message) {

	messageSender := &protobuf.UserMessage_Sender{
		Id:       sender.Id.String(),
		Email:    sender.Email,
		Username: sender.Username,
	}

	messageToSend := &protobuf.UserMessage{
		Id:        message.MessageId.String(),
		Content:   message.Content,
		Channelid: message.ChannelId,
		Sender:    messageSender,
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

		err := scanner.Scan(&message.ChannelId, &message.MessageId, &message.Content, &message.CreatedAt, &message.UserId, &message.ServerId)
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
