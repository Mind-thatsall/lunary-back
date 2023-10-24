package router

import (
	"github.com/Mind-thatsall/fiber-htmx/cmd/env"
	"github.com/Mind-thatsall/fiber-htmx/cmd/handlers"
	"github.com/Mind-thatsall/fiber-htmx/cmd/middleware"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func SetupRoutes(app *fiber.App) {
	var JWTMiddleware = middleware.JWTAuthMiddleware(env.Variable("SECRET"))

	app.Get("/ws/connect", websocket.New(handlers.Connect))

	// Middleware
	api := app.Group("/api", logger.New())
	api.Post("/new_message/:serverId/:channelId", JWTMiddleware, handlers.NewMessage)
	api.Post("/new_dm/:channelId", JWTMiddleware, handlers.NewDM)
	api.Get("/channels/:serverId", JWTMiddleware, handlers.GetChannelsFromServer)
	api.Get("/messages/:channelId", JWTMiddleware, handlers.GetMessageFromChannel)
	api.Get("/new_signed_url_s3/:entity/:bucketName/:folder/:media/:version", JWTMiddleware, handlers.PutObjectInS3Bucket)
	api.Get("/update/:media/:version", JWTMiddleware, handlers.UpdateMediaForUser)
	api.Post("/update_server_state", JWTMiddleware, handlers.UpdateServerState)
	api.Get("/get_last_servers_state", JWTMiddleware, handlers.GetServerState)
	api.Post("/create_server", JWTMiddleware, handlers.CreateServer)
	api.Post("/delete_server", JWTMiddleware, handlers.DeleteServer)
	api.Post("/leave_server", JWTMiddleware, handlers.LeaveServer)
	api.Post("/join_server", JWTMiddleware, handlers.JoinServer)
	api.Post("/create_channel", JWTMiddleware, handlers.CreateChannel)
	api.Post("/delete_channel", JWTMiddleware, handlers.DeleteChannel)
	//
	// User
	user := api.Group("/user")
	user.Post("/", handlers.CreateUser)
	user.Post("/login", handlers.Login)
	user.Get("/servers", JWTMiddleware, handlers.GetServersOfUser)
}
