package app

import (
	"context"
	"errors"

	"github.com/vslpsl/tournament/model"

	"gorm.io/gorm"
)

func (app *App) GetAdmins(ctx context.Context) ([]*model.User, error) {
	return gorm.G[*model.User](app.db).Where("role = ?", model.UserRoleAdmin).Find(ctx)
}

func (app *App) GetModerators(ctx context.Context) ([]*model.User, error) {
	return gorm.G[*model.User](app.db).Where("role = ?", model.UserRoleModerator).Find(ctx)
}

func (app *App) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	user, err := gorm.G[model.User](app.db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

func (app *App) CreateUser(ctx context.Context, user *model.User) error {
	return gorm.G[model.User](app.db).Create(ctx, user)
}

func (app *App) ListUsers(ctx context.Context, offset, limit int64) (users []*model.User, totalCount int64, err error) {
	query := app.db.WithContext(ctx).Model(&users).Order("created_at asc")
	err = query.Count(&totalCount).Offset(int(offset)).Limit(int(limit)).Find(&users).Error
	return users, totalCount, err
}

func (app *App) SetUserRole(ctx context.Context, userID int64, role string) (user *model.User, err error) {
	err = app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err = gorm.G[*model.User](tx).Where("id = ?", userID).First(ctx)
		if err != nil {
			return err
		}

		user.Role = role

		return tx.Save(user).Error
	})

	return user, err
}

func (app *App) Participants(ctx context.Context) ([]*model.User, error) {
	return gorm.G[*model.User](app.db).Preload("Catches", nil).Where("is_participant = true").Order("created_at desc").Find(ctx)
}

func (app *App) ListParticipants(ctx context.Context, offset, limit int64) (participants []*model.User, totalCount int64, err error) {
	err = app.db.WithContext(ctx).Model(&model.User{}).Where("is_participant = true").Order("created_at asc").Count(&totalCount).Limit(int(limit)).Offset(int(offset)).Find(&participants).Error
	return participants, totalCount, err
}
