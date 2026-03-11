package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"	

	amqp "github.com/rabbitmq/amqp091-go"	
)

func main() {
	const rabbitConnString = "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(rabbitConnString)
	if err != nil {
		log.Fatalf("could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()
	
	fmt.Println("Starting Peril client...")

	userName, err := pubsub.ClientWelcome()
	queueName := routing.PauseKey + "." + userName
	pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, transient)
}
