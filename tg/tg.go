package tg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vslpsl/tournament/model"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	ListRequestsLimit int64 = 3
)

const (
	TournamentButton  = "Мемориальный турнир памяти Святого Антона Краснодарского"
	ArbitrationButton = "Арбитраж"
)

type nextMessageHandlerKey struct {
	ChatID telego.ChatID
	UserID int64
}

var nextMessageHandlers = sync.Map{}

var (
	competitionStartDate = time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
)

type App interface {
	GetAdmins(ctx context.Context) ([]*model.User, error)
	GetModerators(ctx context.Context) ([]*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	ListUsers(ctx context.Context, offset, limit int64) ([]*model.User, int64, error)
	SetUserRole(ctx context.Context, userID int64, role string) (user *model.User, err error)

	Participants(ctx context.Context) ([]*model.User, error)
	ListParticipants(ctx context.Context, offset, limit int64) ([]*model.User, int64, error)

	CreateParticipationRequest(ctx context.Context, userID int64) (user *model.User, request *model.ParticipationRequest, err error)
	NextParticipationRequest(ctx context.Context, offset int64) (_ *model.ParticipationRequest, totalCount int64, err error)
	GetParticipationRequest(ctx context.Context, requestID int64) (*model.ParticipationRequest, error)
	AcceptParticipationRequest(ctx context.Context, requestID int64) (user *model.User, request *model.ParticipationRequest, err error)
	RejectParticipationRequest(ctx context.Context, requestID int64, reason string) (user *model.User, request *model.ParticipationRequest, err error)

	CreateData(userID int64, fileName string, data io.Reader) (string, error)

	CreateCatch(ctx context.Context, catch *model.Catch) error
	GetCatch(ctx context.Context, catchID int64) (*model.Catch, error)
	CreateAcceptedCatchReview(ctx context.Context, catchID int64, reviewerID int64, species model.Species, size int, condition string) (*model.CatchReview, error)
	CreateRejectedCatchReview(ctx context.Context, catchID int64, reviewerID int64, reason string) (*model.CatchReview, error)
	ListCatches(ctx context.Context, userID int64, asc bool, offset, limit int64) (catches []*model.Catch, totalCount int64, err error)

	NextCatchForValidation(ctx context.Context, targetCatchID int64) (_ *model.Catch, totalCount int64, err error)
	CreateCatchValidationWithReview(ctx context.Context, catchID int64, moderatorID int64, reviewID int64) (catch *model.Catch, validation *model.CatchValidation, err error)
}

func Run(token string, app App) error {
	bot, err := telego.NewBot(token, telego.WithDefaultLogger(false, true))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates, err := bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{})
	if err != nil {
		log.Fatal(err)
	}

	bh, err := th.NewBotHandler(bot, updates)
	if err != nil {
		log.Fatal(err)
	}

	defer bh.Stop()

	bh.Use(func(ctx *th.Context, update telego.Update) error {
		if update.Message == nil {
			return ctx.Next(update)
		}

		message := *update.Message
		anyHandler, loaded := nextMessageHandlers.LoadAndDelete(nextMessageHandlerKey{
			ChatID: message.Chat.ChatID(),
			UserID: message.From.ID,
		})
		if loaded {
			handler := anyHandler.(th.MessageHandler)
			return handler(ctx, message)
		}

		return ctx.Next(update)
	})

	bh.HandleMessage(HandleStartCommand(app), th.CommandEqual("start"))
	bh.HandleMessage(HandleTournamentMessage(app), th.TextEqual(TournamentButton))
	bh.HandleMessage(HandleArbitrationMessage(app), th.TextEqual(ArbitrationButton))

	bh.HandleMessage(HandleListParticipationRequestsCommand(app), th.CommandEqual("participation_requests"))
	bh.HandleMessage(HandleHelpCommand(app), th.CommandEqual("help"))

	bh.HandleMessage(HandleListUsersCommand(app), th.CommandEqual("users"))
	bh.HandleMessage(HandleReviewCatchCommand(app), th.CommandPrefix("review_catch"))
	bh.HandleMessage(CreateCatchMessageHandler(app), th.AnyMessageWithMedia())

	handlers := []func(app App) (th.CallbackQueryHandler, []th.Predicate){
		DoneCallbackQueryHandler,
		RulesCallbackQueryHandler,

		RequestParticipationCallbackQueryHandler,
		ListParticipationRequestsCallbackQueryHandler,
		AcceptParticipationRequestCallbackQueryHandler,
		RejectParticipationRequestCallbackQueryHandler,

		LeaderboardCallbackQueryHandler,

		ReviewCatchCallbackQueryHandler,
		ListCatchesCallbackQueryHandler,

		ListParticipantsCallbackQueryHandler,

		ValidateCatchCallbackQueryHandler,

		ListUsersCallbackQueryHandler,
		UserDetailsCallbackQueryHandler,
		SetModeratorRoleForUserCallbackQueryHandler,
		SetUserRoleForUserCallbackQueryHandler,

		NextPendingCatchCallbackQueryHandler,
		AcceptCatchWithSizeCallbackQueryHandler,
	}

	for _, handler := range handlers {
		callbackQueryHandler, predicates := handler(app)
		bh.HandleCallbackQuery(callbackQueryHandler, predicates...)
	}

	return bh.Start()
}

