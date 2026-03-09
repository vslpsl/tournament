package model

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type CatchReview struct {
	ID           int64 `gorm:"primarykey"`
	CatchID      int64 `gorm:"uniqueIndex:idx_catch_id_reviewer_id"`
	ReviewerID   int64 `gorm:"uniqueIndex:idx_catch_id_reviewer_id"`
	Species      Species
	Size         int
	Condition    string
	Accepted     bool
	RejectReason sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}
