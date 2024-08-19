package internal

import (
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MqttHandlerFunc func(ctx context.Context, client *MqttClient, message mqtt.Message)

type MqttClient struct {
	client mqtt.Client
}

func NewMqttClient(broker string, clientId string) (*MqttClient, error) {
	// Create options for the MQTT client
	options := mqtt.NewClientOptions()
	options.AddBroker(broker)
	options.SetClientID(clientId)

	// Client connection
	client := mqtt.NewClient(options)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &MqttClient{client: client}, nil
}

func (mc *MqttClient) Subscribe(topic string, handler MqttHandlerFunc) error {
	token := mc.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
		handler(context.Background(), mc, message)
	})

	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (mc *MqttClient) Publish(topic string, payload interface{}) error {
	token := mc.client.Publish(topic, 0, false, payload)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	
	return nil
}

func (mc *MqttClient) Close() {
	mc.client.Disconnect(250)
}
