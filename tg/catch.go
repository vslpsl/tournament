package tg

import (
	"errors"
	"fmt"
	"github.com/vslpsl/tournament/model"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func CreateCatchMessageHandler(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		if message.Video == nil && len(message.Photo) == 0 && (message.Document == nil || !strings.HasPrefix(message.Document.MimeType, "image")) {
			return ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
				ChatID:    message.Chat.ChatID(),
				MessageID: message.MessageID,
			})
		}

		if time.Now().Before(competitionStartDate) {
			return ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
				ChatID:    message.Chat.ChatID(),
				MessageID: message.MessageID,
			})
		}

		user, err := GetUser(ctx, app, message)
		if err != nil {
			if errors.Is(err, model.ErrUserNotFound) {
				return ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
					ChatID:    message.Chat.ChatID(),
					MessageID: message.MessageID,
				})
			}
			return err
		}

		if !user.IsParticipant {
			return SendMessage(ctx, tu.Message(message.Chat.ChatID(), "Для начала, неплохо было бы зарегистрироваться!"))
		}

		if time.Now().Before(competitionStartDate) {
			return SendMessage(ctx, tu.Message(message.Chat.ChatID(), "Дождитесь начала турнира."))
		}

		var fileID string
		var fileUniqueID string
		var fileName string
		var mediaType string

		if message.Video != nil {
			if message.Video.FileSize > 1024*1024*20 {
				return SendMessage(ctx, tu.Message(message.Chat.ChatID(), "Слишком большой размер файла."))
			}

			fileID = message.Video.FileID
			fileUniqueID = message.Video.FileUniqueID
			fileName = message.Video.FileName
			mediaType = model.MediaTypeVideo
		} else if len(message.Photo) > 0 {
			photo := message.Photo[len(message.Photo)-1]
			fileID = photo.FileID
			fileUniqueID = photo.FileUniqueID
			fileName = photo.FileUniqueID
			mediaType = model.MediaTypeImage
		} else if message.Document != nil {
			fileName = message.Document.FileName
			fileID = message.Document.FileID
			fileUniqueID = message.Document.FileUniqueID
			mediaType = model.MediaTypeImage
		} else {
			_, err = ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(user.ChatID), "Будьте любезны, фото или видео."))
			return err
		}

		var file *telego.File
		file, err = ctx.Bot().GetFile(ctx, &telego.GetFileParams{FileID: fileID})
		if err != nil {
			return err
		}

		downloadUrl := ctx.Bot().FileDownloadURL(file.FilePath)
		var resp *http.Response
		resp, err = http.Get(downloadUrl)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return err
		}

		var filePath string

		filePath, err = app.CreateData(user.ID, fileName, resp.Body)
		if err != nil {
			return err
		}

		catch := &model.Catch{
			UserID:               user.ID,
			DataFilePath:         filePath,
			FileName:             fileName,
			TelegramFileID:       fileID,
			TelegramFileUniqueID: fileUniqueID,
			MediaType:            mediaType,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}

		if err = app.CreateCatch(ctx, catch); err != nil {
			return err
		}

		if err = SendMessage(ctx, tu.Message(message.Chat.ChatID(), "Ваш улов зарегистрирован!")); err != nil {
			fmt.Println(err)
		}

		text := fmt.Sprintf("%s", message.From.FirstName)
		if message.From.Username != "" {
			text += fmt.Sprintf(" (@%s)", message.From.Username)
		}
		text += " поймал рыбку!\n"
		reviewCommand := fmt.Sprintf("Посмотреть (/review_catch%d)", catch.ID)
		notifyMsg := tu.MessageWithEntities(telego.ChatID{}, tu.Entity(text), tu.Entity(reviewCommand).BotCommand())

		if err = notifyParticipants(ctx, app, notifyMsg, user.ID); err != nil {
			fmt.Println("failed to notify participants:", err)
		}

		return nil
	}
}

