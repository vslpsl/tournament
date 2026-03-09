package app

import (
	"context"
	"database/sql"
	"github.com/vslpsl/tournament/model"

	"gorm.io/gorm"
)

func (app *App) CreateCatch(ctx context.Context, catch *model.Catch) error {
	return app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(catch).Error
	})
}

func (app *App) GetCatch(ctx context.Context, catchID int64) (*model.Catch, error) {
	return gorm.G[*model.Catch](app.db).Preload("User", nil).Preload("Reviews", nil).Where("id = ?", catchID).First(ctx)
}

func (app *App) CreateAcceptedCatchReview(ctx context.Context, catchID int64, reviewerID int64, species model.Species, size int, condition string) (*model.CatchReview, error) {
	review := &model.CatchReview{
		CatchID:    catchID,
		ReviewerID: reviewerID,
		Species:    species,
		Size:       size,
		Condition:  condition,
		Accepted:   true,
	}

	return review, app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(review).Error
	})
}

func (app *App) CreateRejectedCatchReview(ctx context.Context, catchID int64, reviewerID int64, reason string) (*model.CatchReview, error) {
	review := &model.CatchReview{
		CatchID:      catchID,
		ReviewerID:   reviewerID,
		Accepted:     false,
		RejectReason: sql.NullString{Valid: true, String: reason},
	}

	return review, app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(review).Error
	})
}

func (app *App) ListCatches(ctx context.Context, userID int64, asc bool, offset, limit int64) (catches []*model.Catch, totalCount int64, err error) {
	sordOrder := "desc"
	if asc {
		sordOrder = "asc"
	}

	err = app.db.WithContext(ctx).Model(&model.Catch{}).Preload("User").Preload("Reviews").Where("user_id = ?", userID).Order("created_at " + sordOrder).Count(&totalCount).Offset(int(offset)).Limit(int(limit)).Find(&catches).Error
	return catches, totalCount, err
}

func (app *App) ListCatchesForReview(ctx context.Context, userID int64, offset, limit int64) (catches []*model.Catch, totalCount int64, err error) {
	err = app.db.WithContext(ctx).Model(&model.Catch{}).Preload("User").Preload("Reviews").Where("accepted is null").Order("created_at asc").Count(&totalCount).Offset(int(offset)).Limit(int(limit)).Find(&catches).Error
	return catches, totalCount, err
}

func (app *App) NextCatchForValidation(ctx context.Context, targetCatchID int64) (_ *model.Catch, totalCount int64, err error) {
	var catch model.Catch
	err = app.db.WithContext(ctx).Model(&model.Catch{}).Preload("User").Preload("Reviews").Where("accepted is null and id > ?", targetCatchID).Order("created_at asc").Count(&totalCount).Find(&catch).Error
	if err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return nil, 0, gorm.ErrRecordNotFound
	}

	return &catch, totalCount, nil
}

func (app *App) CreateCatchValidationWithReview(ctx context.Context, catchID int64, moderatorID int64, reviewID int64) (catch *model.Catch, validation *model.CatchValidation, err error) {
	err = app.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		catch, err = gorm.G[*model.Catch](tx).Where("id = ? and accepted is null", catchID).First(ctx)
		if err != nil {
			return err
		}

		var review *model.CatchReview
		review, err = gorm.G[*model.CatchReview](tx).Where("id = ?", reviewID).First(ctx)
		if err != nil {
			return err
		}

		validation = &model.CatchValidation{
			CatchID:      catch.ID,
			ModeratorID:  moderatorID,
			ReviewID:     &reviewID,
			Species:      review.Species,
			Size:         review.Size,
			Condition:    review.Condition,
			Accepted:     review.Accepted,
			RejectReason: review.RejectReason,
		}

		err = gorm.G[model.CatchValidation](tx).Create(ctx, validation)
		if err != nil {
			return err
		}

		catch.Species = validation.Species
		catch.Size = validation.Size
		catch.Condition = validation.Condition
		catch.Accepted = sql.NullBool{Bool: validation.Accepted, Valid: true}
		catch.RejectReason = validation.RejectReason
		if err = tx.Save(catch).Error; err != nil {
			return err
		}

		return nil
	})

	return catch, validation, err
}
