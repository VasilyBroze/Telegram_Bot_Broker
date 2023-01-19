package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type bnResp struct { //BINANCE
	Price float64 `json:"price,string"`
	Code  int64   `json:"code"`
}

/*
type MoexStock struct {
	MarketdataYields struct {
		Data []interface{} `json:"data"` //ЗДЕСЬ ИНТЕРЕСУЕТ [12] 13ый элемент интерфейса
	} `json:"marketdata_yields"`
}
*/

type MoexStock struct {
	Marketdata struct {
		Data [][]interface{} `json:"data"` //interface
	} `json:"marketdata"`
}

type yfResp struct { //YAHOO FINANCE
	QuoteSummary struct {
		Result []struct {
			Price struct {
				RegularMarketPrice struct {
					Raw float64 `json:"raw"`
					Fmt string  `json:"fmt"`
				} `json:"regularMarketPrice"`
			} `json:"price"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"quoteSummary"`
}

func main() {
	//СОЗДАНИЕ БД
	database, _ := sql.Open("sqlite3", "./gopher.db")

	statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS people (id INTEGER PRIMARY KEY, chat_id INTEGER,ticker TEXT, amount FLOAT)")
	statement.Exec()
	//БД СОЗДАНА

	bot, err := tgbotapi.NewBotAPI("5405522760:AAFqA15HEI8tn--bRzzEd-TQiobMIv2AAEo")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	//log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // If we got a message
			continue
		}
		command := strings.Split(strings.ToUpper(update.Message.Text), " ")

		switch command[0] {

		case "ADD": //ДОБАВИТЬ ТИКЕР
			if len(command) != 3 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверная команда"))
			} else {
				amountInput, err := strconv.ParseFloat(command[2], 64)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверное количество"))
				} else {

					//СЧИТЫВАНИЕ ИЗ БАЗЫ
					data1, _ := database.Query("SELECT chat_id, ticker, amount FROM people WHERE ticker = ? AND chat_id = ?", command[1], update.Message.Chat.ID)
					var chatId int
					var ticker string
					var amount float64

					data1.Next()
					data1.Scan(&chatId, &ticker, &amount)
					data1.Close()
					if ticker == "" {
						//ЕСЛИ СТРОКИ НЕТ - ДОБАВЛЕНИЕ СТРОКИ
						statement, _ = database.Prepare("INSERT INTO people (chat_id, ticker, amount) VALUES (?, ?, ?)")
						statement.Exec(update.Message.Chat.ID, command[1], command[2])
						//ВЫВОД В ЧАТ
						balanceText := fmt.Sprintf("Тикер добавлен. Баланс %v: %v", command[1], command[2])
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, balanceText))

					} else {
						//ЕСЛИ СТРОКА ЕСТЬ - ОБНОВЛЯЕМ ЗНАЧЕНИЕ
						_, err := database.Exec("UPDATE people SET amount=amount + ? WHERE chat_id = ? AND ticker = ?", amountInput, update.Message.Chat.ID, command[1])
						if err != nil {
							fmt.Println(err)
						}
						//ВЫВОД В ЧАТ
						balanceText := fmt.Sprintf("Баланс %v: %v", command[1], amount+amountInput)
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, balanceText))
					}
				}
			}

		case "SUB": //ОТНЯТЬ ТИКЕР
			if len(command) != 3 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверная команда"))
			} else {
				amountInput, err := strconv.ParseFloat(command[2], 64)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверное количество"))
				} else {

					//СЧИТЫВАНИЕ ИЗ БАЗЫ
					data1, _ := database.Query("SELECT chat_id,ticker, amount FROM people WHERE ticker = ? AND chat_id = ?", command[1], update.Message.Chat.ID)
					var chatId int
					var ticker string
					var amount float64

					data1.Next()
					data1.Scan(&chatId, &ticker, &amount)
					data1.Close()
					if ticker == "" {
						//ЕСЛИ СТРОКИ НЕТ - ДОБАВЛЕНИЕ СТРОКИ
						statement, _ = database.Prepare("INSERT INTO people (chat_id, ticker, amount) VALUES (?, ?, ?)")
						statement.Exec(update.Message.Chat.ID, command[1], command[2])
						//ВЫВОД В ЧАТ
						balanceText := fmt.Sprintf("Тикер добавлен. Баланс %v: %v", command[1], command[2])
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, balanceText))

					} else {
						//ЕСЛИ СТРОКА ЕСТЬ - ОБНОВЛЯЕМ ЗНАЧЕНИЕ
						result, err := database.Exec("UPDATE people SET amount=amount - ? WHERE chat_id = ? AND ticker = ?", amountInput, update.Message.Chat.ID, command[1])
						if err != nil {
							fmt.Println(err)
							fmt.Println(result)
						}
						//ВЫВОД В ЧАТ
						balanceText := fmt.Sprintf("Баланс %v: %v", command[1], amount-amountInput)
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, balanceText))
					}
				}
			}

		case "DEL":
			if len(command) != 2 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверная команда"))
			} else {
				//ЕСЛИ СТРОКА ЕСТЬ - УДАЛЯЕМ
				_, err := database.Exec("DELETE FROM people WHERE chat_id = ? AND ticker = ?", update.Message.Chat.ID, command[1])
				if err != nil {
					fmt.Println("Ошибка удаления")
				}
				//ВЫВОД В ЧАТ
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Тикер удалён"))
			}

		case "SHOW":
			msg := ""
			var sum float64
			rows, _ := database.Query("SELECT ticker, amount FROM people WHERE chat_id = ?", update.Message.Chat.ID)
			var ticker string
			var amount float64
			usd, _ := getPriceUSD()
			//ПРОВЕРЯЕМ ВСЕ ДАННЫЕ В ТАБЛИЦЕ ПО ЧАТ ID
			for rows.Next() {
				var rub bool
				rows.Scan(&ticker, &amount)
				price, _ := getPrice(ticker) //BINANCE
				if price == 0 {
					price, _ = getPrice2(ticker) //MOEX
					if price == 0 {
						price, _ = getPrice3(ticker) //YAHOO
						sum += amount * price
					} else {
						rub = true
						sum += amount * price / usd
					}
				} else {
					sum += amount * price
				}

				sum += amount * price
				if price != 0 {
					if rub == false {
						msg += fmt.Sprintf("%s: %v [%.2f USD] (Цена: %.2f)\n", ticker, amount, amount*price, price)
					} else {
						msg += fmt.Sprintf("%s: %v [%.2f USD] (Цена: %.2f)\n", ticker, amount, amount*price/usd, price/usd)
					}
				} else {
					msg += fmt.Sprintf("%s: %v [%.2f USD (Тикер не найден)]\n", ticker, amount, amount*price)
				}
			}
			msg += fmt.Sprintf("Общий балланс: %.2f USD\n", sum)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))
			rows.Close()

		case "SHOWRUB", "/SHOWRUB":
			msg := ""
			var sum float64
			usd, _ := getPriceUSD()
			rows, _ := database.Query("SELECT ticker, amount FROM people WHERE chat_id = ?", update.Message.Chat.ID)
			var ticker string
			var amount float64

			//ПРОВЕРЯЕМ ВСЕ ДАННЫЕ В ТАБЛИЦЕ ПО ЧАТ ID
			for rows.Next() {
				var rub bool
				rows.Scan(&ticker, &amount)
				price, _ := getPrice(ticker) //BINANCE
				fmt.Println("Binance TEST Ticker")
				fmt.Println(price)
				if price == 0 {
					price, _ = getPrice2(ticker) //MOEX
					fmt.Println("MOEX TEST Ticker")
					fmt.Println(price)
					if price == 0 {
						price, _ = getPrice3(ticker) //YAHOO
						fmt.Println("Yahoo TEST Ticker")
						fmt.Println(price)
						sum += amount * price * usd
					} else {
						rub = true
						sum += amount * price
					}
				} else {
					sum += amount * price * usd
				}

				if price != 0 {
					if rub == false {
						msg += fmt.Sprintf("%s: %v [%.2f RUB] (Цена: %.2f)\n", ticker, amount, amount*price*usd, price*usd)
					} else {
						msg += fmt.Sprintf("%s: %v [%.2f RUB] (Цена: %.2f)\n", ticker, amount, amount*price, price)
					}
				} else {
					msg += fmt.Sprintf("%s: %v [%.2f RUB (Тикер не найден)]\n", ticker, amount, amount*price)
				}
			}
			msg += fmt.Sprintf("Общий балланс: %.2f RUB\n", sum)
			//ВЫВОД В ЧАТ
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

		case "/DESCRIPTION":
			msg := fmt.Sprintf("Описание комманд:\nADD (тикер) (количество) - добавить\nSUB (тикер) (количество) - отнять\nDEL (тикер) - удалить\nSHOW - баланс (USD)\nSHOWRUB - баланс (RUB)")
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

		case "USD", "/USD":
			usd, _ := getPriceUSD()
			msg := fmt.Sprintf("Курс доллара: %.2f", usd)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

		default:
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Команда не найдена"))
		}
	}
}

