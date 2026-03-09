package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	UserRoleAdmin     = "admin"
	UserRoleModerator = "moderator"
	UserRoleUser      = "user"
)

type User struct {
	ID                         int64 `gorm:"primarykey"` // equals to telegram id
	ChatID                     int64
	Role                       string
	ParticipationRequestIsSent bool
	IsParticipant              bool
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	DeletedAt                  gorm.DeletedAt `gorm:"index"`
	Catches                    []Catch        `gorm:"foreignKey:UserID"`
}
