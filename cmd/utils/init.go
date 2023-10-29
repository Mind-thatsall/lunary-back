package utils

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

func GetServerInformations(serverId string) (models.Server, error) {
	db := database.DB
	var server models.Server

	query := "select * from servers where server_id = ?"
	if err := db.Query(query, serverId).Scan(&server.ServerId, &server.CreatedAt, &server.Banner, &server.Description, &server.Name, &server.Owner, &server.Status); err != nil {
		log.Error(err)
	}

	return server, nil

}

func GetChannelsFromServer(c *fiber.Ctx) []models.Channel {
	db := database.DB
	var channelsInformations []models.Channel
	serverId := c.Params("serverId")

	querySub := "SELECT * FROM channels WHERE server_id = ?"
	scanner := db.Query(querySub, serverId).Iter().Scanner()
	for scanner.Next() {
		var channel models.Channel
		err := scanner.Scan(&channel.ChannelId, &channel.Name, &channel.ServerId)
		if err != nil {
			log.Error(err)
		}
		channelsInformations = append(channelsInformations, channel)
	}

	fmt.Println(channelsInformations)

	return channelsInformations
}

func GetChannelsIDFromServerUsingProps(serverId string) []string {
	db := database.DB
	var channelsID []string

	querySub := "SELECT channel_id FROM channels WHERE server_id = ?"
	scanner := db.Query(querySub, serverId).Iter().Scanner()
	for scanner.Next() {
		var channelID string
		err := scanner.Scan(&channelID)
		if err != nil {
			log.Error(err)
		}
		channelsID = append(channelsID, channelID)
	}

	return channelsID
}

func GenerateNanoid() string {
	id := ""

	for i := 0; i < 21; i++ {
		num := rand.Intn(10)
		id += strconv.Itoa(num)
	}

	return id
}
