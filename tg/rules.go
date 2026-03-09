package tg

import (
	"fmt"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	Rules = `Ла-ла-ла-ла-ла-ла-ла-ла-ла-ла
Ла-ла-ла-ла-ла-ла-ла-ла-ла-ла

Развяжите руки, суки!
Всё было бы оки-доки чики-буки
Да жаль у меня нет базуки

Как и два, но я четыре, пятью пятница
220 *бац! и будем улыбаться, молодца!
Я не люблю романсы
Танцы-шманцы-реверансы,
Джанкуй, шишки-шишки, без передышки-одышки
Модные кишки.

Ла-ла-ла-ла-ла-ла-ла-ла-ла-ла
Ла-ла-ла-ла-ла-ла-ла-ла-ла-ла

Старый, заводи давай (ща!)
Фары не гаси давай (ага!)
Парарарарарарарарарарарарам...
Едем едем едем едем, yeah!
Не дают взаймы взаимопонимание!

Громче чем Че Гевара, Че, Че Гевара
Рано умер, мир ему
Теперь никто не командир ему
Теперь не надо вешать звёзды на мундир ему
Всё равно и нам, всё равно и всем всё равно, но
Чему равно всё ни Алсу, ни Басё
Они не говорят, они вряд ли знают вход
Я точно вижу выход, как Дон Кихот
I'm hot! O my God! Uaaa!

Ла-ла-ла-ла-ла-ла-ла-ла-ла-ла
Ла-ла-ла-ла-ла-ла-ла-ла-ла-ла

УАУАУАУАУАУАУАУАУАУАУАУАУАУ....

Небо в алмазах на золотые унитазы
Поменяли не без мазы замазали made
Улыбки, золотые рыбки
Липкие руки, тугие браслеты
Зима, лето тополя балет
Та-тара-та-та-та
Пиво-слива, вино-казино? No!

365 дней на измене в году
Как в аду, пьём воду и стареем
Бреем бреем головы, а вы?
А мы? А? А кто мы?

Пят-ни-ца
I wanna hear you say
5-N-I-Z-Z-A

Пят-ни-ца
Let me hear you say
5-N-I-Z-Z-A

Ля-а-а-а.`
)

func RulesCallbackQueryHandler(app App) (th.CallbackQueryHandler, []th.Predicate) {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		defer func() {
			if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("done")); err != nil {
				fmt.Println(err)
			}
		}()
		return EditMessage(ctx, &telego.EditMessageTextParams{
			ChatID:    query.Message.GetChat().ChatID(),
			MessageID: query.Message.GetMessageID(),
			Text:      Rules,
		})
	}, []th.Predicate{th.AnyCallbackQueryWithMessage(), th.CallbackDataEqual("rules")}
}
