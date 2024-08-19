package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/foxinuni/taller-mqtt-patrones/internal"
)

var (
	CurrentNode int
	ImagePath   string
	BrokerURI   string
	ClientID    string
)

func init() {
	flag.IntVar(&CurrentNode, "node", 1, "Current node number")
	flag.StringVar(&ImagePath, "image", "", "Path to the image file")
	flag.StringVar(&BrokerURI, "broker", "test.mosquitto.org:1883", "MQTT broker URI")
	flag.StringVar(&ClientID, "client", "gnode", "MQTT client ID")
	flag.Parse()

	if ImagePath == "" {
		fmt.Printf("Usage: %s [... args]\n", flag.Arg(0))
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Printf("Current node: %d\n", CurrentNode)
	fmt.Printf("Image path: %s\n", ImagePath)
}

func main() {
	// Read the image from the file
	img, err := internal.ReadImage(ImagePath)
	if err != nil {
		fmt.Println("Failed to read the image.")
		panic(err)
	}

	// Parse the barcode from the image
	barcode, err := internal.ParseBarcode(img)
	if err != nil {
		fmt.Println("Failed to parse the barcode.")
		panic(err)
	}

	fmt.Printf("Barcode: %s\n", barcode)

	// Create a new MQTT client
	client, err := internal.NewMqttClient(BrokerURI, fmt.Sprintf("%s-%d", ClientID, CurrentNode))
	if err != nil {
		fmt.Println("Failed to create a new MQTT client.")
		panic(err)
	}
	defer client.Close()

	// Subscribe to the barcode topic
	fmt.Printf("Subscribing to the topic %q...\n", topicForNode(CurrentNode))
	if err := client.Subscribe(topicForNode(CurrentNode), handleTopic); err != nil {
		fmt.Println("Failed to subscribe to the topic.")
		panic(err)
	}

	// Check if I am the publisher
	if strings.HasPrefix(barcode, fmt.Sprintf("%d", CurrentNode)) {
		fmt.Println("Client is the publisher. Attempting to publish the message.")

		// Get next in line
		nextNode, ok := nextInLine(barcode, CurrentNode)
		if !ok {
			fmt.Println("Failed to get the next in line.")
			os.Exit(1)
		}

		// Publish the message to the topic
		if err := client.Publish(topicForNode(nextNode), barcode); err != nil {
			fmt.Println("Failed to publish the message.")
			panic(err)
		}

		fmt.Printf("Message published to %d\n", nextNode)
	}

	// Wait for the signal to exit
	fmt.Printf("Client is running. Press Ctrl+C to exit.\n")
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt)

	<-sigterm
}

func handleTopic(_ context.Context, client *internal.MqttClient, message mqtt.Message) {
	payload := string(message.Payload())
	fmt.Printf("Received message: %q\n", payload)

	// Get next in line
	nextNode, ok := nextInLine(payload, CurrentNode)
	if !ok {
		fmt.Println("Last node. Nothing to do.")
		return
	}

	// Publish the message to the next node
	if err := client.Publish(topicForNode(nextNode), payload); err != nil {
		fmt.Println("Failed to publish the message.")
		panic(err)
	}
}

func topicForNode(node int) string {
	return fmt.Sprintf("PATRONES/%d", node)
}

func nextInLine(barcode string, node int) (int, bool) {
	nodeStr := fmt.Sprintf("%d", node)
	for i := 0; i < len(barcode)-1; i++ {
		if barcode[i] == nodeStr[0] {
			return int(barcode[i+1] - '0'), true
		}
	}

	return 0, false
}
