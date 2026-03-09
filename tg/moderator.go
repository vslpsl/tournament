package tg

import (
	"errors"
	"fmt"
	"github.com/vslpsl/tournament/model"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"gorm.io/gorm"
)

func HandleModeratorStartCommand(ctx *th.Context, user *model.User, app App, message telego.Message) error {
	if err := sendMessageWithModeratorCommands(ctx, user, "Поехали!"); err != nil {
		fmt.Println(err)
	}

	return HandleTournamentMessage(app)(ctx, message)
}

func sendMessageWithModeratorCommands(ctx *th.Context, user *model.User, text string) error {
	commands := []telego.BotCommand{
		{Command: "start", Description: "Start the bot"},
		{Command: "help", Description: "Help"},
	}

	err := ctx.Bot().SetMyCommands(ctx, &telego.SetMyCommandsParams{
		Commands: commands,
		Scope:    tu.ScopeChat(tu.ID(user.ChatID)),
	})
	if err != nil {
		return err
	}

	replyMessage := tu.Message(tu.ID(user.ChatID), text)

	var keyboardRows [][]telego.KeyboardButton

	keyboardRows = append(keyboardRows, tu.KeyboardRow(tu.KeyboardButton(TournamentButton)))
	keyboardRows = append(keyboardRows, tu.KeyboardRow(tu.KeyboardButton(ArbitrationButton)))
	if len(keyboardRows) > 0 {
		replyMessage = replyMessage.WithReplyMarkup(tu.Keyboard(keyboardRows...).WithResizeKeyboard())
	}

	return SendMessage(ctx, replyMessage)
}

func HandleArbitrationMessage(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetUser(ctx, app, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		if user.Role != model.UserRoleModerator {
			return nil
		}

		return sendNextCatchForValidation(ctx, app, 0, message.GetChat().ID)
	}
}

func ValidateCatchCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		if err := ctx.Bot().DeleteMessage(ctx, tu.Delete(query.Message.GetChat().ChatID(), query.Message.GetMessageID())); err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Failed to delete message: %v", err)))
		}

		var catchID int64
		var reviewID *int64
		components := strings.Split(query.Data, ":")
		if len(components) < 2 {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), "Invalid callback data"))
		}

		catchID, err := strconv.ParseInt(components[1], 10, 64)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), "Invalid catch id"))
		}

		if len(components) > 2 {
			var rawReviewID int64
			rawReviewID, err = strconv.ParseInt(components[2], 10, 64)
			if err != nil {
				return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), "Invalid review id"))
			}

			reviewID = &rawReviewID
		}

		if len(components) == 2 {
			return sendNextCatchForValidation(ctx, app, catchID, query.Message.GetChat().ID)
		}

		user, err := app.GetUserByID(ctx, query.From.ID)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Failed to get user: %v", err)))
		}

		if user.Role != model.UserRoleModerator {
			return nil
		}

		catch, err := app.GetCatch(ctx, catchID)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Failed to get catch: %v", err)))
		}

		var review *model.CatchReview
		for _, catchReview := range catch.Reviews {
			if catchReview.ID == *reviewID {
				review = &catchReview
				break
			}
		}

		if review == nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), "Invalid review id"))
		}

		var validation *model.CatchValidation
		catch, validation, err = app.CreateCatchValidationWithReview(ctx, catchID, user.ID, *reviewID)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Failed to create catch validation: %v", err)))
		}

		if err = SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Провалидировано!"))); err != nil {
			fmt.Println(err)
		}

		if validation.Accepted {
			err = SendMessage(ctx, tu.Message(tu.ID(catch.UserID), fmt.Sprintf("Ваша поимочка длиной %.1f см засчитана!", float64(validation.Size)/10)))
		} else {
			err = SendMessage(ctx, tu.Message(tu.ID(catch.UserID), fmt.Sprintf("Ваша поимочку зарежектили и пускай сегодня не повезло, но игра продолжается!")))
		}

		if err != nil {
			fmt.Println(err)
		}

		return sendNextCatchForValidation(ctx, app, catchID, query.Message.GetChat().ID)
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("catch_validate")}
}

