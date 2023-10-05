package database

import (
	"log"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2/table"
)

var subscriberMetadata = table.Metadata{
	Name:    "subscribers",
	Columns: []string{"channels_id", "server_id", "user_id"},
}

var serverMetadata = table.Metadata{
	Name:    "servers",
	Columns: []string{"server_id", "name"},
}

var channelMetadata = table.Metadata{
	Name:    "channels",
	Columns: []string{"channel_id", "name", "server_id"},
}

var messageMetadata = table.Metadata{
	Name:    "messages",
	Columns: []string{"message_id", "channel_id", "content", "created_at", "sender_id", "server_id"},
}

var userMetadata = table.Metadata{
	Name:    "users",
	Columns: []string{"id", "username", "email", "password"},
}

var (
	SubscriberTable = table.New(subscriberMetadata)
	ServerTable     = table.New(serverMetadata)
	ChannelTable    = table.New(channelMetadata)
	MessageTable    = table.New(messageMetadata)
	UserTable       = table.New(userMetadata)
)

var DB *gocql.Session

func InitScyllaDB() {
	cluster := gocql.NewCluster("127.0.0.1")
	cluster.Keyspace = "social"
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	DB = session
}
