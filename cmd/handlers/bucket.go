package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Mind-thatsall/fiber-htmx/cmd/database"
	"github.com/Mind-thatsall/fiber-htmx/cmd/env"
	"github.com/Mind-thatsall/fiber-htmx/cmd/utils"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

var Client *s3.Client
var PresignerClient Presigner

func InitBucket() *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile("userbulles"))
	if err != nil {
		log.Fatal(err)
	}

	return s3.NewFromConfig(cfg)
}

type Presigner struct {
	PresignClient *s3.PresignClient
}

func NewPresigner() {
	Client = InitBucket()
	PresignerClient.PresignClient = s3.NewPresignClient(Client)
}

func (presigner Presigner) PutObject(bucketName string, objectKey string) (*v4.PresignedHTTPRequest, error) {

	request, err := presigner.PresignClient.PresignPutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(5) * time.Second
	})
	if err != nil {
		log.Errorf("Couldn't get a presigned request to put %v:%v. Here's why: %v\n",
			bucketName, objectKey, err)
	}
	return request, err

}

func PutObjectInS3Bucket(c *fiber.Ctx) error {
	bucketName := c.Params("bucketName")
	folder := c.Params("folder")
	media := c.Params("media")
	version := c.Params("version")
	entity := c.Params("entity")

	var objectKey string
	var serverId string
	var userId string
	if entity == "user" {
		userId = c.Locals("user_id").(string)
		objectKey = fmt.Sprintf("%s/%s_%s_v%s.webp", folder, media, userId, version)
	} else {
		serverId = utils.GenerateNanoid()
		objectKey = fmt.Sprintf("%s/%s_%s_v%s.webp", folder, media, serverId, version)
	}

	url, err := PresignerClient.PutObject(bucketName, objectKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't create the presigned URL"})
	}

	var finalObject fiber.Map
	if entity == "user" {
		finalObject = fiber.Map{"url": url}
	} else {
		finalObject = fiber.Map{"url": url, "server_id": serverId}
	}

	return c.Status(200).JSON(finalObject)
}

func UpdateMediaForUser(c *fiber.Ctx) error {
	db := database.DB

	media := c.Params("media")
	version := c.Params("version")
	oldVersion, _ := strconv.Atoi(version)
	oldVersionStr := strconv.Itoa(oldVersion - 1)

	user_id := c.Locals("user_id").(string)

	url := fmt.Sprintf("%suser_%s/%s_%s_v%s.webp", env.Variable("CLOUDFRONT_URL"), media, media, user_id, version)

	bucketName := "bulles-bucket"
	objectKey := fmt.Sprintf("user_%s/%s_%s_v%s.webp", media, media, user_id, oldVersionStr)

	_, err := Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Errorf("Error when deleting old media: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Couldn't delete old media"})
	}

	query := fmt.Sprintf("UPDATE users SET %s = ? WHERE id = ?", media)
	q := db.Query(query, url, user_id)
	if err := q.Exec(); err != nil {
		log.Errorf("Error when updating medias: %v", err)
	}

	return c.Status(200).JSON(fiber.Map{"url": url})
}
