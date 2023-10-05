package env

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func Variable(key string) string {
	// load .env file
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Can't read env")
	}

	return os.Getenv(key)
}
