package tg

import (
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func HandleHelpCommand(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetUser(ctx, app, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		_ = user
		// todo: adjust help with user data
		help := `
/start - start/restart bot
/help - show this message
`
		return SendMessage(ctx, tu.Message(message.Chat.ChatID(), help))
	}
}
