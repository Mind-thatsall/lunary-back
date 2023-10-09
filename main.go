package main

import (
	"log"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/handlers"
	"github.com/Mind-thatsall/fiber-htmx/cmd/router"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	app := fiber.New()

	database.InitScyllaDB()
	handlers.NewPresigner()

	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Set-Cookie",
		AllowCredentials: true,
	}))

	app.Static("/", "./public")

	router.SetupRoutes(app)
	log.Fatal(app.Listen("localhost:3000"))
}
