package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"nhooyr.io/websocket"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	outgoingMessagesCh := make(chan []byte, 16)
	incomingMessagesCh := make(chan []byte, 16)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	c, err := connect()
	if err != nil {
		fmt.Println("Failed to connect to chat server")
		return
	}

	g.Go(func() error {
		return listener(ctx, c, incomingMessagesCh)
	})

	g.Go(func() error {
		return cliWriter(ctx, incomingMessagesCh)
	})

	g.Go(func() error {
		return cliReader(ctx, c, outgoingMessagesCh)
	})

	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		c.Close(websocket.StatusNormalClosure, "Closing connection")
		cancel()
	}()

	<-ctx.Done()
}

func listener(ctx context.Context, c *websocket.Conn, messages chan []byte) error {
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			fmt.Println(err)
			c.Close(websocket.StatusInternalError, "Server connection lost")
			return err
		}

		messages <- b
	}
}

func cliWriter(ctx context.Context, messages chan []byte) error {
	for {
		select {
		case mes := <-messages:
			fmt.Println(string(mes))
		case <-ctx.Done():
			return nil
		}
	}
}

func cliReader(ctx context.Context, wsConn *websocket.Conn, messages chan []byte) error {
	for {
		userinput, err := readUserInput()

		if err != nil {
			return err
		}

		writeTimeout(ctx, time.Second*5, wsConn, userinput)
	}
}

func readUserInput() ([]byte, error) {
	reader := bufio.NewReader(os.Stdin)
	userInput, err := reader.ReadBytes('\n')

	if err != nil {
		return userInput, err
	}

	return userInput, nil
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Write(ctx, websocket.MessageText, msg)
}

func connect() (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	c, _, err := websocket.Dial(ctx, "ws://localhost:8080/subscribe", nil)
	if err != nil {
		return c, err
	}

	return c, err
}
