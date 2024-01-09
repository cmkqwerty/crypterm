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

const wsURL = "wss://fstream.binance.com/stream?streams=btcusdt@markPrice@1s/btcusdt@depth"

var (
	WIDTH         = 0
	HEIGHT        = 0
	curMarkPrice  = 0.0
	prevMarkPrice = 0.0
	ARROW_UP      = '▲'
	ARROW_DOWN    = '▼'
)

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

func (ob *OrderBook) handleDepthResponse(asks, bids []any) {
	for _, v := range asks {
		ask := v.([]any)
		price, _ := strconv.ParseFloat(ask[0].(string), 64)
		volume, _ := strconv.ParseFloat(ask[1].(string), 64)
		if entry, ok := ob.Asks.Get(getAskByPrice(price)); ok {
			if volume == 0 {
				ob.Asks.Delete(entry)
			} else {
				entry.Volume = volume
			}
			continue
		}

		entry := &OrderBookEntry{
			Price:  price,
			Volume: volume,
		}

		ob.Asks.Insert(entry)
	}

	for _, v := range bids {
		bid := v.([]any)
		price, _ := strconv.ParseFloat(bid[0].(string), 64)
		volume, _ := strconv.ParseFloat(bid[1].(string), 64)
		if entry, ok := ob.Bids.Get(getBidByPrice(price)); ok {
			if volume == 0 {
				ob.Bids.Delete(entry)
			} else {
				entry.Volume = volume
			}
			continue
		}

		entry := &OrderBookEntry{
			Price:  price,
			Volume: volume,
		}

		ob.Bids.Insert(entry)
	}
}

func (ob *OrderBook) getBids() []*OrderBookEntry {
	var (
		depth = 10
		bids  = make([]*OrderBookEntry, depth)
		it    = ob.Bids.Iterator(nil, nil)
		i     = 0
	)
	for it.Next() {
		if i == depth {
			break
		}

		bids[i] = it.Item()
		i++
	}

	it.Release()

	return bids
}

func (ob *OrderBook) getAsks() []*OrderBookEntry {
	var (
		depth = 10
		asks  = make([]*OrderBookEntry, depth)
		it    = ob.Asks.Iterator(nil, nil)
		i     = 0
	)
	for it.Next() {
		if i == depth {
			break
		}

		asks[i] = it.Item()
		i++
	}

	it.Release()

	return asks
}

func (ob *OrderBook) render(x, y int) {
	// render the orderBook left-frame border
	for i := 0; i < HEIGHT; i++ {
		termbox.SetCell(WIDTH-22, i, '|', termbox.ColorWhite, termbox.ColorDefault)
	}

	for i, ask := range ob.getAsks() {
		if ask == nil {
			continue
		}

		price := fmt.Sprintf("%.2f", ask.Price)
		volume := fmt.Sprintf("%.2f", ask.Volume)
		renderText(x, y+i, price, termbox.ColorRed)
		renderText(x+10, y+i, volume, termbox.ColorCyan)
	}

	for i, bid := range ob.getBids() {
		if bid == nil {
			continue
		}

		price := fmt.Sprintf("%.2f", bid.Price)
		volume := fmt.Sprintf("%.2f", bid.Volume)
		renderText(x, 10+i, price, termbox.ColorGreen)
		renderText(x+10, 10+i, volume, termbox.ColorCyan)
	}
}

type BinanceTradeResult struct {
	Data struct {
		Price string `json:"p"`
	} `json:"data"`
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
	WIDTH, HEIGHT = termbox.Size()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal(err)
	}

	var (
		ob     = NewOrderBook()
		result map[string]any
	)
	go func() {
		for {
			if err := conn.ReadJSON(&result); err != nil {
				log.Fatal(err)
			}

			stream := result["stream"]

			if stream == "btcusdt@depth" {
				data := result["data"].(map[string]any)
				asks := data["a"].([]any)
				bids := data["b"].([]any)
				ob.handleDepthResponse(asks, bids)
			}
			if stream == "btcusdt@markPrice@1s" {
				prevMarkPrice = curMarkPrice
				data := result["data"].(map[string]any)
				priceStr := data["p"].(string)
				curMarkPrice, _ = strconv.ParseFloat(priceStr, 64)
			}
		}
	}()

	isRunning := true
	eventch := make(chan termbox.Event, 1)
	go func() {
		for {
			eventch <- termbox.PollEvent()
		}
	}()

	for isRunning {
		select {
		case event := <-eventch:
			switch event.Key {
			case termbox.KeyEsc:
				isRunning = false
			}
		default:
		}

		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		render()
		ob.render(WIDTH-18, 2)
		time.Sleep(time.Millisecond * 16)
		termbox.Flush()
	}
}

func renderMarketPrice() {
	color := termbox.ColorRed
	arrow := ARROW_DOWN
	if curMarkPrice > prevMarkPrice {
		color = termbox.ColorGreen
		arrow = ARROW_UP
	}

	renderText(2, 1, fmt.Sprintf("%.2f", curMarkPrice), color)
	renderText(9, 1, string(arrow), color)
}

func render() {
	// render the panel border
	for i := 0; i < WIDTH; i++ {
		termbox.SetCell(i, 0, '-', termbox.ColorWhite, termbox.ColorDefault)
		termbox.SetCell(i, HEIGHT-1, '-', termbox.ColorWhite, termbox.ColorDefault)
	}

	for i := 0; i < HEIGHT; i++ {
		termbox.SetCell(0, i, '|', termbox.ColorWhite, termbox.ColorDefault)
		termbox.SetCell(WIDTH-1, i, '|', termbox.ColorWhite, termbox.ColorDefault)
	}

	for i := 0; i < WIDTH; i++ {
		termbox.SetCell(i, 2, '-', termbox.ColorWhite, termbox.ColorDefault)
	}

	// render the misc border
	renderMarketPrice()
}

func renderLine(s, e int, y int, color termbox.Attribute) {
	for i := s; i < e; i++ {
		termbox.SetCell(i, y, '-', color, termbox.ColorDefault)
	}
}

func renderText(x int, y int, msg string, color termbox.Attribute) {
	for _, c := range msg {
		termbox.SetCell(x, y, c, color, termbox.ColorDefault)

		w := runewidth.RuneWidth(c)
		x += w
	}
}
