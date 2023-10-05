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

	app.Get("/", handlers.IndexPage)
	app.Get("/login", JWTMiddleware, handlers.LoginPage)
	app.Get("/register", JWTMiddleware, handlers.RegisterPage)
	app.Get("/ws/connect/:userId", websocket.New(handlers.Connect))

	// Middleware
	api := app.Group("/api", logger.New())
	api.Post("/new_message/:serverId/:channelId", JWTMiddleware, handlers.NewMessage)
	api.Post("/join_server", JWTMiddleware, handlers.JoinServer)
	api.Get("/channels/:serverId", JWTMiddleware, handlers.GetChannelsFromServer)
	api.Get("/messages/:channelId", JWTMiddleware, handlers.GetMessageFromChannel)
	api.Get("/new_signed_url_s3/:bucketName/:folder/:media/:version", JWTMiddleware, handlers.PutObjectInS3Bucket)
	api.Get("/update/:media/:version", JWTMiddleware, handlers.UpdateMediaForUser)
	api.Post("/update_server_state", JWTMiddleware, handlers.UpdateServerState)
	api.Get("/get_last_servers_state", JWTMiddleware, handlers.GetServerState)
	//
	// User
	user := api.Group("/user")
	// user.Get("/:id", handler.GetUser)
	user.Post("/", handlers.CreateUser)
	user.Post("/login", handlers.Login)
	user.Get("/check", handlers.CheckUserAuthenticity)
	user.Get("/servers", JWTMiddleware, handlers.GetServersOfUser)
	// user.Patch("/:id", middleware.Protected(), handler.UpdateUser)
	// user.Delete("/:id", middleware.Protected(), handler.DeleteUser)

	pages := app.Group("/me")
	pages.Get("/", handlers.UserHomePage)
	pages.Get("/:serverId", handlers.ServerPage)
	pages.Get("/:serverId/:channelId", handlers.ChannelPage)
}
