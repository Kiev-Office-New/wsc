package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"context"
	"golang.org/x/net/websocket"
	"cloud.google.com/go/pubsub"
)

type headers []string

func (h *headers) String() string {
	return strings.Join(*h, ", ")
}

func (h *headers) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
	var (
		target  = flag.String("u", "", "The URL to connect to")
		origin  = flag.String("o", "", "The origin to use in the WS request")
		h       headers
		origURL *url.URL
	)
	flag.Var(&h, "H", `Headers to use in the WS request, can be used to multiple times to specify multiple headers.`+
		` Example: -H "Sample-Header-1: foo" -H "Sample-Header-2: bar"`)
	flag.Parse()

	if *target == "" {
		fmt.Fprintf(os.Stderr, "missing url\n")
		os.Exit(1)
	}

	if *origin != "" {
		var err error
		origURL, err = url.Parse(*origin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse origin URL: %s", err.Error())
			os.Exit(1)
		}
	}
	ws := connect(*target, makeHeader(h), origURL)
	trapCtrlC(ws)
	go write(ws)
	read(ws)
}

func makeHeader(h headers) http.Header {
	httpH := make(http.Header)
	for _, hv := range h {
		splits := strings.SplitN(hv, ":", 2)
		httpH.Add(strings.TrimSpace(splits[0]), strings.TrimSpace(splits[1]))
	}
	return httpH
}

func connect(addr string, h http.Header, origin *url.URL) *websocket.Conn {
	log.Printf("connecting to %s...", addr)
	conf, err := websocket.NewConfig(addr, addr)
	if err != nil {
		log.Fatal(err)
	}
	conf.Header = h
	conf.Origin = origin
	ws, err := websocket.DialConfig(conf)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("ready, exit with CTRL+C.")
	return ws
}

// Graceful shutdown
func trapCtrlC(c *websocket.Conn) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			fmt.Println("\nexiting")
			c.Close()
			os.Exit(0)
		}
	}()
}

// Send STDIN lines to websocket server.
func write(ws *websocket.Conn) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		t := scanner.Text()
		ws.Write([]byte(t))
		fmt.Printf(">> %s\n", t)
	}
}

// Read from websocket and print messages to STDOUT
func read(ws *websocket.Conn) {
	msg := make([]byte, 16384)
	for {
		n, err := ws.Read(msg)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("<< %s\n", msg[:n])
		publish("crawler-249816","bwin-events",msg[:n])
	}
}

func publish(projectID string, topicID string, msg []byte) error {
        // projectID := "my-project-id"
        // topicID := "my-topic"
        // msg := "Hello World"
        ctx := context.Background()
        client, err := pubsub.NewClient(ctx, projectID)
        if err != nil {
		fmt.Printf("pubsub.NewClient: %v", err)
                return fmt.Errorf("pubsub.NewClient: %v", err)
        }

        t := client.Topic(topicID)
        result := t.Publish(ctx, &pubsub.Message{
                Data: msg,
        })
        // Block until the result is returned and a server-generated
        // ID is returned for the published message.
        id, err := result.Get(ctx)
        if err != nil {
		fmt.Printf("Get: %v", err)
                return fmt.Errorf("Get: %v", err)
        }
	fmt.Printf("^^ Published a message; msg ID: %v\n", id)
        return nil
}
