package handlers

import (
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

var Engine = html.New("./templates", ".html")

func IndexPage(c *fiber.Ctx) error {
	// Render index within layouts/main
	return c.Render("pages/index", fiber.Map{
		"Text":        "Hello, World!",
		"Title":       "Home",
		"Description": "Page about something",
	}, "main")
}

func ChatsPage(c *fiber.Ctx) error {
	return c.Render("pages/chats", fiber.Map{
		"Text":        "Chats!",
		"Title":       "Chats",
		"Description": "Page about chats",
		"User":        c.Locals("user"),
	}, "main")
}

func UserHomePage(c *fiber.Ctx) error {
	return c.Render("pages/user_home", fiber.Map{
		"Text":        "Welcome to your page!",
		"Title":       "User Home",
		"Description": "This is your profile",
		"User":        c.Locals("user"),
	}, "main")
}

func ServerPage(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	return c.Render("pages/server", fiber.Map{
		"Text":        "Welcome to this server!",
		"Title":       "Server page",
		"Description": "This is a Server",
		"User":        user,
	}, "main")
}

func ChannelPage(c *fiber.Ctx) error {
	return c.Render("pages/channel", fiber.Map{
		"Text":        "Welcome to this Channel!",
		"Title":       "Channel page",
		"Description": "This is a Channel",
		"User":        c.Locals("user"),
		"ChannelId":   c.Params("channelId"),
		"ServerId":    c.Params("serverId"),
	}, "main")
}

func LoginPage(c *fiber.Ctx) error {
	return c.Render("pages/login", fiber.Map{
		"Text":        "login!",
		"Title":       "login",
		"Description": "login page",
	}, "main")
}

func RegisterPage(c *fiber.Ctx) error {
	return c.Render("pages/register", fiber.Map{
		"Text":        "register!",
		"Title":       "register",
		"Description": "register page",
	}, "main")
}
