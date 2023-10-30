package models

import (
	"io"
	"time"

	"github.com/gocql/gocql"
)

type Views interface {
	Load() error
	Render(io.Writer, string, interface{}, ...string) error
}

type NewMessage struct {
	Message string `json:"chat_message"`
}

type User struct {
	Id          gocql.UUID `db:"id" json:"id"`
	Email       string     `db:"email" json:"email"`
	Username    string     `db:"username" json:"username"`
	About       string     `db:"about" json:"about"`
	Avatar      string     `db:"avatar" json:"avatar"`
	Banner      string     `db:"banner" json:"banner"`
	DisplayName string     `db:"displayname" json:"displayName"`
	Password    string     `db:"password" json:"-"`
}

type Subscriber struct {
	UserId     gocql.UUID   `db:"user_id"`
	ServerId   gocql.UUID   `db:"server_id"`
	ChannelsId []gocql.UUID `db:"channels_id"`
}

type Session struct {
	SessionId string     `db:"session_id"`
	UserId    gocql.UUID `db:"user_id"`
	Timezone  string     `db:"timezone"`
	UserAgent string     `db:"user_agent"`
}

type Message struct {
	MessageId     gocql.UUID   `db:"message_id" json:"id"`
	ChannelId     string       `db:"channel_id" json:"channel_id"`
	ServerId      string       `db:"server_id" json:"server_id"`
	Content       string       `db:"content" json:"content"`
	Mentions      []gocql.UUID `db:"mentions" json:"mentions"`
	MentionsRoles []string     `db:"mentions_roles" json:"mentionsRoles"`
	User          User         `db:"-" json:"sender"`
	UserId        gocql.UUID   `db:"user_id" json:"-"`
	CreatedAt     time.Time    `db:"created_at" json:"createdAt"`
}

type Server struct {
	ServerId    string     `db:"server_id" json:"serverId"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	Banner      string     `db:"banner" json:"banner"`
	Description string     `db:"description" json:"description"`
	Name        string     `db:"name" json:"name"`
	Owner       gocql.UUID `db:"owner" json:"owner"`
	Status      string     `db:"status" json:"status"`
}

type ServerState map[string]string

type Channel struct {
	ServerId       string `db:"server_id" json:"serverId"`
	ChannelId      string `db:"channel_id" json:"channelId"`
	Category       string `db:"group" json:"group"`
	Name           string `db:"name" json:"name"`
	ParentId       string `db:"parent_id" json:"parentId"`
	ParentPosition int    `db:"parent_position" json:"parentPosition"`
	Position       int    `db:"position" json:"position"`
	Status         string `db:"status" json:"status"`
	Type           string `db:"type" json:"type"`
}

type Category struct {
	ServerId   string     `db:"server_id"`
	CategoryId gocql.UUID `db:"category_id"`
	Name       string     `db:"name"`
}

type Invitation struct {
	Id string `json:"invitation_id"`
}
