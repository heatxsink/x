package iot

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type IoT struct {
	opts   *mqtt.ClientOptions
	client mqtt.Client
}

func New(brokerAddr string, username string, password string, clientID string, cleanSession bool) *IoT {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerAddr)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	opts.SetCleanSession(cleanSession)
	return &IoT{
		opts:   opts,
		client: mqtt.NewClient(opts),
	}
}

func (iot *IoT) Connect() (mqtt.Token, error) {
	var token mqtt.Token
	if token = iot.client.Connect(); token.Wait() && token.Error() != nil {
		return token, token.Error()
	}
	return token, nil
}

func (iot *IoT) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	if token := iot.client.Publish(topic, qos, retained, payload); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (iot *IoT) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	if token := iot.client.Subscribe(topic, qos, callback); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
