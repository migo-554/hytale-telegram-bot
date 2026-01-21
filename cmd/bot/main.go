package main

import (
	"fmt"
	"hytale-bot/internal/config"
	"hytale-bot/internal/state"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/telebot.v3"
)

func main() {
	// 1. Инициализация
	_ = godotenv.Load()
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN не найден")
	}

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}

	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	// --- 2. КНОПКИ ---
	menu := &telebot.ReplyMarkup{}
	btnBuy := menu.Data("Купить Hytale", "start_buy")
	btnPay := menu.Data("Перейти к оплате", "go_to_pay")
	btnBack := menu.Data("« В главное меню", "go_to_main")
	btnCancel := menu.Data("❌ Отменить заявку", "cancel_order")

	// --- 3. ОБРАБОТЧИКИ ---

	b.Handle("/start", func(c telebot.Context) error {
		menu.Inline(menu.Row(btnBuy))
		return c.Send(cfg.Messages.Greeting, menu)
	})

	b.Handle(telebot.OnCallback, func(c telebot.Context) error {
		session := state.GetSession(c.Sender().ID)
		data := c.Callback().Data

		if len(data) > 0 && data[0] == '\f' {
			data = data[1:]
		}

		switch {
		// --- ЛОГИКА КЛИЕНТА ---

		case data == "go_to_main":
			session.State = state.StateNone
			menu.Inline(menu.Row(btnBuy))
			if c.Message().Photo != nil {
				c.Delete() // Удаляем старое фото
				return c.Send(cfg.Messages.Greeting, menu)
			}
			return c.Edit(cfg.Messages.Greeting, menu)

		case data == "cancel_order":
			// Если отменили - пишем админу, что отмена, но данные оставляем
			if session.AdminMsgID != 0 {
				// Получаем доступ к сообщению админа (мы не можем прочитать текст удаленно без сохранения,
				// но мы можем просто добавить пометку "ОТМЕНА" к тому что есть, если бы знали текст.
				// Но так как мы не храним текст, тут проще полностью заменить статус)
				// ВАЖНО: При отмене пользователем текст админа обычно заменяется на "Отменено".
				b.EditCaption(&telebot.Message{
					ID:   session.AdminMsgID,
					Chat: &telebot.Chat{ID: cfg.AdminID},
				}, fmt.Sprintf("⚠️ Пользователь @%s ОТМЕНИЛ заявку.\n(Процесс остановлен)", c.Sender().Username))
			}
			session.State = state.StateNone
			menu.Inline(menu.Row(btnBuy))
			return c.Edit("Заявка отменена. Вы в главном меню.", menu)

		case data == "start_buy":
			infoText := fmt.Sprintf("%s\n\nЦена: %s", cfg.Messages.Info, cfg.PriceText)
			menu.Inline(menu.Row(btnPay), menu.Row(btnBack))
			return c.Edit(infoText, menu)

		case data == "go_to_pay":
			session.State = state.StateAwaitingScreenshot
			menu.Inline(menu.Row(btnCancel))
			return c.Edit(cfg.Messages.Instructions, menu)

		// --- ЛОГИКА АДМИНА (Исправленная) ---

		case len(data) > 7 && data[:7] == "accept_":
			var targetID int64
			fmt.Sscanf(data[7:], "%d", &targetID)

			// 1. Клиенту отправляем просто текст (БЕЗ КНОПОК), чтобы он ждал
			b.Send(telebot.ChatID(targetID), "✅ Ваша оплата подтверждена! Мы начали выполнение заказа. Ожидайте.")

			// 2. У админа берем СТАРЫЙ текст и добавляем статус
			oldCaption := c.Message().Caption
			newCaption := oldCaption + "\n\n✅ ЗАЯВКА ПРИНЯТА В РАБОТУ"

			// 3. Редактируем подпись админа: старый текст + статус + пустая клавиатура (удаляем кнопки)
			return c.EditCaption(newCaption, &telebot.ReplyMarkup{})

		case len(data) > 7 && data[:7] == "reject_":
			var targetID int64
			fmt.Sscanf(data[7:], "%d", &targetID)

			// 1. Клиенту отправляем текст отказа (БЕЗ КНОПОК "В меню", чтобы он прочитал причину)
			b.Send(telebot.ChatID(targetID), "❌ Оплата не найдена или данные неверны. Пожалуйста, проверьте и оформите заново.")

			// 2. У админа берем СТАРЫЙ текст и добавляем статус
			oldCaption := c.Message().Caption
			newCaption := oldCaption + "\n\n❌ ЗАЯВКА ОТКЛОНЕНА"

			// 3. Редактируем подпись
			return c.EditCaption(newCaption, &telebot.ReplyMarkup{})
		}
		return nil
	})

	b.Handle(telebot.OnPhoto, func(c telebot.Context) error {
		session := state.GetSession(c.Sender().ID)
		if session.State == state.StateAwaitingScreenshot {
			session.PhotoID = c.Message().Photo.FileID
			session.State = state.StateAwaitingEmail
			return c.Send(cfg.Messages.GetEmail)
		}
		return nil
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		session := state.GetSession(c.Sender().ID)

		if session.State == state.StateAwaitingScreenshot {
			return c.Send("Пожалуйста, пришлите скриншот.")
		}

		switch session.State {
		case state.StateAwaitingEmail:
			session.Email = c.Text()
			session.State = state.StateAwaitingPass
			return c.Send(cfg.Messages.GetPassword)

		case state.StateAwaitingPass:
			session.Password = c.Text()
			session.State = state.StateNone

			// Меню отмены для клиента
			menu.Inline(menu.Row(btnCancel))
			c.Send(cfg.Messages.Success, menu)

			// Формируем отчет
			adminReport := fmt.Sprintf("🚀 Заявка!\n👤 @%s (ID: %d)\n📧 `%s`\n🔑 `%s` ",
				c.Sender().Username, c.Sender().ID, session.Email, session.Password)

			// Кнопки для админа
			adminMarkup := &telebot.ReplyMarkup{}
			btnA := adminMarkup.Data("✅ Принять", fmt.Sprintf("accept_%d", c.Sender().ID))
			btnR := adminMarkup.Data("❌ Отклонить", fmt.Sprintf("reject_%d", c.Sender().ID))
			adminMarkup.Inline(adminMarkup.Row(btnA, btnR))

			photo := &telebot.Photo{File: telebot.File{FileID: session.PhotoID}, Caption: adminReport}
			msg, _ := b.Send(telebot.ChatID(cfg.AdminID), photo, telebot.ModeMarkdown, adminMarkup)

			session.AdminMsgID = msg.ID
			return nil
		}
		return nil
	})

	log.Println("Бот запущен...")
	b.Start()
}
