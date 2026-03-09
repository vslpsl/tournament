package model

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

const (
	MediaTypeImage = "image"
	MediaTypeVideo = "video"
)

const (
	ConditionShore    = "shore"
	ConditionOffshore = "offshore"
)

type Species string

const (
	SpeciesPerch  = "perch"
	SpeciesZander = "zander"
	SpeciesPike   = "pike"
	SpeciesBream  = "bream"
)

func (s Species) Translation() string {
	switch s {
	case SpeciesPerch:
		return "Окунь"
	case SpeciesZander:
		return "Судак"
	case SpeciesPike:
		return "Щука"
	case SpeciesBream:
		return "Лещ"
	default:
		return string(s)
	}
}

func (s Species) IsValid() bool {
	for _, species := range SpeciesList {
		if species == s {
			return true
		}
	}
	return false
}

var SpeciesList = [4]Species{SpeciesPerch, SpeciesZander, SpeciesPike, SpeciesBream}

type Catch struct {
	ID                   int64 `gorm:"primarykey"`
	UserID               int64
	DataFilePath         string
	FileName             string
	TelegramFileID       string
	TelegramFileUniqueID string
	MediaType            string
	Size                 int
	Condition            string
	Species              Species
	Accepted             sql.NullBool
	RejectReason         sql.NullString
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`
	User                 User
	Reviews              []CatchReview `gorm:"foreignKey:CatchID"`
}
