package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
)

const wsURL = "wss://fstream.binance.com/stream?streams=btcusdt@depth"

type BinanceDepthResult struct {
	Asks [][]string `json:"a"`
	Bids [][]string `json:"b"`
}

type BinanceDepthResponse struct {
	Stream string             `json:"stream"`
	Data   BinanceDepthResult `json:"data"`
}

func main() {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal(err)
	}

	var result BinanceDepthResponse
	for {
		if err := conn.ReadJSON(&result); err != nil {
			log.Fatal(err)
		}

		fmt.Println(result)
	}
}
