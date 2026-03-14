package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	dat, err := json.Marshal(val)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		return err
	}

	// Prepare this message to be persistent.  Your publishing requirements may
	// be different.
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        dat,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx, exchange, key, false, false, msg)
	if err != nil {
		// Since publish is asynchronous this can happen if the network connection
		// is reset or if the server has run out of resources.
		log.Fatalf("basic.publish: %v", err)
		return err
	}

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // SimpleQueueType is an "enum" type I made to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	durable := false
	autodelete := false
	exclusive := false
	nowait := false

	ch, err := conn.Channel()
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}

	if queueType == 0 {
		durable = true
	}

	if queueType == 1 {
		autodelete = true
		exclusive = true
	}

	args := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}
	q, err := ch.QueueDeclare(queueName, durable, autodelete, exclusive, nowait, args)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}

	err = ch.QueueBind(queueName, key, exchange, nowait, args)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}

	return ch, q, nil
}

type SimpleQueueType int

const (
	SimpleQueueDurable SimpleQueueType = iota
	SimpleQueueTransient
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) Acktype,
) error {
	// Call DeclareAndBind
	ch, queue, err := DeclareAndBind(
		conn,
		exchange,
		queueName,
		key,
		queueType,
	)
	if err != nil {
		return fmt.Errorf("could not declare and bind queue: %v", err)
	}

	fmt.Printf("Queue %v declared and bound!\n", queue.Name)

	// Get a new chan of amqp.Delivery structs using channel.Consume method

	msgs, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("could not consume messages: %v", err)
	}

	unmarshaller := func(data []byte) (T, error) {
		var target T
		err := json.Unmarshal(data, &target)
		return target, err
	}

	go func() {
		defer ch.Close()
		for msg := range msgs {
			target, err := unmarshaller(msg.Body)
			if err != nil {
				fmt.Printf("could not unmarshal message: %v\n", err)
				continue
			}
			switch handler(target) {
			case Ack:
				msg.Ack(false)
				fmt.Println("Ack")
			case NackDiscard:
				msg.Nack(false, false)
				fmt.Println("NackDiscard")
			case NackRequeue:
				msg.Nack(false, true)
				fmt.Println("NackRequeue")
			}
		}
	}()
	return nil
}

type Acktype int

const (
	Ack Acktype = iota
	NackDiscard
	NackRequeue
)