func getPrice(symbol string) (price float64, err error) {
	fmt.Println("Зашли в апи бинанс")
	resp, err := http.Get(fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%sUSDT", symbol))
	if err != nil {
		return
	}

	defer resp.Body.Close()

	var jsonResp bnResp

	err = json.NewDecoder(resp.Body).Decode(&jsonResp)
	if err != nil {
		return
	}
	if jsonResp.Code != 0 {
		err = errors.New("Неверный символ")
	}

	price = jsonResp.Price
	fmt.Println("Вышли из апи бинанс")
	return
}

func getPrice2(symbol string) (price2 float64, err error) { //РУБЛЁВЫЕ АКЦИИ
	fmt.Println("Зашли в апи моекс")
	resp, err := http.Get(fmt.Sprintf("https://iss.moex.com/iss/engines/stock/markets/shares/securities.json?securities=%v", symbol))

	if err != nil {
		fmt.Println("Ошибка моекс")
		fmt.Println(err)
		return
	}
	fmt.Println("Зашли в апи яху бакс")
	resp2, err := http.Get(fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/USDRUB.ME?modules=price"))
	if err != nil {
		fmt.Println("Ошибка яху бакс")
		return
	}
	defer resp.Body.Close()
	defer resp2.Body.Close()

	var jsonResp MoexStock
	//var jsonRespUSD yfResp

	body, err := ioutil.ReadAll(resp.Body)

	if err := json.Unmarshal(body, &jsonResp); err != nil {
		panic(err)
	}

	if jsonResp.Marketdata.Data != nil {

		for i := 0; i < len(jsonResp.Marketdata.Data); i++ {

			if jsonResp.Marketdata.Data[i][1] == "TQBR" {

				actualprice := jsonResp.Marketdata.Data[i][12]
				switch s := actualprice.(type) {
				case float64:
					price2 = s
					return
				case float32:
					price2 = float64(s)
					return
				case int64:
					price2 = float64(s)
					return
				}
				return
			}
		}

		return
	}

	/*	body2, err := ioutil.ReadAll(resp2.Body)

		if err := json.Unmarshal(body2, &jsonRespUSD); err != nil {
			panic(err)
		}

		//price2 = (jsonResp.QuoteSummary.Result[0].Price.RegularMarketPrice.Raw) / (jsonRespUSD.QuoteSummary.Result[0].Price.RegularMarketPrice.Raw)
	*/
	return
}

func getPrice3(symbol string) (price3 float64, err error) { //АМЕРИКАНСКИЕ АКЦИИ
	fmt.Println("Зашли в апи яху")
	resp, _ := http.Get(fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=price", symbol))

	if err != nil {
		return
	}

	defer resp.Body.Close()

	var jsonResp yfResp

	body, err := ioutil.ReadAll(resp.Body)

	if err := json.Unmarshal(body, &jsonResp); err != nil {
		panic(err)
	}

	if jsonResp.QuoteSummary.Error != nil {
		return
	}

	price3 = (jsonResp.QuoteSummary.Result[0].Price.RegularMarketPrice.Raw)
	fmt.Println("Вышли из апи яху")
	return
}

// ФУНКЦИЯ ПОЛУЧЕНИЯ КУРСА ДОЛЛАРА ЧЕРЕЗ МОСБИРЖУ
func getPriceUSD() (price4 float64, err error) {

	resp2, _ := http.Get(fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/RUB=X?modules=price"))

	defer resp2.Body.Close()

	var jsonRespUSD yfResp

	body2, err := ioutil.ReadAll(resp2.Body)

	if err := json.Unmarshal(body2, &jsonRespUSD); err != nil {
		panic(err)
	}

	if jsonRespUSD.QuoteSummary.Error != nil {
		return
	}

	price4 = (jsonRespUSD.QuoteSummary.Result[0].Price.RegularMarketPrice.Raw)

	return
}