func HandleReviewCatchCommand(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetUser(ctx, app, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}
		if !user.IsParticipant {
			return HandleError(ctx, errors.New("Для начала, хорошо бы принять участие в турнире!"), message)
		}

		catchIDString := strings.TrimPrefix(message.Text, "/review_catch")
		catchID, err := strconv.Atoi(catchIDString)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		catch, err := app.GetCatch(ctx, int64(catchID))
		if err != nil {
			return err
		}

		if catch.UserID == user.ID {
			return HandleError(ctx, errors.New("Нельзя оценивать собственные достижения"), message)
		}

		var review *model.CatchReview
		for _, r := range catch.Reviews {
			if r.ReviewerID == user.ID {
				review = &r
				break
			}
		}

		chatMember, err := ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{
			ChatID: tu.ID(catch.UserID),
			UserID: catch.UserID,
		})
		if err != nil {
			return HandleError(ctx, err, message)
		}

		caption := fmt.Sprintf("%s ", chatMember.MemberUser().FirstName)
		if chatMember.MemberUser().Username != "" {
			caption += fmt.Sprintf("(@%s) ", chatMember.MemberUser().Username)
		}
		caption += fmt.Sprintf("поймал это %s", catch.CreatedAt.Format("2006-01-02 15:04:05"))
		var keyboardRows [][]telego.InlineKeyboardButton
		if review != nil {
			if review.Accepted {
				var condition string
				if review.Condition == model.ConditionShore {
					condition = "берегового"
				} else {
					condition = "офшорного"
				}
				caption += fmt.Sprintf("\nВы оценили %s окушка в %.1f см", condition, float64(review.Size)/10)
				keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))
			} else {
				caption += fmt.Sprintf("\nВы забраковали этот хлам")
				keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))
			}
		} else {
			caption += "\nОцените размер окушка, пожалуйста"
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оценить").WithCallbackData(fmt.Sprintf("review_catch:%d", catch.ID))))
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Зарежектить").WithCallbackData(fmt.Sprintf("review_catch:%d:any:shore:0:0", catch.ID))))
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))
		}

		switch catch.MediaType {
		case model.MediaTypeImage:
			_, err = ctx.Bot().SendPhoto(
				ctx,
				tu.Photo(message.GetChat().ChatID(), telego.InputFile{FileID: catch.TelegramFileID}).
					WithCaption(caption).
					WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)),
			)
		case model.MediaTypeVideo:
			_, err = ctx.Bot().SendVideo(
				ctx,
				tu.Video(message.GetChat().ChatID(), telego.InputFile{FileID: catch.TelegramFileID}).
					WithCaption(caption).
					WithReplyMarkup(tu.InlineKeyboard(keyboardRows...)),
			)
		}

		return nil
	}
}

func ReviewCatchCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		var catchID int64
		var species *model.Species
		var condition *string
		var size *int64
		var accepted *bool

		components := strings.Split(query.Data, ":")
		if len(components) < 2 {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "invalid callback data"))
		}

		catchID, err := strconv.ParseInt(components[1], 10, 64)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid catch id: %v", err)))
		}

		if len(components) > 2 {
			rawSpecies := model.Species(components[2])
			if !rawSpecies.IsValid() {
				return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "invalid species: "+string(rawSpecies)))
			}
			species = &rawSpecies
		}

		if len(components) > 3 {
			condition = &components[3]
			if *condition != model.ConditionShore && *condition != model.ConditionOffshore {
				return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "invalid condition"))
			}
		}

		if len(components) > 4 {
			var rawSize int64
			rawSize, err = strconv.ParseInt(components[4], 10, 64)
			if err != nil {
				return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid size: %v", err)))
			}
			size = &rawSize
		}

		if len(components) > 5 {
			var rawAccepted bool
			rawAccepted, err = strconv.ParseBool(components[5])
			if err != nil {
				return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid accepted value: %v", err)))
			}

			accepted = &rawAccepted
		}

		if species == nil {
			var keyboardRows [][]telego.InlineKeyboardButton
			for _, s := range model.SpeciesList {
				keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(s.Translation()).WithCallbackData(fmt.Sprintf("review_catch:%d:%s", catchID, string(s)))))
			}
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))

			_, err = ctx.Bot().EditMessageCaption(ctx, &telego.EditMessageCaptionParams{
				ChatID:      query.Message.GetChat().ChatID(),
				MessageID:   query.Message.GetMessageID(),
				Caption:     "Выберите вид рыбейки",
				ReplyMarkup: tu.InlineKeyboard(keyboardRows...),
			})
			return err
		}

		if condition == nil {
			keyboardRows := [][]telego.InlineKeyboardButton{
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Берег").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:shore", catchID, *species))),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Плавсредство").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:offshore", catchID, *species))),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")),
			}

			_, err = ctx.Bot().EditMessageCaption(ctx, &telego.EditMessageCaptionParams{
				ChatID:      query.Message.GetChat().ChatID(),
				MessageID:   query.Message.GetMessageID(),
				Caption:     "Выберите условия поимки",
				ReplyMarkup: tu.InlineKeyboard(keyboardRows...),
			})
			return err
		}

		if accepted == nil {
			sizeString := ""
			if size != nil {
				sizeString = strconv.Itoa(int(*size))
			}

			var backCallbackData string
			if sizeString == "" {
				backCallbackData = fmt.Sprintf("review_catch:%d:%s", catchID, *species)
			} else {
				previousSizeString := sizeString[:len(sizeString)-1]
				if previousSizeString == "" {
					backCallbackData = fmt.Sprintf("review_catch:%d:%s:%s", catchID, *species, *condition)
				} else {
					backCallbackData = fmt.Sprintf("review_catch:%d:%s:%s:%s", catchID, *species, *condition, previousSizeString)
				}
			}

			keyboardRows := [][]telego.InlineKeyboardButton{
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("1").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s1", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("2").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s2", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("3").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s3", catchID, *species, *condition, sizeString)),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("4").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s4", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("5").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s5", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("6").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s6", catchID, *species, *condition, sizeString)),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("7").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s7", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("8").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s8", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("9").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s9", catchID, *species, *condition, sizeString)),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(" ").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("0").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s0", catchID, *species, *condition, sizeString)),
					tu.InlineKeyboardButton("<").WithCallbackData(backCallbackData),
				),
			}

			if len(sizeString) > 0 && size != nil && *size > 0 {
				//keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("No op spacer")))
				sizeInCm := float64(*size) / 10
				keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(fmt.Sprintf("Зафиксировать %.1f см", sizeInCm)).WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s:1", catchID, *species, *condition, sizeString))))
			} else {
				keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(" ").WithCallbackData(fmt.Sprintf("review_catch:%d:%s:%s:%s", catchID, *species, *condition, sizeString))))
			}

			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))

			_, err = ctx.Bot().EditMessageCaption(ctx, &telego.EditMessageCaptionParams{
				ChatID:      query.Message.GetChat().ChatID(),
				MessageID:   query.Message.GetMessageID(),
				Caption:     fmt.Sprintf("Укажите длину в миллиметрах:"),
				ReplyMarkup: tu.InlineKeyboard(keyboardRows...),
			})
			return err
		}

		_, err = ctx.Bot().EditMessageCaption(ctx, &telego.EditMessageCaptionParams{
			ChatID:    query.Message.GetChat().ChatID(),
			MessageID: query.Message.GetMessageID(),
			Caption:   "",
		})
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), err.Error()))
		}

		catch, err := app.GetCatch(ctx, catchID)
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), err.Error()))
		}

		if *accepted {
			_, err = app.CreateAcceptedCatchReview(ctx, catchID, query.From.ID, *species, int(*size), *condition)
			if err != nil {
				return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), err.Error()))
			}

			acceptMessage := "Вы зафиксировали"
			if *condition == model.ConditionShore {
				acceptMessage += " берегового"
			} else {
				acceptMessage += " офшорного"
			}
			acceptMessage += fmt.Sprintf(" %s длиной %.1f см", species.Translation(), float64(*size)/10)

			acceptNotification := fmt.Sprintf("%s", query.From.FirstName)
			if query.From.Username != "" {
				acceptNotification += fmt.Sprintf(" (@%s)", query.From.Username)
			}
			acceptNotification += " зафиксировал"
			if *condition == model.ConditionShore {
				acceptNotification += " берегового"
			} else {
				acceptNotification += " офшорного"
			}
			acceptNotification += fmt.Sprintf(" %s длиной %.1f см", species.Translation(), float64(*size)/10)

			if err = SendMessage(ctx, tu.Message(tu.ID(catch.UserID), acceptNotification)); err != nil {
				fmt.Println("failed to send message to user:", err)
			}

			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), acceptMessage))
		}

		_, err = app.CreateRejectedCatchReview(ctx, catchID, query.From.ID, "да просто так")
		if err != nil {
			return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), err.Error()))
		}

		rejectNotification := fmt.Sprintf("%s", query.From.FirstName)
		if query.From.Username != "" {
			rejectNotification += fmt.Sprintf(" (@%s)", query.From.Username)
		}
		rejectNotification += fmt.Sprintf(" зарежектил вашу поимочку #%d", catch.ID)

		if err = SendMessage(ctx, tu.Message(tu.ID(catch.UserID), rejectNotification)); err != nil {
			fmt.Println("failed to send message to user:", err)
		}

		return SendMessage(ctx, tu.Message(query.Message.GetChat().ChatID(), "Вы зарежектили эту шляпу!"))
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("review_catch")}
}

func ListCatchesCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()

		components := strings.Split(query.Data, ":")
		if len(components) < 3 {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "invalid callback data"))
		}

		userID, err := strconv.ParseInt(components[1], 10, 64)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid user id: %v", err)))
		}

		offset, err := strconv.ParseInt(components[2], 10, 64)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), fmt.Sprintf("invalid catch id: %v", err)))
		}

		catches, totalCount, err := app.ListCatches(ctx, userID, false, offset, ListRequestsLimit)
		if err != nil {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), err.Error()))
		}

		if totalCount == 0 {
			return EditMessage(ctx, tu.EditMessageText(query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Поимочки не найдены"))
		}

		if err = ctx.Bot().DeleteMessage(ctx, tu.Delete(query.Message.GetChat().ChatID(), query.Message.GetMessageID())); err != nil {
			fmt.Println("failed to delete message:", err)
		}

		for _, catch := range catches {
			caption := "Улов"
			if catch.Accepted.Valid {
				if catch.Accepted.Bool {
					caption += " засчитан\n"
					caption += fmt.Sprintf("Длина: %.1f см\n", float64(catch.Size)/10)
					if catch.Condition == model.ConditionShore {
						caption += "Пойман с берега!\n"
					}
				} else {
					caption += " не засчитан"
				}
			} else {
				caption += " ожидает валидации"
				for _, review := range catch.Reviews {
					var chatMember telego.ChatMember
					chatMember, err = ctx.Bot().GetChatMember(ctx, &telego.GetChatMemberParams{ChatID: tu.ID(review.ReviewerID), UserID: review.ReviewerID})
					if err != nil {
						fmt.Println("failed to get chat member:", err)
						continue
					}

					caption += fmt.Sprintf("\n%s", chatMember.MemberUser().FirstName)
					if chatMember.MemberUser().Username != "" {
						caption += fmt.Sprintf(" (@%s) считает, что вы поймали", chatMember.MemberUser().Username)
					}

					if review.Accepted {
						if review.Condition == model.ConditionShore {
							caption += " берегового"
						} else {
							caption += " офшорного"
						}
						caption += fmt.Sprintf(" окушка длиной %.1f см", float64(review.Size)/10)
					} else {
						caption += " хуету"
					}
				}
			}

			switch catch.MediaType {
			case model.MediaTypeImage:
				_, err = ctx.Bot().SendPhoto(
					ctx,
					tu.Photo(query.Message.GetChat().ChatID(), telego.InputFile{FileID: catch.TelegramFileID}).WithCaption(caption),
				)
			case model.MediaTypeVideo:
				_, err = ctx.Bot().SendVideo(
					ctx,
					tu.Video(query.Message.GetChat().ChatID(), telego.InputFile{FileID: catch.TelegramFileID}).WithCaption(caption),
				)
			}
		}

		text := ""
		var keyboardRows [][]telego.InlineKeyboardButton
		if offset+int64(len(catches)) >= totalCount {
			text = "Вот и все ваши поимочки"
		} else {
			text = fmt.Sprintf("Есть еще %d", totalCount-offset-int64(len(catches)))
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Следующие").WithCallbackData(fmt.Sprintf("catch_list:%d:%d", userID, offset+int64(len(catches))))))
			keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))
		}

		replyMsg := tu.Message(query.Message.GetChat().ChatID(), text)
		if len(keyboardRows) > 0 {
			replyMsg.ReplyMarkup = tu.InlineKeyboard(keyboardRows...)
		}

		return SendMessage(ctx, replyMsg)

	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("catch_list")}
}

func ListCatchesForReviewCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		return nil
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataPrefix("catch_list_for_review")}
}
