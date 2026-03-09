package tg

import (
	"fmt"
	"github.com/vslpsl/tournament/model"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func HandleAdminStartCommand(ctx *th.Context, user *model.User, message telego.Message) error {
	commands := []telego.BotCommand{
		{Command: "start", Description: "Start the bot"},
		//{Command: "rules", Description: "Rules and regulations"},
		//{Command: "participate", Description: "Participate in the competition"},
		{Command: "participation_requests", Description: "List participation requests"},
		//{Command: "users", Description: "List users"},
		//{Command: "leaderboard", Description: "Leaderboard"},
		{Command: "help", Description: "Help"},
	}

	err := ctx.Bot().SetMyCommands(ctx, &telego.SetMyCommandsParams{
		Commands: commands,
		Scope:    tu.ScopeChat(message.Chat.ChatID()),
	})
	if err != nil {
		return err
	}

	replyMessage := tu.Message(message.Chat.ChatID(), fmt.Sprintf("Поехали!"))

	var keyboardRows [][]telego.KeyboardButton

	keyboardRows = append(keyboardRows, tu.KeyboardRow(tu.KeyboardButton(TournamentButton)))

	if len(keyboardRows) > 0 {
		replyMessage = replyMessage.WithReplyMarkup(tu.Keyboard(keyboardRows...).WithResizeKeyboard())
	}

	return SendMessage(ctx, replyMessage)
}

func defaultAdminResponse(ctx *th.Context, user *model.User, text string) error {
	keyboardRows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Rules and regulations").
			WithCallbackData("rules_and_regulations")),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Leaderboard").
			WithCallbackData("leaderboard")),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("List participation requests").
			WithCallbackData("participation_request_list:0")),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("List users").
			WithCallbackData("user_list:0")),
	}

	if !user.IsParticipant && !user.ParticipationRequestIsSent {
		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Participate").
			WithCallbackData(fmt.Sprintf("request_participation:%d", user.ID))))
	}

	if user.IsParticipant {
		if time.Now().After(competitionStartDate) {
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("I've catch a fish!").
				WithCallbackData(fmt.Sprintf("catch_request:%d", user.ID))))
		}

	}

	_, err := ctx.Bot().SendMessage(
		ctx,
		tu.Message(
			tu.ID(user.ChatID),
			text,
		).
			WithReplyMarkup(
				tu.InlineKeyboard(
					keyboardRows...,
				),
			),
	)

	return err
}

func HandleListUsersCommand(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetUser(ctx, app, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		if user.Role != model.UserRoleAdmin {
			return nil
		}

		return listUsers(ctx, app, message.Chat.ID, nil, 0)
	}
}

func listUsers(ctx *th.Context, app App, chatID int64, messageID *int, offset int64) error {
	users, totalCount, err := app.ListUsers(ctx, offset, ListRequestsLimit)
	if err != nil {
		return SendMessage(ctx, tu.Message(tu.ID(chatID), fmt.Sprintf("failed to list users: %v", err)))
	}

	var keyboardRows [][]telego.InlineKeyboardButton

	for _, user := range users {
		var chatMember telego.ChatMember
		chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
			ChatID: tu.ID(user.ChatID),
			UserID: user.ID,
		})
		if err != nil {
			fmt.Printf("failed to get chat member: %v\n", err)
			continue
		}

		participationMark := "+"
		if !user.IsParticipant {
			participationMark = "-"
		}

		keyboardRows = append(
			keyboardRows,
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(fmt.Sprintf("%s(@%s) %s %s", chatMember.MemberUser().FirstName, chatMember.MemberUser().Username, user.Role, participationMark)).
					WithCallbackData(fmt.Sprintf("user_details:%d:%d", user.ID, offset)),
			),
		)
	}

	if offset > 0 {
		keyboardRows = append(
			keyboardRows,
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(fmt.Sprintf("Previous %d", ListRequestsLimit)).
					WithCallbackData(fmt.Sprintf("user_list:%d", offset-ListRequestsLimit)),
			),
		)
	}

	if offset+int64(len(users)) < totalCount {
		keyboardRows = append(
			keyboardRows,
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(fmt.Sprintf("Next %d", totalCount-(offset+int64(len(users))))).
					WithCallbackData(fmt.Sprintf("user_list:%d", offset+ListRequestsLimit)),
			),
		)
	}

	keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Done").WithCallbackData("done")))

	text := fmt.Sprintf("Here is list of users from %d to %d (total: %d)", offset+1, offset+int64(len(users)), totalCount)

	if messageID != nil {
		_, err = ctx.Bot().EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    tu.ID(chatID),
			MessageID: *messageID,
			Text:      text,
			//ParseMode:          "",
			ReplyMarkup: tu.InlineKeyboard(keyboardRows...),
		})
	} else {
		err = SendMessage(ctx, tu.Message(tu.ID(chatID), text).WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)))
	}

	if err != nil {
		fmt.Printf("failed to send message: %v\n", err)
	}

	return nil
}

func ListUsersCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
		}()

		offset, err := getOffsetFromCallbackData(query.Data)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("invalid offset: %v", err)))
		}

		return listUsers(ctx, app, query.Message.GetChat().ID, &query.Message.Message().MessageID, offset)
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("user_list")}
}

func UserDetailsCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("invalid user id"))
		}()

		userID, offset, err := getIDAndOffsetFromCallbackData(query.Data)
		if err != nil {
			return err
		}

		user, err := app.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}

		chatMember, err := ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
			ChatID: tu.ID(user.ChatID),
			UserID: user.ID,
		})
		if err != nil {
			return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("failed to get chat member"))
		}

		text := fmt.Sprintf("ID: %d\n", user.ID)
		text += fmt.Sprintf("FirstName: %s", chatMember.MemberUser().FirstName)
		text += fmt.Sprintf("\nUsername: @%s", chatMember.MemberUser().Username)
		text += fmt.Sprintf("\nRole: %s", user.Role)
		text += fmt.Sprintf("\nParticipationRequestIsSent: %t", user.ParticipationRequestIsSent)
		text += fmt.Sprintf("\nIsParticipant: %t", user.IsParticipant)

		var keyboardRows [][]telego.InlineKeyboardButton

		if user.Role == model.UserRoleUser {
			keyboardRows = append(
				keyboardRows,
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Set moderator role").
						WithCallbackData(fmt.Sprintf("user_set_moderator_role:%d", user.ID)),
				),
			)
		}

		if user.Role == model.UserRoleModerator {
			keyboardRows = append(
				keyboardRows,
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("Set user role").
						WithCallbackData(fmt.Sprintf("user_set_user_role:%d", user.ID)),
				),
			)
		}

		keyboardRows = append(
			keyboardRows,
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Back").
					WithCallbackData(fmt.Sprintf("user_list:%d", offset)),
			),
		)

		_, err = ctx.Bot().EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      query.Message.GetChat().ChatID(),
			MessageID:   query.Message.Message().MessageID,
			Text:        text,
			ReplyMarkup: tu.InlineKeyboard(keyboardRows...),
		})

		return err
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("user_details")}
}

func SetModeratorRoleForUserCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
		}()
		userID, err := getIDFromCallbackData(query.Data)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("invalid user id: %v", err)))
		}

		user, err := app.SetUserRole(ctx, userID, model.UserRoleModerator)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("failed to set user role: %v", err)))
		}

		if err = sendMessageWithModeratorCommands(ctx, user, fmt.Sprintf("You were granted moderator role")); err != nil {
			fmt.Println("failed to send message:", err)
		}

		notifyMsg := tu.Message(telego.ChatID{}, fmt.Sprintf("%d has become moderator", user.ID))

		if err = notifyParticipants(ctx, app, notifyMsg, user.ID); err != nil {
			fmt.Println("failed to notify participants:", err)
		}

		return ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    query.Message.GetChat().ChatID(),
			MessageID: query.Message.Message().MessageID,
		})
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("user_set_moderator_role")}
}

func SetUserRoleForUserCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
		}()
		userID, err := getIDFromCallbackData(query.Data)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("invalid user id: %v", err)))
		}

		user, err := app.SetUserRole(ctx, userID, model.UserRoleUser)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), fmt.Sprintf("failed to set user role: %v", err)))
		}

		if err = sendMessageWithUserCommands(ctx, user, fmt.Sprintf("You are no longer moderator, sorry.")); err != nil {
			fmt.Println("failed to send message:", err)
		}

		notifyMsg := tu.Message(telego.ChatID{}, fmt.Sprintf("%d is no longer moderator", user.ID))
		if err = notifyParticipants(ctx, app, notifyMsg, user.ID); err != nil {
			fmt.Println("failed to notify participants:", err)
		}

		return ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    query.Message.GetChat().ChatID(),
			MessageID: query.Message.Message().MessageID,
		})
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("user_set_user_role")}
}
