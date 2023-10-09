package handlers

import (
	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/Mind-thatsall/fiber-htmx/cmd/utils"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

type Invitation struct {
	Id   string      `json:"invitation_id"`
	User models.User `json:"user"`
}

func JoinServer(c *fiber.Ctx) error {
	db := database.DB
	var serverId string
	var serverInformations models.Server
	var channels []string
	var Invitation Invitation

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

	channels = utils.GetChannelsIDFromServerUsingProps(serverId)

	queryJoin := "INSERT INTO channel_to_users (channel_id, user_id) VALUES (?, ?)"
	for _, channel := range channels {
		if err := db.Query(queryJoin, channel, Invitation.User.Id).Exec(); err != nil {
			log.Error(err)
			return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
		}
	}

	queryServerInformations := "SELECT * FROM servers WHERE server_id = ?"
	if err := db.Query(queryServerInformations, serverId).Scan(&serverInformations.ServerId, &serverInformations.Banner, &serverInformations.Description, &serverInformations.Name, &serverInformations.Owner, &serverInformations.Status); err != nil {
		log.Error(err)
	}

	return c.JSON(serverInformations)
}

func join(c *fiber.Ctx, server models.Server, channelId string, db *gocql.Session) error {
	userId := c.Locals("user_id")

	queryJoinChannel := "INSERT INTO channel_to_users (channel_id, user_id) VALUES (?, ?)"
	if err := db.Query(queryJoinChannel, channelId, userId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	queryJoinServer := "INSERT INTO server_to_users (server_id, user_id) VALUES (?, ?)"
	if err := db.Query(queryJoinServer, server.ServerId, userId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	queryJoinUserServerList := "INSERT INTO user_to_servers (user_id, server_id) VALUES (?, ?)"
	if err := db.Query(queryJoinUserServerList, userId, server.ServerId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

	queryAddUserServerState := "INSERT INTO user_to_server_state (user_id, server_id, last_channel_id) VALUES (?, ?, ?)"
	if err := db.Query(queryAddUserServerState, userId, server.ServerId, channelId).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't join the server"})
	}

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

	querySub := "SELECT server_id FROM user_to_servers WHERE user_id = ?"
	scanner := db.Query(querySub, userId).Iter().Scanner()
	for scanner.Next() {
		var serverId string
		err := scanner.Scan(&serverId)
		if err != nil {
			log.Error(err)
		}
		serverIds = append(serverIds, serverId)
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

func GetChannelsFromServer(c *fiber.Ctx) error {
	db := database.DB
	var channelsInformations []models.Channel
	serverId := c.Params("serverId")

	querySub := "SELECT * FROM channels WHERE server_id = ?"
	scanner := db.Query(querySub, serverId).Iter().Scanner()
	for scanner.Next() {
		var channel models.Channel
		err := scanner.Scan(&channel.ServerId, &channel.ChannelId, &channel.Group, &channel.Name, &channel.Type)
		if err != nil {
			log.Error(err)
		}
		channelsInformations = append(channelsInformations, channel)
	}

	return c.JSON(channelsInformations)
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
	userId := c.Locals("user_id")
	var server models.Server

	err := c.BodyParser(&server)
	if err != nil {
		log.Errorf("Error when parsing the body: %v", err)
		return c.Status(422).JSON(fiber.Map{"error": "Error when parsing the server's informations"})
	}

	channelId := utils.GenerateNanoid()
	serverId := utils.GenerateNanoid()

	server.ServerId = serverId

	queryCreateServer := "INSERT INTO servers (server_id, banner, description, name, owner, status) VALUES (?, ?, ?, ?, ?, ?)"
	if err := db.Query(queryCreateServer, server.ServerId, server.Banner, server.Description, server.Name, userId, server.Status).Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create the server"})
	}

	queryCreateChannel := "INSERT INTO channels (server_id, channel_id, group, name, type) VALUES (?, ?, ?, ?, ?)"
	if err := db.Query(queryCreateChannel, serverId, channelId, "Home", "General", "textual").Exec(); err != nil {
		log.Error(err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create a new channel"})
	}

	errorJoin := join(c, server, channelId, db)
	if errorJoin != nil {
		return errorJoin
	}

	return nil
}
