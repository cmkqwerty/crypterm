package main

import (
	"fmt"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/gorilla/websocket"
	"log"
	"sort"
	"strconv"
	"time"
)

// TODO: refactor
// TODO: fix ws connection bug
// TODO: add indicators to the ui

const wsURL = "wss://fstream.binance.com/stream?streams=btcusdt@markPrice@1s/btcusdt@depth"

var (
	WIDTH         = 0
	HEIGHT        = 0
	curMarkPrice  = 0.0
	prevMarkPrice = 0.0
	fundingRate   = "n/a"
	ARROW_UP      = "▲"
	ARROW_DOWN    = "▼"
)

type OrderBookEntry struct {
	Price  float64
	Volume float64
}

type byBestAsk []OrderBookEntry

func (a byBestAsk) Len() int {
	return len(a)
}

func (a byBestAsk) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a byBestAsk) Less(i, j int) bool {
	return a[i].Price < a[j].Price
}

type byBestBid []OrderBookEntry

func (b byBestBid) Len() int {
	return len(b)
}

func (b byBestBid) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byBestBid) Less(i, j int) bool {
	return b[i].Price > b[j].Price
}

type OrderBook struct {
	Asks map[float64]float64
	Bids map[float64]float64
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Asks: make(map[float64]float64),
		Bids: make(map[float64]float64),
	}
}

func (ob *OrderBook) addAsk(price, volume float64) {
	if volume == 0 {
		delete(ob.Asks, price)
		return
	}

	ob.Asks[price] = volume
}

func (ob *OrderBook) addBid(price, volume float64) {
	if _, ok := ob.Bids[price]; ok {
		if volume == 0 {
			delete(ob.Bids, price)
			return
		}
	}

	ob.Bids[price] = volume
}

func (ob *OrderBook) handleDepthResponse(asks, bids []any) {
	for _, v := range asks {
		ask := v.([]any)
		price, _ := strconv.ParseFloat(ask[0].(string), 64)
		volume, _ := strconv.ParseFloat(ask[1].(string), 64)

		ob.addAsk(price, volume)
	}

	for _, v := range bids {
		bid := v.([]any)
		price, _ := strconv.ParseFloat(bid[0].(string), 64)
		volume, _ := strconv.ParseFloat(bid[1].(string), 64)

		ob.addBid(price, volume)
	}
}

func (ob *OrderBook) getAsks() []OrderBookEntry {
	var (
		depth   = 10
		entries = make(byBestAsk, len(ob.Asks))
		i       = 0
	)

	for price, volume := range ob.Asks {
		entries[i] = OrderBookEntry{
			Price:  price,
			Volume: volume,
		}

		i++
	}

	sort.Sort(entries)
	if len(entries) >= depth {
		return entries[:depth]
	}

	return entries
}

func (ob *OrderBook) getBids() []OrderBookEntry {
	var (
		depth   = 10
		entries = make(byBestBid, len(ob.Bids))
		i       = 0
	)

	for price, volume := range ob.Bids {
		if volume == 0 {
			continue
		}
		entries[i] = OrderBookEntry{
			Price:  price,
			Volume: volume,
		}

		i++
	}

	sort.Sort(entries)
	if len(entries) >= depth {
		return entries[:depth]
	}

	return entries
}

func (ob *OrderBook) render(x, y int) {
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
	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

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
				fundingRate = data["r"].(string)
				curMarkPrice, _ = strconv.ParseFloat(priceStr, 64)
			}
		}
	}()

	isRunning := true
	margin := 2
	pHeight := 3

	pTicker := widgets.NewParagraph()
	pTicker.Title = "BinanceFtr"
	pTicker.Text = "[BTCUSDT](fg:cyan)"
	pTicker.SetRect(0, 0, 14, pHeight)

	pPrice := widgets.NewParagraph()
	pPrice.Title = "Market Price"
	pPriceOffset := 28 + margin*2
	pPrice.SetRect(14+margin, 0, pPriceOffset, pHeight)

	pFund := widgets.NewParagraph()
	pFund.Title = "Funding Rate"
	pFund.SetRect(pPriceOffset+margin, 0, pPriceOffset+margin+16, pHeight)

	oBook := widgets.NewTable()
	out := make([][]string, 20)
	for i := 0; i < 20; i++ {
		out[i] = []string{"n/a", "n/a"}
	}
	oBook.TextStyle = ui.NewStyle(ui.ColorWhite)
	oBook.SetRect(0, pHeight+margin, 30, 22+pHeight+margin)
	oBook.PaddingBottom = 0
	oBook.PaddingTop = 0
	oBook.RowSeparator = false
	oBook.TextAlignment = ui.AlignCenter

	for isRunning {
		var (
			asks = ob.getAsks()
			bids = ob.getBids()
		)

		if len(asks) >= 10 {
			for i := 0; i < 10; i++ {
				out[i] = []string{
					fmt.Sprintf("[%.2f](fg:red)", asks[i].Price),
					fmt.Sprintf("[%.2f](fg:cyan)", asks[i].Volume),
				}
			}
		}

		if len(bids) >= 10 {
			for i := 0; i < 10; i++ {
				out[i+10] = []string{
					fmt.Sprintf("[%.2f](fg:green)", bids[i].Price),
					fmt.Sprintf("[%.2f](fg:cyan)", bids[i].Volume),
				}
			}
		}

		oBook.Rows = out

		pPrice.Text = getMarketPrice()
		pFund.Text = fmt.Sprintf("[%s%%](fg:yellow)", fundingRate)
		ui.Render(pTicker, pPrice, pFund, oBook)
		time.Sleep(time.Millisecond * 20)
	}
}

func getMarketPrice() string {
	price := fmt.Sprintf("[%s %.2f](fg:red)", ARROW_DOWN, curMarkPrice)
	if curMarkPrice > prevMarkPrice {
		price = fmt.Sprintf("[%s %.2f](fg:green)", ARROW_UP, curMarkPrice)
	}

	return price
}