func sendNextCatchForValidation(ctx *th.Context, app App, targetCatchID int64, chatID int64) error {
	var catch *model.Catch
	var totalCount int64
	var err error
	catch, totalCount, err = app.NextCatchForValidation(ctx, targetCatchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SendMessage(ctx, tu.Message(tu.ID(chatID), "А нет поимочек то"))
		}
		return SendMessage(ctx, tu.Message(tu.ID(chatID), fmt.Sprintf("Failed to get next catch: %v", err)))
	}

	var keyboardRows [][]telego.InlineKeyboardButton

	for _, review := range catch.Reviews {
		var text string
		if review.Accepted {
			if review.Condition == model.ConditionShore {
				text = "Береговой"
			} else {
				text = "Офшорный"
			}
			text += fmt.Sprintf(" окушок длиной %.1f см", float64(review.Size)/10)
		} else {
			text = "Это говно зарежектнуто"
		}

		keyboardRows = append(
			keyboardRows,
			tu.InlineKeyboardRow(tu.InlineKeyboardButton(text).WithCallbackData(fmt.Sprintf("catch_validate:%d:%d", catch.ID, review.ID))),
		)
	}

	if totalCount > 1 {
		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Следующая поимочка").WithCallbackData(fmt.Sprintf("catch_validate:%d", catch.ID))))
	}

	keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))

	var chatMember telego.ChatMember
	chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
		ChatID: tu.ID(catch.UserID),
		UserID: catch.UserID,
	})
	if err != nil {
		return SendMessage(ctx, tu.Message(tu.ID(chatID), fmt.Sprintf("Failed to get chat member: %v", err)))
	}

	caption := fmt.Sprintf("%s", chatMember.MemberUser().FirstName)
	if chatMember.MemberUser().Username != "" {
		caption += fmt.Sprintf(" (@%s)", chatMember.MemberUser().Username)
	}
	caption += fmt.Sprintf(" поймал это чудо %s", catch.CreatedAt)

	switch catch.MediaType {
	case model.MediaTypeImage:
		_, err = ctx.Bot().SendPhoto(
			ctx,
			tu.Photo(tu.ID(chatID), telego.InputFile{FileID: catch.TelegramFileID}).
				WithCaption(caption).
				WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)),
		)
	case model.MediaTypeVideo:
		_, err = ctx.Bot().SendVideo(
			ctx,
			tu.Video(tu.ID(chatID), telego.InputFile{FileID: catch.TelegramFileID}).
				WithCaption(caption).
				WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)),
		)
	default:
		return SendMessage(ctx, tu.Message(tu.ID(chatID), "Unsupported media type"))
	}

	return nil
}

func notifyAllModeratorsAboutNewCatch(ctx *th.Context, app App, catch *model.Catch) error {
	moderators, err := app.GetModerators(ctx)
	if err != nil {
		return err
	}

	for _, moderator := range moderators {
		_, _ = ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(moderator.ChatID), "There is new catch!"))
	}

	return nil
}

func NextPendingCatchCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		//catch, err := app.NextPendingCatch(ctx, query.From.ID)
		//if err != nil {
		//	return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("failed to get next pending catch"))
		//}
		//
		//keyboardRows := [][]telego.InlineKeyboardButton{
		//	tu.InlineKeyboardRow(tu.InlineKeyboardButton("Reject").WithCallbackData(fmt.Sprintf("reject_catch:%d", catch.ID))),
		//}

		//if len(catch.Validations) == 2 {
		//	keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(fmt.Sprintf("Accept with size (%d)", catch.Validations[0].Size)).
		//		WithCallbackData(fmt.Sprintf("accept_catch_with_size:%d:%d", catch.ID, catch.Validations[0].Size))))
		//	keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(fmt.Sprintf("Accept with size (%d)", catch.Validations[1].Size)).
		//		WithCallbackData(fmt.Sprintf("accept_catch_with_size:%d:%d", catch.ID, catch.Validations[1].Size))))
		//} else {
		//	keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Accept").WithCallbackData(fmt.Sprintf("accept_catch:%d", catch.ID))))
		//}

		//switch catch.MediaType {
		//case model.MediaTypeImage:
		//	_, err = ctx.Bot().SendPhoto(
		//		ctx,
		//		tu.Photo(query.Message.GetChat().ChatID(), telego.InputFile{FileID: catch.TelegramFileID}).
		//			WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)),
		//	)
		//case model.MediaTypeVideo:
		//	_, err = ctx.Bot().SendVideo(
		//		ctx,
		//		tu.Video(query.Message.GetChat().ChatID(), telego.InputFile{FileID: catch.TelegramFileID}).
		//			WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)),
		//	)
		//}
		//
		//if err != nil {
		//	return err
		//}

		return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("next_pending_catch")}
}

