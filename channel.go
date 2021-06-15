package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type channelMetadata struct {
	connMetadata
	channelID   int
	channelType string
}

func (metadata channelMetadata) getLogEntry() *logrus.Entry {
	return metadata.connMetadata.getLogEntry().WithFields(logrus.Fields{
		"channel_id":   metadata.channelID,
		"channel_type": metadata.channelType,
	})
}

type channelData fmt.Stringer

type channelDataParser func(data []byte) (channelData, error)

type tcpipChannelData struct {
	Address           string
	Port              uint32
	OriginatorAddress string
	OriginatorPort    uint32
}

func (data tcpipChannelData) String() string {
	return fmt.Sprintf("%v -> %v", net.JoinHostPort(data.OriginatorAddress, fmt.Sprint(data.OriginatorPort)), net.JoinHostPort(data.Address, fmt.Sprint(data.Port)))
}

var channelDataParsers = map[string]channelDataParser{
	"session": func(data []byte) (channelData, error) { return nil, nil },
	"direct-tcpip": func(data []byte) (channelData, error) {
		tcpipData := &tcpipChannelData{}
		if err := ssh.Unmarshal(data, tcpipData); err != nil {
			return nil, err
		}
		return tcpipData, nil
	},
}

func handleNewChannel(newChannel ssh.NewChannel, metadata channelMetadata) error {
	accept := true
	var data channelData
	if parser := channelDataParsers[newChannel.ChannelType()]; parser == nil {
		log.Println("Unsupported channel type", newChannel.ChannelType())
		accept = false
	} else {
		var err error
		data, err = parser(newChannel.ExtraData())
		if err != nil {
			return err
		}
	}
	var channelDataString string
	if data != nil {
		channelDataString = fmt.Sprint(data)
	} else {
		channelDataString = base64.RawStdEncoding.EncodeToString(newChannel.ExtraData())
	}
	metadata.getLogEntry().WithFields(logrus.Fields{
		"channel_extra_data": channelDataString,
		"accepted":           accept,
	}).Infoln("New channel requested")

	if !accept {
		return newChannel.Reject(ssh.Prohibited, "")
	}

	channel, requests, err := newChannel.Accept()
	if err != nil {
		return err
	}
	defer channel.Close()
	defer metadata.getLogEntry().Infoln("Channel closed")

	go func() {
		for request := range requests {
			if err := handleChannelRequest(request, metadata); err != nil {
				log.Println("Failed to handle channel request:", err)
				channel.Close()
			}
		}
	}()

	channelInput := make(chan string)
	defer close(channelInput)

	go func() {
		for input := range channelInput {
			metadata.getLogEntry().WithField("input", input).Infoln("Channel input received")
		}
	}()

	switch newChannel.ChannelType() {
	case "direct-tcpip":
		err = handleTCPIPChannel(channel, data.(*tcpipChannelData).Port, channelInput)
	case "session":
		err = handleSessionChannel(channel, channelInput)
	}
	return err
}
