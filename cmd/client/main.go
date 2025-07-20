package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")
	const rabbitConnString = "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(rabbitConnString)
	if err != nil {
		log.Fatalf("could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()
	fmt.Println("Peril game client connected to RabbitMQ!")

	publishCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("could not get username: %v", err)
	}
	gs := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+gs.GetUsername(),
		routing.PauseKey,
		pubsub.SimpleQueueTransient,
		handlerPause(gs),
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+gs.GetUsername(),
		routing.ArmyMovesPrefix+".*",
		pubsub.SimpleQueueTransient,
		handlerMove(gs, publishCh),
	)
	if err != nil {
		log.Fatalf("could not subscribe to move: %v", err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		pubsub.SimpleQueueDurable,
		handlerWar(gs, publishCh),
	)
	if err != nil {
		log.Fatalf("could not subscribe to war declarations: %v", err)
	}

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "move":
			move, err := gs.CommandMove(words)
			if err != nil {
				fmt.Println(err)
				continue
			}

			err = pubsub.PublishJSON(
				publishCh,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+gs.Player.Username,
				move,
			)
			if err != nil {
				log.Printf("could not publish move: %v", err)
			}
			fmt.Printf("Moved %v units to %s\n", len(move.Units), move.ToLocation)
		case "spawn":
			err = gs.CommandSpawn(words)
			if err != nil {
				fmt.Println(err)
				continue
			}
		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(words) != 2 {
				fmt.Println("invalid length")
				continue
			}

			i, err := strconv.Atoi(words[1])
			if err != nil {
				fmt.Println("invalid number")
				continue
			}
			for range i {
				message := gamelogic.GetMaliciousLog()

				err = publishGameLog(message, username, publishCh)
				if err != nil {
					fmt.Printf("error publishing malicious log: %s\n", err)
				}
			}
			fmt.Printf("Published %v malicious logs\n", i)
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("unknown command")
		}
	}
}