//func RejectCatchCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
//	return func(ctx *th.Context, query telego.CallbackQuery) error {
//		catchID, err := getIDFromCallbackData(query.Data)
//		if err != nil {
//			return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("invalid catch id"))
//		}
//
//		nextHandlerKey := nextMessageHandlerKey{
//			ChatID: query.Message.GetChat().ChatID(),
//			UserID: query.From.ID,
//		}
//		nextMessageHandlers.Store(nextHandlerKey, th.MessageHandler(func(ctx *th.Context, message telego.Message) error {
//			if len(message.Text) == 0 {
//				_, err = ctx.Bot().SendMessage(
//					ctx,
//					tu.Message(query.Message.GetChat().ChatID(), "Empty reason is not allowed"),
//				)
//				return err
//			}
//
//			var catch *model.Catch
//			var rejectedNow bool
//			catch, rejectedNow, err = app.RejectCatch(ctx, catchID, query.From.ID, message.Text)
//			if err != nil {
//				fmt.Println(err)
//				return err
//			}
//
//			err = DefaultResponseByUserID(ctx, app, message.From.ID, fmt.Sprintf("Catch #%d rejected", catch.ID))
//			if err != nil {
//				fmt.Println(err)
//			}
//
//			if rejectedNow && message.From.ID != catch.User.ID {
//				err = DefaultResponse(ctx, app, &catch.User, fmt.Sprintf("Your catch was rejected: %s", message.Text))
//				if err != nil {
//					fmt.Println(err)
//				}
//			} else {
//
//			}
//
//			return nil
//		}))
//
//		_, err = ctx.Bot().SendMessage(
//			ctx,
//			tu.Message(query.Message.GetChat().ChatID(), "Please, provide reason for rejection:"),
//		)
//		if err != nil {
//			nextMessageHandlers.Delete(nextHandlerKey)
//			return err
//		}
//
//		return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
//	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("reject_catch")}
//}

//func AcceptCatchCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
//	return func(ctx *th.Context, query telego.CallbackQuery) error {
//		catchID, err := getIDFromCallbackData(query.Data)
//		if err != nil {
//			return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("invalid catch id"))
//		}
//
//		nextHandlerKey := nextMessageHandlerKey{
//			ChatID: query.Message.GetChat().ChatID(),
//			UserID: query.From.ID,
//		}
//		nextMessageHandlers.Store(nextHandlerKey, th.MessageHandler(func(ctx *th.Context, message telego.Message) error {
//			if len(message.Text) == 0 {
//				_, err = ctx.Bot().SendMessage(
//					ctx,
//					tu.Message(query.Message.GetChat().ChatID(), "Empty reason is not allowed"),
//				)
//				return err
//			}
//
//			var sizeInt64 int64
//			sizeInt64, err = strconv.ParseInt(message.Text, 10, 64)
//			if err != nil {
//				_, err = ctx.Bot().SendMessage(
//					ctx,
//					tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Invalid size measurement: %v", err)),
//				)
//				return err
//			}
//
//			var catch *model.Catch
//			var acceptedNow bool
//			catch, acceptedNow, err = app.AcceptCatch(ctx, catchID, query.From.ID, int(sizeInt64))
//			if err != nil {
//				_, err = ctx.Bot().SendMessage(
//					ctx,
//					tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Failed to accept catch: %v", err)),
//				)
//				return err
//			}
//
//			err = DefaultResponseByUserID(ctx, app, message.From.ID, fmt.Sprintf("Catch was accepted"))
//			if err != nil {
//				fmt.Println(err)
//			}
//
//			if acceptedNow && message.From.ID != catch.User.ID {
//				err = DefaultResponse(ctx, app, &catch.User, fmt.Sprintf("Your catch was accepted: %d", sizeInt64))
//				if err != nil {
//					fmt.Println(err)
//				}
//			}
//
//			return nil
//		}))
//
//		_, err = ctx.Bot().SendMessage(
//			ctx,
//			tu.Message(query.Message.GetChat().ChatID(), "Please, input size measurement:"),
//		)
//		if err != nil {
//			nextMessageHandlers.Delete(nextHandlerKey)
//			return err
//		}
//		return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
//	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("accept_catch")}
//}

func AcceptCatchWithSizeCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		//components := strings.Split(query.Data, ":")
		//if len(components) != 3 {
		//	return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("invalid callback data"))
		//}
		//
		//catchID, err := strconv.ParseInt(components[1], 10, 64)
		//if err != nil {
		//	return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("invalid catch id"))
		//}
		//
		//size, err := strconv.ParseInt(components[2], 10, 64)
		//if err != nil {
		//	return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("invalid size"))
		//}
		//
		//var catch *model.Catch
		//var acceptedNow bool
		//catch, acceptedNow, err = app.AcceptCatch(ctx, catchID, query.From.ID, int(size))
		//if err != nil {
		//	_, err = ctx.Bot().SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Failed to accept catch: %v", err)))
		//	if err != nil {
		//		fmt.Println(err)
		//	}
		//	return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
		//}
		//
		//_, err = ctx.Bot().SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("Catch was accepted")))
		//if err != nil {
		//	fmt.Println(err)
		//}
		//
		//if acceptedNow {
		//	_, err = ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(catch.User.ChatID), fmt.Sprintf("Your catch was accepted: %d", size)))
		//	if err != nil {
		//		fmt.Println(err)
		//	}
		//}

		return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("accept_catch_with_size")}
}