func HandleStartCommand(app App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetOrCreateUser(ctx, app, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		switch user.Role {
		case model.UserRoleAdmin:
			return HandleAdminStartCommand(ctx, user, message)
		case model.UserRoleModerator:
			return HandleModeratorStartCommand(ctx, user, app, message)
		case model.UserRoleUser:
			return HandleUserStartCommand(ctx, user, app, message)
		}

		return nil
	}
}

//
//func HandleMessage(app App) th.MessageHandler {
//	return func(ctx *th.Context, message telego.Message) error {

//
//		user, err := GetOrCreateUser(ctx, app, message)
//		if err != nil {
//			return HandleError(ctx, err, message)
//		}
//
//		if user.ID == 769933568 { // ID Олега
//			// todo: уменьшать длину поимки на 10см
//		}
//
//		switch user.Role {
//		case model.UserRoleAdmin:
//			return HandleAdminMessage(ctx, user, message)
//		case model.UserRoleModerator:
//			return HandleModeratorMessage(ctx, app, user, message)
//		case model.UserRoleUser:
//			return HandleUserMessage(ctx, user, message)
//		}
//
//		return fmt.Errorf("unknown user role: %s", user.Role)
//	}
//}

func HandleError(ctx *th.Context, err error, message telego.Message) error {
	return SendMessage(ctx, tu.Message(message.Chat.ChatID(), fmt.Sprintf("Something went wrong: %v", err)))
}

