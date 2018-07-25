package shared

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	storage "github.com/Azure/azure-sdk-for-go/storage"
)

//good samples here: https://github.com/luigialves/sample-golang-with-azure-table-storage/blob/master/sample.go

// GameServerEntity represents a pod
type GameServerEntity struct {
	Name           string
	Namespace      string
	PublicIP       string
	NodeName       string
	Status         string
	Port           string
	ActiveSessions string
}

func CreatePort(port int) (bool, error) {
	storageclient := GetStorageClient()
	tableservice := storageclient.GetTableService()
	table := tableservice.GetTableReference(PortsTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)
	stringPort := strconv.Itoa(port)
	entity := table.GetEntityReference(stringPort, stringPort)
	err := entity.Insert(storage.MinimalMetadata, nil)
	if err != nil {
		if strings.Contains(err.Error(), "StatusCode=409") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpsertGameServerEntity upserts entity
func UpsertGameServerEntity(pod *GameServerEntity) error {

	if pod.Name == "" || pod.Namespace == "" {
		return errors.New("New pod should include both Name and Namespace properties")
	}

	storageclient := GetStorageClient()

	tableservice := storageclient.GetTableService()

	table := tableservice.GetTableReference(GameServersTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)

	//partition key is the same as row key (pod's name)
	entity := table.GetEntityReference(pod.Namespace, pod.Name)

	props := make(map[string]interface{})

	if pod.PublicIP != "" {
		props["PublicIP"] = pod.PublicIP
	}

	if pod.NodeName != "" {
		props["NodeName"] = pod.NodeName
	}

	if pod.Status != "" {
		props["Status"] = pod.Status
	}

	fmt.Println("port lala" + pod.Port)

	if pod.Port != "" {
		props["Port"] = pod.Port
	}

	if pod.ActiveSessions != "" {
		props["ActiveSessions"] = pod.ActiveSessions
	}

	entity.Properties = props

	return entity.InsertOrMerge(nil)
}

// GetEntity gets table entity
func GetEntity(namespace, name string) (*storage.Entity, error) {
	storageclient := GetStorageClient()

	tableservice := storageclient.GetTableService()

	table := tableservice.GetTableReference(GameServersTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)

	entity := table.GetEntityReference(namespace, name)

	err := entity.Get(Timeout, storage.MinimalMetadata, nil)

	return entity, err
}

// DeleteDedicatedGameServerEntity deletes DedicatedGameServer table entity. Will suppress 404 errors
func DeleteDedicatedGameServerEntity(namespace, name string) error {
	storageclient := GetStorageClient()

	tableservice := storageclient.GetTableService()

	table := tableservice.GetTableReference(GameServersTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)

	// retrieve entire object
	entity, err := GetEntity(namespace, name)

	if err != nil {
		return err
	}

	port, errAtoi := strconv.Atoi(entity.Properties["Port"].(string))

	if errAtoi != nil {
		return err
	}

	errDeletePort := DeletePort(port)

	if errDeletePort != nil {
		return errDeletePort
	}

	errDelete := entity.Delete(true, nil)

	if errDelete != nil && !strings.Contains(err.Error(), "StatusCode=404, ErrorCode=ResourceNotFound") {
		return errDelete
	}
	return nil
}

// GetRunningEntities returns all entities in the running state
func GetRunningEntities() ([]*storage.Entity, error) {
	storageclient := GetStorageClient()

	tableservice := storageclient.GetTableService()

	table := tableservice.GetTableReference(GameServersTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)

	result, err := table.QueryEntities(Timeout, storage.MinimalMetadata, &storage.QueryOptions{
		Filter: "Status eq 'Running'",
	})

	return result.Entities, err
}

// GetEntitiesMarkedForDeletionWithZeroSessions returns all entities in the marked for deletion state with 0 active sessions
func GetEntitiesMarkedForDeletionWithZeroSessions() ([]*storage.Entity, error) {
	storageclient := GetStorageClient()

	tableservice := storageclient.GetTableService()

	table := tableservice.GetTableReference(GameServersTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)

	result, err := table.QueryEntities(Timeout, storage.MinimalMetadata, &storage.QueryOptions{
		Filter: fmt.Sprintf("Status eq '%s' and ActiveSessions eq '0'", MarkedForDeletionState),
	})

	return result.Entities, err
}

// DeletePort deletes Port table entity. Will suppress 404 errors
func DeletePort(port int) error {
	storageclient := GetStorageClient()

	tableservice := storageclient.GetTableService()

	table := tableservice.GetTableReference(PortsTableName)
	table.Create(Timeout, storage.MinimalMetadata, nil)

	stringPort := strconv.Itoa(port)

	entity := table.GetEntityReference(stringPort, stringPort)

	err := entity.Delete(true, nil)

	if err != nil && !strings.Contains(err.Error(), "StatusCode=404, ErrorCode=ResourceNotFound") {
		return err
	}
	return nil
}
