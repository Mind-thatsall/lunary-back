package utils

import (
	"bytes"
	"fmt"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/template/html/v2"
)

func GetServerInformations(serverId string) (models.Server, error) {
	db := database.DB
	var server models.Server

	query := "select * from servers where server_id = ?"
	if err := db.Query(query, serverId).Scan(&server.ServerId, &server.Banner, &server.Description, &server.Name, &server.Owner, &server.Status); err != nil {
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

func RenderTemplate(engine *html.Engine, templateName string, informations fiber.Map) ([]byte, error) {
	var buf bytes.Buffer
	err := engine.Render(&buf, templateName, informations)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
