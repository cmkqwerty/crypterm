package main

import (
	"fmt"
	"github.com/VictorLowther/btree"
	"github.com/gorilla/websocket"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"log"
	"strconv"
	"time"
)

const wsURL = "wss://fstream.binance.com/stream?streams=btcusdt@depth"

func byBestBid(a, b *OrderBookEntry) bool {
	return a.Price >= b.Price
}

func byBestAsk(a, b *OrderBookEntry) bool {
	return a.Price < b.Price
}

type OrderBookEntry struct {
	Price  float64
	Volume float64
}

type OrderBook struct {
	Asks *btree.Tree[*OrderBookEntry]
	Bids *btree.Tree[*OrderBookEntry]
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Asks: btree.New(byBestAsk),
		Bids: btree.New(byBestBid),
	}
}

func getAskByPrice(price float64) btree.CompareAgainst[*OrderBookEntry] {
	return func(e *OrderBookEntry) int {
		switch {
		case e.Price < price:
			return -1
		case e.Price > price:
			return 1
		default:
			return 0
		}
	}
}

func getBidByPrice(price float64) btree.CompareAgainst[*OrderBookEntry] {
	return func(e *OrderBookEntry) int {
		switch {
		case e.Price > price:
			return -1
		case e.Price < price:
			return 1
		default:
			return 0
		}
	}
}

func (ob *OrderBook) handleDepthResponse(result BinanceDepthResult) {
	go func() {
		for _, ask := range result.Asks {
			price, _ := strconv.ParseFloat(ask[0], 64)
			volume, _ := strconv.ParseFloat(ask[1], 64)
			if volume == 0 {
				if entry, ok := ob.Asks.Get(getAskByPrice(price)); ok {
					// fmt.Printf("---Deleting level %.2f---\n", price)
					ob.Asks.Delete(entry)
				}

				return
			}

			entry := &OrderBookEntry{
				Price:  price,
				Volume: volume,
			}

			ob.Asks.Insert(entry)
		}
	}()

	for _, bid := range result.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		volume, _ := strconv.ParseFloat(bid[1], 64)
		if volume == 0 {
			if entry, ok := ob.Bids.Get(getBidByPrice(price)); ok {
				// fmt.Printf("---Deleting level %.2f---\n", price)
				ob.Bids.Delete(entry)
			}

			return
		}

		entry := &OrderBookEntry{
			Price:  price,
			Volume: volume,
		}

		ob.Bids.Insert(entry)
	}
}

func (ob *OrderBook) render(x, y int) {
	it := ob.Asks.Iterator(nil, nil)
	i := 0
	for it.Next() {
		item := it.Item()
		priceStr := fmt.Sprintf("%.2f", item.Price)
		renderText(x, y+i, priceStr, termbox.ColorRed)

		i++
	}

	it = ob.Bids.Iterator(nil, nil)
	i = 0
	x += 10
	for it.Next() {
		item := it.Item()
		priceStr := fmt.Sprintf("%.2f", item.Price)
		renderText(x, y+i, priceStr, termbox.ColorGreen)

		i++
	}
}

type BinanceDepthResult struct {
	Asks [][]string `json:"a"`
	Bids [][]string `json:"b"`
}

type BinanceDepthResponse struct {
	Stream string             `json:"stream"`
	Data   BinanceDepthResult `json:"data"`
}

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal(err)
	}

	var (
		ob     = NewOrderBook()
		result BinanceDepthResponse
	)
	go func() {
		for {
			if err := conn.ReadJSON(&result); err != nil {
				log.Fatal(err)
			}

			ob.handleDepthResponse(result.Data)
		}
	}()

	isRunning := true
	go func() {
		time.Sleep(time.Second * 60)
		isRunning = false
	}()

	for isRunning {
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		ob.render(0, 0)
		termbox.Flush()
	}
}

func renderText(x int, y int, msg string, color termbox.Attribute) {
	for _, c := range msg {
		termbox.SetCell(x, y, c, color, termbox.ColorDefault)

		w := runewidth.RuneWidth(c)
		x += w
	}
}
