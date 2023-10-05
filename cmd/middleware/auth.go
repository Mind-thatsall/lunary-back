package middleware

import (
	"fmt"

	"github.com/Mind-thatsall/fiber-htmx/cmd/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuthMiddleware(secretKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		cookie := c.Cookies("jwt")

		token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
			// Validate the alg
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secretKey), nil
		})

		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userId := claims["user_id"]
			if userId == nil {
				// If there is no userId claim in the JWT token, return error
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			} else {
				_, err := handlers.GetUserById(userId)
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
				}
				c.Locals("user_id", claims["user_id"])
			}
		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		// If everything is ok, let the request pass
		return c.Next()
	}
}
