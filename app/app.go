package app

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/vslpsl/tournament/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type App struct {
	db      *gorm.DB
	dataDir string
}

func NewApp(dataDir string) (*App, error) {
	connStr := "postgres://tgbot:tgbot@localhost:5432/tgbot?sslmode=disable"

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level (Info, Warn, Error, Silent)
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      false,       // Print SQL with arguments
			Colorful:                  true,        // Enable color
		},
	)

	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}

	if err = db.AutoMigrate(&model.User{}, &model.ParticipationRequest{}, &model.Catch{}, &model.CatchReview{}, &model.CatchValidation{}); err != nil {
		return nil, err
	}

	users := []model.User{
		{
			ID:            371976717,
			ChatID:        371976717,
			Role:          model.UserRoleAdmin,
			IsParticipant: false,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for _, user := range users {
		_, err = gorm.G[model.User](db).Where("id = ?", user.ID).First(ctx)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			err = gorm.G[model.User](db).Create(ctx, &user)
			if err != nil {
				return nil, err
			}
		}

	}

	return &App{db, dataDir}, nil
}

func (app *App) Close() error {
	return nil
}
