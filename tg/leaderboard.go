package tg

import (
	"fmt"
	"github.com/vslpsl/tournament/model"
	"slices"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func HandleLeaderboardCommand(app App) th.MessageHandler {
	return HandleLeaderboardMessage(app)
}

func HandleLeaderboardMessage(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		participants, err := app.Participants(ctx)
		if err != nil {
			return SendMessage(ctx, tu.Message(message.Chat.ChatID(), fmt.Sprintf("Something went wrong: %v", err)))
		}

		filteredUsers := make([]*model.User, 0, len(participants))
		for _, p := range participants {
			if len(p.Catches) == 0 {
				continue
			}
			filteredCatches := make([]model.Catch, 0, len(p.Catches))
			for _, c := range p.Catches {
				if c.Accepted.Valid {
					filteredCatches = append(filteredCatches, c)
				}
			}

			if len(filteredCatches) == 0 {
				continue
			}

			slices.SortFunc(filteredCatches, func(a, b model.Catch) int {
				if a.Size < b.Size {
					return 1
				} else if a.Size > b.Size {
					return -1
				}
				return 0
			})

			if len(filteredCatches) >= 3 {
				filteredCatches = filteredCatches[:3]
			}
			p.Catches = filteredCatches
			filteredUsers = append(filteredUsers, p)
		}

		slices.SortFunc(filteredUsers, func(a, b *model.User) int {
			var aSize int
			for _, c := range a.Catches {
				aSize += c.Size
			}
			var bSize int
			for _, c := range b.Catches {
				bSize += c.Size
			}
			if aSize < bSize {
				return 1
			} else if aSize > bSize {
				return -1
			}
			return 0
		})

		text := fmt.Sprintf("Leaderboard:")
		for i, p := range filteredUsers {
			var size int
			var sizes []string
			for _, c := range p.Catches {
				size += c.Size
				sizes = append(sizes, fmt.Sprintf("%d", c.Size))
			}

			var chatMember telego.ChatMember
			chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
				ChatID: tu.ID(p.ChatID),
				UserID: p.ID,
			})
			if err != nil {
				fmt.Println("failed to get chat member:", err)
			}

			name := chatMember.MemberUser().FirstName
			if chatMember.MemberUser().Username != "" {
				name += fmt.Sprintf("(@%s)", chatMember.MemberUser().Username)
			}

			text += fmt.Sprintf("\n%d: %s %d", i+1, name, size)
			if len(sizes) > 0 {
				text += fmt.Sprintf(" (%s)", strings.Join(sizes, "+"))
			}
		}

		return SendMessage(ctx, tu.Message(message.Chat.ChatID(), text))
	}
}

func LeaderboardCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		participants, err := app.Participants(ctx)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("Something went wrong: %v", err)))
		}

		for _, p := range participants {
			acceptedCatches := make([]model.Catch, 0, len(p.Catches))
			for _, c := range p.Catches {
				if c.Accepted.Valid && c.Accepted.Bool {
					acceptedCatches = append(acceptedCatches, c)
				}
			}
			slices.SortFunc(acceptedCatches, func(a, b model.Catch) int {
				if a.Size < b.Size {
					return 1
				} else if a.Size > b.Size {
					return -1
				}
				return 0
			})

			if len(acceptedCatches) > 3 {
				acceptedCatches = acceptedCatches[:3]
			}
			p.Catches = acceptedCatches
		}

		slices.SortFunc(participants, func(a, b *model.User) int {
			var aSize int
			for _, c := range a.Catches {
				aSize += c.Size
			}
			var bSize int
			for _, c := range b.Catches {
				bSize += c.Size
			}
			if aSize < bSize {
				return 1
			} else if aSize > bSize {
				return -1
			}

			var latestACatch time.Time
			for _, c := range a.Catches {
				if c.CreatedAt.After(latestACatch) {
					latestACatch = c.CreatedAt
				}
			}
			var latestBCatch time.Time
			for _, c := range b.Catches {
				if c.CreatedAt.After(latestBCatch) {
					latestBCatch = c.CreatedAt
				}
			}

			if latestACatch.Before(latestBCatch) {
				return 1
			} else if latestACatch.After(latestBCatch) {
				return -1
			}

			return 0
		})

		text := fmt.Sprintf("Leaderboard:")
		for i, p := range participants {
			var size int
			var sizes []string
			for _, c := range p.Catches {
				size += c.Size
				sizes = append(sizes, fmt.Sprintf("%.1f", float64(c.Size)/10))
			}

			var chatMember telego.ChatMember
			chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
				ChatID: tu.ID(p.ChatID),
				UserID: p.ID,
			})
			if err != nil {
				fmt.Println("failed to get chat member:", err)
			}

			name := chatMember.MemberUser().FirstName
			if chatMember.MemberUser().Username != "" {
				name += fmt.Sprintf("(@%s)", chatMember.MemberUser().Username)
			}

			text += fmt.Sprintf("\n%d: %s %.1f", i+1, name, float64(size)/10)
			if len(sizes) > 0 {
				text += fmt.Sprintf(" (%s)", strings.Join(sizes, "+"))
			}
		}

		return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text))
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataEqual("leaderboard")}
}

