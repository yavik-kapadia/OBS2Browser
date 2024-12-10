package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"golang.org/x/net/websocket"
)

//go:embed index.html
var content embed.FS

func htmlHandler(w http.ResponseWriter, req *http.Request) {
	http.ServeFileFS(w, req, content, "index.html")
}

var (
	answerChan       chan string
	currentWebsocket atomic.Value
	nilWs            *websocket.Conn
)

func whipHandler(w http.ResponseWriter, req *http.Request) {
	log.Println("Accepted WHIP Request")
	offer, err := io.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(err)
	}

	ws, ok := currentWebsocket.Load().(*websocket.Conn)
	if !ok || ws == nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic("WHIP Offer received with no WebSocket client connected")
	}

	if err := websocket.Message.Send(ws, string(offer)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(err)
	}

	w.Header().Set("content-type", "application/sdp")
	w.Header().Set("location", "/")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, <-answerChan)
}

func websocketHandler(ws *websocket.Conn) {
	log.Println("WebSocket connected")
	currentWebsocket.Store(ws)

	for {
		var answer string
		if err := websocket.Message.Receive(ws, &answer); err != nil {
			break
		}

		answerChan <- answer
	}

	log.Println("WebSocket disconnected")
	currentWebsocket.Store(nilWs)
}

func main() {
	currentWebsocket.Store(nilWs)
	answerChan = make(chan string)

	fs := http.FileServer(http.Dir("."))
	http.Handle("/styles.css", fs)

	http.HandleFunc("/", htmlHandler)
	http.HandleFunc("/whip", whipHandler)
	http.Handle("/websocket", websocket.Handler(func(ws *websocket.Conn) {
		websocketHandler(ws)
	}))

	httpAddr := os.Getenv("HTTP_ADDR")
	if len(httpAddr) == 0 {
		httpAddr = ":80"
	}

	log.Printf("Starting HTTP Server on %s", httpAddr)
	panic(http.ListenAndServe(httpAddr, nil))
}
