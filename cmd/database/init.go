package database

import (
	"log"

	"github.com/gocql/gocql"
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
