// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azureDevops

import (
	"encoding/base64"
	"github.com/google/uuid"
	"strings"
	"sync"
)

func NewConnection(organizationUrl string, personalAccessToken string) *Connection {
	authorizationString := CreateBasicAuthHeaderValue("", personalAccessToken)
	organizationUrl = normalizeUrl(organizationUrl)
	return &Connection{
		AuthorizationString:     authorizationString,
		BaseUrl:                 organizationUrl,
		SuppressFedAuthRedirect: true,
	}
}

func NewAnonymousConnection(organizationUrl string) *Connection {
	organizationUrl = normalizeUrl(organizationUrl)
	return &Connection{
		BaseUrl:                 organizationUrl,
		SuppressFedAuthRedirect: true,
	}
}

type Connection struct {
	AuthorizationString     string
	BaseUrl                 string
	UserAgent               string
	SuppressFedAuthRedirect bool
	ForceMsaPassThrough     bool
}

func CreateBasicAuthHeaderValue(username, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

func normalizeUrl(url string) string {
	return strings.ToLower(strings.TrimRight(url, "/"))
}

func (connection Connection) GetClientByResourceAreaId(resourceAreaID uuid.UUID) (*Client, error) {
	resourceAreInfo, err := connection.getResourceAreaInfo(resourceAreaID)
	if err != nil {
		return nil, err
	}
	client := connection.GetClientByUrl(*resourceAreInfo.LocationUrl)
	return client, nil
}

func (connection Connection) GetClientByUrl(baseUrl string) *Client {
	normalizedUrl := normalizeUrl(baseUrl)
	azureDevOpsClient, ok := getClientCacheEntry(normalizedUrl)
	if !ok {
		azureDevOpsClient = NewClient(connection, normalizedUrl)
		setClientCacheEntry(normalizedUrl, azureDevOpsClient)
	}
	return azureDevOpsClient
}

func (connection Connection) getResourceAreaInfo(resourceAreaId uuid.UUID) (*ResourceAreaInfo, error) {
	resourceAreaInfo, ok := getResourceAreaCacheEntry(resourceAreaId)
	if !ok {
		client := connection.GetClientByUrl(connection.BaseUrl)
		resourceAreaInfos, err := client.GetResourceAreas()
		if err != nil {
			return nil, err
		}

		for _, resourceEntry := range *resourceAreaInfos {
			setResourceAreaCacheEntry(*resourceEntry.Id, resourceEntry)
		}
		resourceAreaInfo, ok = getResourceAreaCacheEntry(resourceAreaId)
	}

	if ok {
		return &resourceAreaInfo, nil
	}

	return nil, &ResourceAreaIdNotRegisteredError { resourceAreaId, connection.BaseUrl }
}

// Client Cache by Url
var clientCache = make(map[string] *Client)
var clientCacheLock = sync.RWMutex{}

func getClientCacheEntry(url string) (*Client, bool) {
	clientCacheLock.RLock()
	defer clientCacheLock.RUnlock()
	locationsMap, ok := clientCache[url]
	return locationsMap, ok
}

func setClientCacheEntry(url string, client *Client) {
	clientCacheLock.Lock()
	defer clientCacheLock.Unlock()
	clientCache[url] = client
}

// Resource Area Cache by Url
var resourceAreaCache = make(map[uuid.UUID] ResourceAreaInfo)
var resourceAreaCacheLock = sync.RWMutex{}

func getResourceAreaCacheEntry(resourceAreaId uuid.UUID) (ResourceAreaInfo, bool) {
	resourceAreaCacheLock.RLock()
	defer resourceAreaCacheLock.RUnlock()
	resourceAreaInfo, ok := resourceAreaCache[resourceAreaId]
	return resourceAreaInfo, ok
}

func setResourceAreaCacheEntry(resourceAreaId uuid.UUID, resourceAreaInfo ResourceAreaInfo) {
	resourceAreaCacheLock.Lock()
	defer resourceAreaCacheLock.Unlock()
	resourceAreaCache[resourceAreaId] = resourceAreaInfo
}

type ResourceAreaIdNotRegisteredError struct {
	ResourceAreaId uuid.UUID
	Url string
}

func (e ResourceAreaIdNotRegisteredError) Error() string {
	return "API resource area Id " + e.ResourceAreaId.String() + " is not registered on " + e.Url + "."
}