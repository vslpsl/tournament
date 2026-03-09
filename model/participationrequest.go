package model

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type ParticipationRequest struct {
	ID         int64 `gorm:"primarykey"`
	UserID     int64
	Accepted   sql.NullBool
	Reason     sql.NullString
	ResolvedAt sql.NullTime
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	User       User
}
