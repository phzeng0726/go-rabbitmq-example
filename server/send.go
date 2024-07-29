package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitMQURL    = "amqp://guest:guest@localhost:5672/"
	loginReqQueue  = "login_req" // The topic of "request login"
	loginResQueue  = "login_res" // The topic of "token response"
	publishTimeout = 5 * time.Second
	messageTTL     = 3 * time.Minute // 3 minutes in milliseconds
)

func loginHandler(ch *amqp.Channel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var authRequest map[string]string
		if err := json.NewDecoder(r.Body).Decode(&authRequest); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		account := authRequest["account"]
		password := authRequest["password"]

		publishAuthRequest(ch, account, password)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Login request sent"))
	}
}

func declareQueueWithTTL(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-message-ttl": int32(messageTTL.Milliseconds()),
		},
	)
}

func publishAuthRequest(ch *amqp.Channel, account, password string) error {
	q, err := declareQueueWithTTL(ch, loginReqQueue)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auth := map[string]string{
		"account":  account,
		"password": password,
	}

	// Serialize the map to JSON
	body, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})
	if err != nil {
		return err
	}

	log.Printf(" [x] Sent %s\n", body)
	return nil
}

// Equipment，send account and password to rabbitmq
func main() {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %s", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %s", err)
	}
	defer ch.Close()

	// Declare a queue for login requests with TTL
	q, err := declareQueueWithTTL(ch, loginResQueue)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %s", err)
	}

	// Start consuming messages
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %s", err)
	}
	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	http.HandleFunc("/login", loginHandler(ch))
	log.Println("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %s", err)
	}

	select {}
}
