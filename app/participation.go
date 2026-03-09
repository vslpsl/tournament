package app

import (
	"context"
	"database/sql"
	"github.com/vslpsl/tournament/model"
	"time"

	"gorm.io/gorm"
)

func (app *App) CreateParticipationRequest(ctx context.Context, userID int64) (user *model.User, request *model.ParticipationRequest, err error) {
	err = app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err = gorm.G[*model.User](tx).Where("id = ? and participation_request_is_sent = ? and is_participant = ?", userID, false, false).First(ctx)
		if err != nil {
			return err
		}

		request = &model.ParticipationRequest{
			UserID:    user.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err = gorm.G[model.ParticipationRequest](tx).Create(ctx, request)
		if err != nil {
			return err
		}

		user.ParticipationRequestIsSent = true
		tx.Save(user)

		return tx.Error
	})

	return user, request, err
}

//func (app *App) ListParticipationRequests(ctx context.Context, offset int64, limit int64) (requests []*model.ParticipationRequest, totalCount int64, err error) {
//	query := app.app.WithContext(ctx).Model(&requests).Preload("User", nil).Where("accepted is null").Order("created_at asc")
//	err = query.Count(&totalCount).Offset(int(offset)).Limit(int(limit)).Find(&requests).Error
//	return requests, totalCount, err
//}

func (app *App) NextParticipationRequest(ctx context.Context, offset int64) (_ *model.ParticipationRequest, totalCount int64, err error) {
	var next model.ParticipationRequest
	err = app.db.WithContext(ctx).Model(&next).Preload("User", nil).Where("accepted is null").Order("created_at asc").Count(&totalCount).Offset(int(offset)).First(&next).Error
	return &next, totalCount, err
}

func (app *App) GetParticipationRequest(ctx context.Context, requestID int64) (*model.ParticipationRequest, error) {
	return gorm.G[*model.ParticipationRequest](app.db).Where("id = ?", requestID).Preload("User", nil).First(ctx)

}

func (app *App) AcceptParticipationRequest(ctx context.Context, requestID int64) (user *model.User, request *model.ParticipationRequest, err error) {
	err = app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request, err = gorm.G[*model.ParticipationRequest](tx).Where("id = ? and accepted is null", requestID).First(ctx)
		if err != nil {
			return err
		}

		user, err = gorm.G[*model.User](tx).Where("id = ? and participation_request_is_sent = ? and is_participant = ?", request.UserID, true, false).First(ctx)
		if err != nil {
			return err
		}

		request.Accepted = sql.NullBool{Bool: true, Valid: true}
		request.ResolvedAt = sql.NullTime{Time: time.Now(), Valid: true}

		tx.Model(request).Updates(
			map[string]interface{}{
				"accepted":    request.Accepted,
				"resolved_at": request.ResolvedAt,
			},
		)

		tx.Model(user).Updates(
			map[string]interface{}{
				"is_participant": true,
			},
		)

		return tx.Error
	})

	return user, request, err
}

func (app *App) RejectParticipationRequest(ctx context.Context, requestID int64, reason string) (user *model.User, request *model.ParticipationRequest, err error) {
	err = app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request, err = gorm.G[*model.ParticipationRequest](tx).Where("id = ? and accepted is null", requestID).First(ctx)
		if err != nil {
			return err
		}

		user, err = gorm.G[*model.User](tx).Where("id = ? and participation_request_is_sent = ? and is_participant = ?", request.UserID, true, false).First(ctx)
		if err != nil {
			return err
		}

		request.Accepted = sql.NullBool{Bool: false, Valid: true}
		request.Reason = sql.NullString{String: reason, Valid: true}
		request.ResolvedAt = sql.NullTime{Time: time.Now(), Valid: true}

		tx.Model(request).Updates(
			map[string]interface{}{
				"accepted":    request.Accepted,
				"reason":      request.Reason,
				"resolved_at": request.ResolvedAt,
			},
		)

		tx.Model(user).Updates(
			map[string]interface{}{
				"participation_request_is_sent": false,
				"is_participant":                false,
			},
		)

		return tx.Error
	})

	return user, request, err
}
