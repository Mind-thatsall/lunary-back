package handlers

import (
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/env"
	"github.com/Mind-thatsall/fiber-htmx/cmd/models"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkHashedPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CreateUser(c *fiber.Ctx) error {
	db := database.DB

	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Review your input", "data": err})
	}

	ID := gocql.MustRandomUUID()
	addr, err := mail.ParseAddress(user.Email)
	if err != nil {
		return c.Status(500).Send([]byte("Invalid email address"))
	}

	user.Id = ID
	user.Email = addr.Address
	errExist := checkIfUserExist(user.Username, user.Email, db)
	if errExist != nil {
		return c.Status(409).Send([]byte("User already exist"))
	}

	hash, err := hashPassword(user.Password)
	if err != nil {
		return c.Status(500).Send([]byte("Couldn't hash password"))
	}

	user.Password = hash

	q := db.Query("INSERT INTO users (id, username, email, password) VALUES (?, ?, ?, ?)", user.Id, user.Username, user.Email, user.Password)
	if err := q.Exec(); err != nil {
		log.Errorf("Error when creating the user", err)
	}

	return nil
}

func checkIfUserExist(username, email string, db *gocql.Session) error {
	type ExistingUser struct {
		Username string
		Email    string
	}

	var user ExistingUser

	queryUsername := db.Query("SELECT * FROM existing_username WHERE username = ?", username)
	if err := queryUsername.Scan(&user.Username); err != nil {

		queryEmail := db.Query("SELECT * FROM existing_email WHERE email = ?", email)
		if err := queryEmail.Scan(&user.Email); err != nil {
			return nil
		}

	}

	return errors.New("User already Exist")
}

func getUserByEmail(email string) (models.User, error) {
	db := database.DB
	var user models.User
	var user_id gocql.UUID

	queryUserId := "SELECT user_id FROM existing_email WHERE email = ?"
	if err := db.Query(queryUserId, email).Scan(&user_id); err != nil {
		log.Error(err)
		return user, errors.New("User not found")
	}

	queryUser := "SELECT * FROM users WHERE id = ?"
	if err := db.Query(queryUser, user_id).Scan(&user.Id, &user.About, &user.Avatar, &user.Banner, &user.DisplayName, &user.Email, &user.Password, &user.Username); err != nil {
		log.Error(err)
		return user, errors.New("User not found")
	}

	return user, nil
}

func GetUserById(id interface{}) (models.User, error) {
	db := database.DB
	var user models.User

	query := "SELECT * FROM users WHERE id = ?"
	if err := db.Query(query, id).Scan(&user.Id, &user.About, &user.Avatar, &user.Banner, &user.DisplayName, &user.Email, &user.Password, &user.Username); err != nil {
		log.Error(err)
		return user, errors.New("User not found")
	}

	return user, nil
}

func Login(c *fiber.Ctx) error {
	type LoginInput struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	input := new(LoginInput)

	if err := c.BodyParser(input); err != nil {
		log.Error(err)
		return c.Status(fiber.StatusBadRequest).Send([]byte("Error on login request"))
	}

	userData, err := getUserByEmail(input.Email)
	if err != nil {
		return c.Status(404).Send([]byte("User not found"))
	}

	fmt.Println(userData.Password)

	if !checkHashedPassword(userData.Password, input.Password) {
		return c.Status(fiber.StatusUnauthorized).Send([]byte("Your email or password is invalid"))
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = userData.Id
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	t, err := token.SignedString([]byte(env.Variable("SECRET")))
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	cookie := new(fiber.Cookie)
	cookie.Name = "jwt"
	cookie.Value = t
	cookie.HTTPOnly = true
	cookie.Expires = time.Now().Add(time.Hour * 72)
	cookie.SameSite = "None"

	c.Cookie(cookie)
	c.Status(fiber.StatusOK).JSON(userData)
	return nil
}

func CheckUserAuthenticity(c *fiber.Ctx) error {
	cookie := c.Cookies("jwt")

	token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(env.Variable("SECRET")), nil
	})

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Not found"})
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userId := claims["user_id"]
		if userId == nil {
			// If there is no userId claim in the JWT token, return error
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Not found"})
		} else {
			user, err := GetUserById(userId)
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Not found"})
			}

			return c.JSON(user)
		}
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
}