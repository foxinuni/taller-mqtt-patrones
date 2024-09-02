package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"image/jpeg"
	"os"
	"os/signal"
	"strconv"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/foxinuni/taller-mqtt-patrones/internal"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

const PADDING = 6

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
		if err := client.Publish(topicForNode(nextNode), nextBarcode(barcode)); err != nil {
			fmt.Println("Failed to publish the message.")
			panic(err)
		}

		fmt.Printf("Message published to %q\n", topicForNode(nextNode))
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
		fmt.Println("Last node. Sending email.")
		if err := sendEmail(CurrentNode, payload); err != nil {
			fmt.Println("Failed to send email.")
			panic(err)
		}

		return
	}

	// Publish the message to the next node
	fmt.Printf("Message published to %q\n", topicForNode(nextNode))
	if err := client.Publish(topicForNode(nextNode), nextBarcode(payload)); err != nil {
		fmt.Println("Failed to publish the message.")
		panic(err)
	}
}

func topicForNode(node int) string {
	return fmt.Sprintf("PATRONES2024/GRUPO%d", node)
}

func nextInLine(barcode string, node int) (int, bool) {
	nodeStr := fmt.Sprintf("%d", node)
	for i := 0; i < len(barcode)-PADDING-1; i++ {
		if barcode[i] == nodeStr[0] {
			return int(barcode[i+1] - '0'), true
		}
	}

	return 0, false
}

// adds 1 the padding of the barcode
func nextBarcode(barcode string) string {
	padding := barcode[len(barcode)-PADDING:]
	paddingInt, err := strconv.Atoi(padding)
	if err != nil {
		return ""
	}

	paddingInt++
	return fmt.Sprintf("%s%0*d", barcode[:len(barcode)-PADDING], PADDING, paddingInt)
}

func sendEmail(node int, barcode string) error {
	image, err := internal.GenerateBarcode(barcode)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, image, nil); err != nil {
		return err
	}

	encodedImage := base64.StdEncoding.EncodeToString(buf.Bytes())

	from := mail.NewEmail(fmt.Sprintf("Grupo #%d", node), os.Getenv("SENDGRID_EMAIL"))
	to := mail.NewEmail("Example User", os.Getenv("SENDGRID_RECEIVER"))
	subject := fmt.Sprintf("Grupo #%d - mqtt barcode", node)
	content := mail.NewContent("text/html", "<strong>A continuacion se muestra el codigo de barras</strong> <img src='cid:barcode' />")

	attachment := mail.NewAttachment()
	attachment.SetType("image/jpeg")
	attachment.SetFilename("barcode.jpg")
	attachment.SetContent(encodedImage)
	attachment.SetDisposition("attachment")
	attachment.SetContentID("barcode")

	message := mail.NewV3MailInit(from, subject, to, content)
	message.AddAttachment(attachment)

	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	_, err = client.Send(message)
	return err
}
