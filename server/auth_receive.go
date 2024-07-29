package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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

type AuthRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

func login(authReq AuthRequest) (string, error) {
	if authReq.Account != "my_account" || authReq.Password != "my_password" {
		return "", errors.New("invalid credentials")
	}

	token := "generate_a_success_token_for_user"

	return token, nil
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

func publishAuthResult(ch *amqp.Channel, token string) error {
	q, err := declareQueueWithTTL(ch, loginResQueue)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	err = ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(token),
		})
	if err != nil {
		return err
	}

	log.Printf(" [%s] Sent %s", map[bool]string{true: "o", false: "x"}[token != ""], token)
	return nil
}

func processMessage(body []byte, ch *amqp.Channel) {
	log.Printf("Received a message: %s", body)

	var authReq AuthRequest
	if err := json.Unmarshal(body, &authReq); err != nil {
		log.Printf("Error decoding JSON: %s", err)
		return
	}

	token, err := login(authReq)
	if err != nil {
		log.Printf("Failed to login: %s", err)
		token = "" // Send empty token on failure
	}

	if err := publishAuthResult(ch, token); err != nil {
		log.Printf("Failed to publish auth result: %s", err)
	}
}

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
	q, err := declareQueueWithTTL(ch, loginReqQueue)
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

	log.Println(" [*] Waiting for messages. To exit press CTRL+C")

	for d := range msgs {
		processMessage(d.Body, ch)
	}
}