func GetOrCreateUser(ctx context.Context, app App, message telego.Message) (*model.User, error) {
	fmt.Printf("user.id: %d, chat.id: %d\n", message.From.ID, message.Chat.ID)
	user, err := app.GetUserByID(ctx, message.From.ID)
	if err != nil {
		if !errors.Is(err, model.ErrUserNotFound) {
			return nil, err
		}

		user = &model.User{
			ID:                         message.From.ID,
			ChatID:                     message.Chat.ID,
			Role:                       model.UserRoleUser,
			ParticipationRequestIsSent: false,
			IsParticipant:              false,
			CreatedAt:                  time.Now(),
			UpdatedAt:                  time.Now(),
		}

		err = app.CreateUser(ctx, user)
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func GetUser(ctx context.Context, app App, message telego.Message) (*model.User, error) {
	return app.GetUserByID(ctx, message.From.ID)
}

func getIDFromCallbackData(data string) (int64, error) {
	components := strings.Split(data, ":")
	if len(components) != 2 {
		return 0, fmt.Errorf("invalid callback data: %s", data)
	}

	return strconv.ParseInt(components[1], 10, 64)
}

func getOffsetFromCallbackData(data string) (int64, error) {
	components := strings.Split(data, ":")
	if len(components) != 2 {
		return 0, fmt.Errorf("invalid callback data: %s", data)
	}

	return strconv.ParseInt(components[1], 10, 64)
}

func getIDAndOffsetFromCallbackData(data string) (int64, int64, error) {
	components := strings.Split(data, ":")
	if len(components) != 3 {
		return 0, 0, fmt.Errorf("invalid callback data: %s", data)
	}

	id, ierr := strconv.ParseInt(components[1], 10, 64)
	offset, oerr := strconv.ParseInt(components[2], 10, 64)

	return id, offset, errors.Join(ierr, oerr)
}

func SendMessage(ctx *th.Context, params *telego.SendMessageParams) error {
	_, err := ctx.Bot().SendMessage(ctx, params)
	return err
}

func EditMessage(ctx *th.Context, params *telego.EditMessageTextParams) error {
	_, err := ctx.Bot().EditMessageText(ctx, params)
	return err
}

func DoneCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done"))
		}()
		return ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    query.Message.GetChat().ChatID(),
			MessageID: query.Message.GetMessageID(),
		})
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataEqual("done")}
}

func notifyParticipants(ctx *th.Context, db App, message *telego.SendMessageParams, excludeIDs ...int64) error {
	participants, err := db.Participants(ctx)
	if err != nil {
		return err
	}

	for _, user := range participants {
		excluded := false
		for _, excludeID := range excludeIDs {
			if user.ID == excludeID {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		_, _ = ctx.Bot().SendMessage(ctx, message.WithChatID(tu.ID(user.ChatID)))
	}

	return nil
}

func HandleTournamentMessage(db App) th.MessageHandler {
	return func(ctx *th.Context, message telego.Message) error {
		user, err := GetUser(ctx, db, message)
		if err != nil {
			return HandleError(ctx, err, message)
		}

		_ = user
		text := fmt.Sprintf("Привет, %s!", message.From.FirstName)

		if user.IsParticipant {
			if competitionStartDate.After(time.Now()) {
				text += fmt.Sprintf("\nТурнир стартует %s", competitionStartDate)
			} else {
				text += fmt.Sprintf("\nТурнир начался, поймайте рыбку и отправьте фото/видео подтверждение!")
			}
		}

		var keyboardRows [][]telego.InlineKeyboardButton

		keyboardRows = append(
			keyboardRows,
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Регламент").WithCallbackData("rules")),
		)

		if !user.IsParticipant {
			if !user.ParticipationRequestIsSent {
				keyboardRows = append(
					keyboardRows,
					tu.InlineKeyboardRow(tu.InlineKeyboardButton("Подать заявочку").
						WithCallbackData(fmt.Sprintf("participation_request_create:%d", user.ID))),
				)
			}
		} else {
			keyboardRows = append(
				keyboardRows,
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Мои поимочки").
					WithCallbackData(fmt.Sprintf("catch_list:%d:0", user.ID))),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Участники").
					WithCallbackData("participants_list:0")),
				tu.InlineKeyboardRow(tu.InlineKeyboardButton("Чужие поимочки").
					WithCallbackData(fmt.Sprintf("catch_list_for_review:%d:0", user.ID))),
			)
		}

		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Таблица лидеров").WithCallbackData("leaderboard")))

		keyboardRows = append(keyboardRows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Скрыть").WithCallbackData("done")))

		reply := tu.Message(message.Chat.ChatID(), text)
		if len(keyboardRows) > 0 {
			reply = reply.WithReplyMarkup(tu.InlineKeyboard(keyboardRows...))
		}

		return SendMessage(ctx, reply)
	}
}
