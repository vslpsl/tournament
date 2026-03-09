package tg

import (
	"fmt"

	"github.com/vslpsl/tournament/model"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func HandleUserStartCommand(ctx *th.Context, user *model.User, app App, message telego.Message) error {
	if err := sendMessageWithUserCommands(ctx, user, "Поехали!"); err != nil {
		fmt.Println(err)
	}

	return HandleTournamentMessage(app)(ctx, message)
}

func sendMessageWithUserCommands(ctx *th.Context, user *model.User, text string) error {
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

	if len(keyboardRows) > 0 {
		replyMessage = replyMessage.WithReplyMarkup(tu.Keyboard(keyboardRows...).WithResizeKeyboard())
	}

	return SendMessage(ctx, replyMessage)
}

func ListParticipantsCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
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

		participants, totalCount, err := app.ListParticipants(ctx, offset, ListRequestsLimit)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), err.Error()))
		}

		var keyboardRows [][]telego.InlineKeyboardButton

		for _, participant := range participants {
			var chatMember telego.ChatMember
			chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
				ChatID: tu.ID(participant.ChatID),
				UserID: participant.ID,
			})
			if err != nil {
				fmt.Println("failed to get chat member:", err)
				continue
			}

			text := fmt.Sprintf("%s", chatMember.MemberUser().FirstName)
			if chatMember.MemberUser().Username != "" {
				text += fmt.Sprintf(" (@%s)", chatMember.MemberUser().Username)
			}
			keyboardRows = append(
				keyboardRows, tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(text).WithCallbackData(fmt.Sprintf("participants_list:%d", offset)),
					tu.InlineKeyboardButton("Поимочки").WithCallbackData(fmt.Sprintf("catch_list:%d:0", participant.ID)),
				),
			)
		}

		if offset > 0 {
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Предыдущие").WithCallbackData(fmt.Sprintf("participants_list:%d", offset-ListRequestsLimit))))
		}

		if offset+int64(len(participants)) < totalCount {
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Следующие").WithCallbackData(fmt.Sprintf("participants_list:%d", offset+int64(len(participants))))))
		}

		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))

		return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Списочек участничков").
			WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)))
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("participants_list")}
}
