package tg

import (
	"errors"
	"fmt"
	"github.com/vslpsl/tournament/model"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"gorm.io/gorm"
)

func RequestParticipationCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		userID, err := getIDFromCallbackData(query.Data)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Failed to get userID: %v", err)))
		}

		user, _, err := app.CreateParticipationRequest(ctx, userID)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Failed to create participation request: %v", err)))
		}

		if err = notifyAdminsAboutNewParticipationRequest(ctx, app, user); err != nil {
			fmt.Printf("failed to notify admins about new participation request: %v\n", err)
		}

		return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Вы подали заявку"))
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("participation_request_create")}
}

func HandleListParticipationRequestsCommand(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetUser(ctx, app, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		if user.Role != model.UserRoleAdmin {
			return nil
		}

		request, totalCount, err := app.NextParticipationRequest(ctx, 0)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		var chatMember telego.ChatMember
		chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
			ChatID: tu.ID(request.User.ChatID),
			UserID: request.User.ID,
		})
		if err != nil {
			return HandleError(ctx, err, message)
		}

		text := fmt.Sprintf("Participation request (%d of %d):", 1, totalCount)
		text += fmt.Sprintf("\nName: %s", chatMember.MemberUser().FirstName)
		if len(chatMember.MemberUser().Username) > 0 {
			text += fmt.Sprintf("\nUsername: @%s", chatMember.MemberUser().Username)
		}
		text += fmt.Sprintf("\nRequested at: %v", request.CreatedAt)

		keyboardRows := [][]telego.InlineKeyboardButton{
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Reject").WithCallbackData(fmt.Sprintf("participation_request_reject:%d:%d", request.ID, 0))),
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Accept").WithCallbackData(fmt.Sprintf("participation_request_accept:%d:%d", request.ID, 0))),
		}

		if totalCount > 1 {
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Next").WithCallbackData(fmt.Sprintf("participation_request_list:%d", 1))))
		}

		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Close").WithCallbackData("done")))

		reply := tu.Message(message.Chat.ChatID(), text).
			WithReplyMarkup(tu.InlineKeyboard(keyboardRows...))
		return SendMessage(ctx, reply)
	}
}

func ListParticipationRequestsCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		offset, err := getOffsetFromCallbackData(query.Data)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid offset: %v", err)))
		}

		return nextParticipationRequest(ctx, query, app, offset)
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("participation_request_list")}
}

func nextParticipationRequest(ctx *th.Context, query telego.CallbackQuery, app App, targetOffset int64) error {
	request, totalCount, err := app.NextParticipationRequest(ctx, targetOffset)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if targetOffset == 0 {
				return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "No active participation requests found"))
			}
			return nextParticipationRequest(ctx, query, app, targetOffset-1)
		}
		return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Failed to get next participation request: %v", err)))
	}

	chatMember, err := ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
		ChatID: tu.ID(request.User.ChatID),
		UserID: request.User.ID,
	})
	if err != nil {
		return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Failed to get chat member: %v", err)))
	}

	text := fmt.Sprintf("Participation request (%d of %d):", targetOffset+1, totalCount)
	text += fmt.Sprintf("\nName: %s", chatMember.MemberUser().FirstName)
	if len(chatMember.MemberUser().Username) > 0 {
		text += fmt.Sprintf("\nUsername: @%s", chatMember.MemberUser().Username)
	}
	text += fmt.Sprintf("\nRequested at: %v", request.CreatedAt)

	keyboardRows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Reject").WithCallbackData(fmt.Sprintf("participation_request_reject:%d:%d", request.ID, targetOffset))),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Accept").WithCallbackData(fmt.Sprintf("participation_request_accept:%d:%d", request.ID, targetOffset))),
	}

	if targetOffset > 0 {
		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Back").WithCallbackData(fmt.Sprintf("participation_request_list:%d", targetOffset-1))))
	}

	if targetOffset < totalCount-1 {
		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Next").WithCallbackData(fmt.Sprintf("participation_request_list:%d", targetOffset+1))))
	}

	keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Close").WithCallbackData("done")))

	return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text).WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)))
}

