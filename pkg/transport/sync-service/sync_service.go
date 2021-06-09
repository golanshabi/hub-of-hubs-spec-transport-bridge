package sync_service

import (
	"bytes"
	"fmt"
	"github.com/open-horizon/edge-sync-service-client/client"
	"log"
	"os"
	"strconv"
	"sync"
)

const (
	syncServiceProtocol = "SYNC_SERVICE_PROTOCOL"
	syncServiceHost = "SYNC_SERVICE_HOST"
	syncServicePort = "SYNC_SERVICE_PORT"
)

type SyncService struct {
	client		*client.SyncServiceClient
	msgChan		chan *syncServiceMessage
	stopChan	chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
}

func NewSyncService() *SyncService {
	serverProtocol, host, port := readEnvVars()
	syncServiceClient := client.NewSyncServiceClient(serverProtocol, host, port)
	syncServiceClient.SetOrgID("myorg")
	syncServiceClient.SetAppKeyAndSecret("user@myorg", "")
	return &SyncService {
		client:   syncServiceClient,
		msgChan:  make(chan *syncServiceMessage),
		stopChan: make(chan struct{}, 1),
	}
}

func readEnvVars() (string, string, uint16) {
	protocol := os.Getenv(syncServiceProtocol)
	if protocol == "" {
		log.Fatalf("the expected var %s is not set in environment variables", syncServiceProtocol)
	}
	host := os.Getenv(syncServiceHost)
	if host == "" {
		log.Fatalf("the expected var %s is not set in environment variables", syncServiceHost)
	}
	portStr := os.Getenv(syncServicePort)
	if portStr == "" {
		log.Fatalf("the expected env var %s is not set in environment variables", syncServicePort)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("the expected env var %s is not from type uint", syncServicePort)
	}
	return protocol, host, uint16(port)
}

func (s *SyncService) Start() {
	s.startOnce.Do(func() {
		go s.distributeMessages()
	})
}

func (s *SyncService) Stop() {
	close(s.stopChan)
}

func (s *SyncService) Send(id string, msgType string, version string, payload []byte) {
	message := &syncServiceMessage{
		id: id,
		msgType: msgType,
		version: version,
		payload: payload,
	}
	s.msgChan <- message
}

// if the object doesn't exist or an error occurred returns an empty string
func (s *SyncService) GetVersion(id string, msgType string) string {
	objectMetadata, err := s.client.GetObjectMetadata(msgType, id)
	if err != nil {
		return ""
	}
	return objectMetadata.Version
}

func (s *SyncService) distributeMessages() {
	for {
		select {
		case <-s.stopChan:
			return
		case msg := <-s.msgChan:
			metaData := client.ObjectMetaData {
				ObjectID: msg.id,
				ObjectType: msg.msgType,
				Version: msg.version,
			}
			err := s.client.UpdateObject(&metaData)
			if err != nil {
				fmt.Printf("Failed to update the object in the Cloud Sync Service. Error: %s\n", err)
				os.Exit(1)
			}
			reader := bytes.NewReader(msg.payload)
			err = s.client.UpdateObjectData(&metaData, reader)
			if err != nil {
				fmt.Printf("Failed to update the object data in the Cloud Sync Service. Error: %s\n", err)
				os.Exit(1)
			}
			log.Printf("Message '%s' from type '%s' with version '%s' sent", msg.id, msg.msgType, msg.version)
		}
	}
}
