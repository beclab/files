package nats

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	NATS_HOST     = os.Getenv("NATS_HOST")
	NATS_PORT     = os.Getenv("NATS_PORT")
	NATS_USERNAME = os.Getenv("NATS_USERNAME")
	NATS_PASSWORD = os.Getenv("NATS_PASSWORD")
	NATS_SUBJECT  = os.Getenv("NATS_SUBJECT")

	nc       *nats.Conn
	wg       sync.WaitGroup
	subject  = NATS_SUBJECT
	shutdown = make(chan struct{}) // 关闭通道，用于通知消息接收器停止
)

func InitNATSConnection() error {
	var err error
	nc, err = nats.Connect(fmt.Sprintf("nats://%s:%s", NATS_HOST, NATS_PORT), nats.UserInfo(NATS_USERNAME, NATS_PASSWORD))
	if err != nil {
		return fmt.Errorf("error connecting to NATS: %v", err)
	}
	fmt.Println("Connected to NATS", NATS_HOST, NATS_PORT, NATS_USERNAME)
	return nil
}

func SendMessage(msg string) error {
	if nc == nil {
		return fmt.Errorf("NATS connection is not initialized")
	}
	fmt.Println("Sending message", msg)
	return nc.Publish(subject, []byte(msg))
}

func startMessageReceiver() {
	var sub *nats.Subscription
	var err error
	sub, err = nc.Subscribe(subject, func(m *nats.Msg) {
		select {
		case <-shutdown:
			fmt.Println("Received shutdown signal, unsubscribing from subject...")
			sub.Unsubscribe()
			return
		default:
			fmt.Printf("Received message: %s\n", string(m.Data))
		}
	})
	if err != nil {
		fmt.Printf("Error subscribing to subject: %v\n", err)
		wg.Done()
		return
	}
	defer sub.Unsubscribe()

	// Keep the receiver running until shutdown signal is received
	<-shutdown
	wg.Done()
}

func init() {
	err := InitNATSConnection()
	if err != nil {
		fmt.Println("Failed to initialize NATS connection:", err)
		return
	}

	wg.Add(1)
	//go startMessageReceiver()

	time.Sleep(2 * time.Second)

	//err = SendMessage("This is a test message")
	//if err != nil {
	//	fmt.Println("Failed to send test message:", err)
	//}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("Received shutdown signal, closing NATS connection...")

		close(shutdown)

		wg.Wait()
		if nc != nil {
			nc.Close()
		}
		fmt.Println("NATS connection closed, program exiting.")
	}()

	ticker = time.NewTicker(timeout)
	go checkEventQueue()
}