//
//func sendNextParticipationRequestMessage(ctx *th.Context, app App, chatID int64, messageID *int, offset int64) error {
//	request, totalCount, err := app.NextParticipationRequest(ctx, offset)
//	if err != nil {
//		if errors.Is(err, gorm.ErrRecordNotFound) {
//			return SendMessage(ctx, tu.Message(tu.ID(chatID), fmt.Sprintf("No active participation requests found")))
//		}
//		return SendMessage(ctx, tu.Message(tu.ID(chatID), fmt.Sprintf("Failed to get next participation request: %v", err)))
//	}
//	fmt.Println("offset", offset, "total count", totalCount)
//	var chatMember telego.ChatMember
//	chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
//		ChatID: tu.ID(request.User.ChatID),
//		UserID: request.User.ID,
//	})
//	if err != nil {
//		return SendMessage(ctx, tu.Message(tu.ID(chatID), fmt.Sprintf("Failed to get chat member: %v", err)))
//	}
//
//	text := fmt.Sprintf("Name: %s", chatMember.MemberUser().FirstName)
//	if len(chatMember.MemberUser().Username) > 0 {
//		text += fmt.Sprintf("\nUsername: @%s", chatMember.MemberUser().Username)
//	}
//	text += fmt.Sprintf("\nRequested at: %v", request.CreatedAt)
//
//	keyboardRows := [][]telego.InlineKeyboardButton{
//		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Reject").WithCallbackData(fmt.Sprintf("participation_request_reject:%d:%d", request.ID, offset))),
//		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Accept").WithCallbackData(fmt.Sprintf("participation_request_accept:%d:%d", request.ID, offset))),
//	}
//
//	if offset > 0 {
//		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(
//			tu.InlineKeyboardButton("Back").WithCallbackData(fmt.Sprintf("participation_request_list:%d", 0))))
//	}
//
//	reply := tu.Message(tu.ID(chatID), text).
//		WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)).WithProtectContent()
//	return SendMessage(ctx, reply)
//}

func notifyAdminsAboutNewParticipationRequest(ctx *th.Context, app App, candidate *model.User) error {
	admins, err := app.GetAdmins(ctx)
	if err != nil {
		return err
	}

	for _, admin := range admins {
		_, err = ctx.Bot().SendMessage(
			ctx, tu.Messagef(tu.ID(admin.ChatID), "You have new participation request from <a href=tg://user?id=%d>Candidate</a>", candidate.ID),
		)
		if err != nil {
			fmt.Printf("failed to notify admin about new participation request: %v\n", err)
		}
	}

	return nil
}

func AcceptParticipationRequestCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		requestID, offset, err := getIDAndOffsetFromCallbackData(query.Data)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid participation request id or offset: %v", err)))
		}

		user, _, err := app.AcceptParticipationRequest(ctx, requestID)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Failed to accept participation request: %v", err)))
		}

		if err = SendMessage(ctx, tu.Message(tu.ID(user.ChatID), "Ваша заявка была одобрена!\nДобро пожаловать на Мемориальный турнир памяти Святого Антона Краснодарского!")); err != nil {
			fmt.Println(err)
		}

		return nextParticipationRequest(ctx, query, app, offset)
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("participation_request_accept")}
}

func RejectParticipationRequestCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		requestID, offset, err := getIDAndOffsetFromCallbackData(query.Data)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid participation request id or offset: %v", err)))
		}

		var user *model.User
		user, _, err = app.RejectParticipationRequest(ctx, requestID, "Rejected by admin")
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Failed to reject participation request: %v", err)))
		}

		err = SendMessage(ctx, tu.Message(tu.ID(user.ChatID), "К сожалению, мы не смогли принять вашу заявку."))
		if err != nil {
			fmt.Println("failed to notify user about rejection:", err)
		}

		return nextParticipationRequest(ctx, query, app, offset)
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("participation_request_reject")}
}