//func LeaderboardCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
//	return func(ctx *th.Context, query telego.CallbackQuery) error {
//		participants, err := app.Participants(ctx)
//		if err != nil {
//			_, err = ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(query.Message.GetChat().ID), fmt.Sprintf("Something went wrong: %v", err)))
//			return err
//		}
//
//		filteredUsers := make([]*model.User, 0, len(participants))
//		for _, p := range participants {
//			if len(p.Catches) == 0 {
//				continue
//			}
//			filteredCatches := make([]model.Catch, 0, len(p.Catches))
//			for _, c := range p.Catches {
//				if c.Accepted.Valid {
//					filteredCatches = append(filteredCatches, c)
//				}
//			}
//
//			if len(filteredCatches) == 0 {
//				continue
//			}
//
//			slices.SortFunc(filteredCatches, func(a, b model.Catch) int {
//				if a.Size < b.Size {
//					return 1
//				} else if a.Size > b.Size {
//					return -1
//				}
//				return 0
//			})
//
//			if len(filteredCatches) >= 3 {
//				filteredCatches = filteredCatches[:3]
//			}
//			p.Catches = filteredCatches
//			filteredUsers = append(filteredUsers, p)
//		}
//
//		slices.SortFunc(filteredUsers, func(a, b *model.User) int {
//			var aSize int
//			for _, c := range a.Catches {
//				aSize += c.Size
//			}
//			var bSize int
//			for _, c := range b.Catches {
//				bSize += c.Size
//			}
//			if aSize < bSize {
//				return 1
//			} else if aSize > bSize {
//				return -1
//			}
//			return 0
//		})
//
//		text := fmt.Sprintf("Leaderboard:")
//		for i, p := range filteredUsers {
//			var size int
//			var sizes []string
//			for _, c := range p.Catches {
//				size += c.Size
//				sizes = append(sizes, fmt.Sprintf("%d", c.Size))
//			}
//
//			var chatMember telego.ChatMember
//			chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
//				ChatID: tu.ID(p.ChatID),
//				UserID: p.ID,
//			})
//			if err != nil {
//				fmt.Println("failed to get chat member:", err)
//			}
//
//			name := chatMember.MemberUser().FirstName
//			if chatMember.MemberUser().Username != "" {
//				name += fmt.Sprintf("(@%s)", chatMember.MemberUser().Username)
//			}
//
//			text += fmt.Sprintf("\n%d: %s %d", i+1, name, size)
//			if len(sizes) > 0 {
//				text += fmt.Sprintf(" (%s)", strings.Join(sizes, "+"))
//			}
//		}
//
//		err = DefaultResponseByUserID(ctx, app, query.From.ID, text)
//		if err != nil {
//			return err
//		}
//
//		return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
//	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataEqual("leaderboard")}
//}
