package main

// Cheers! https://github.com/go-chef/chef/blob/master/http.go

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chef/chef"
	uuid "github.com/satori/go.uuid"
)

const iso8601DateTime = "2006-01-02T15:04:05Z"

// DataCollectorConfig holds our configuration for the Data Collector
type DataCollectorConfig struct {
	Token   string
	URL     string
	SkipSSL bool
	Timeout time.Duration
}

// DataCollectorClient has our configured HTTP client, our Token and the URL
type DataCollectorClient struct {
	Client *http.Client
	Token  string
	URL    *url.URL
}

type expandedRunListItem struct {
	ItemType string  `json:"type"`
	Name     string  `json:"name"`
	Version  *string `json:"version"`
	Skipped  bool    `json:"skipped"`
}

// NewDataCollectorClient builds our Client struct with our Config
func NewDataCollectorClient(cfg *DataCollectorConfig) (*DataCollectorClient, error) {
	URL, _ := url.Parse(cfg.URL)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.SkipSSL},
	}

	c := &DataCollectorClient{
		Client: &http.Client{
			Transport: tr,
			Timeout:   cfg.Timeout * time.Second,
		},
		URL:   URL,
		Token: cfg.Token,
	}
	return c, nil
}

// Update the data collector endpoint with our map
func (dcc *DataCollectorClient) Update(body map[string]interface{}) error {
	// Convert our body to encoded JSON
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(body)
	encodedBody := bytes.NewReader(buf.Bytes())

	// Create an HTTP Request
	req, err := http.NewRequest("POST", dcc.URL.String(), encodedBody)
	if err != nil {
		return err
	}

	// Set our headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-data-collector-auth", "version=1.0")
	req.Header.Set("x-data-collector-token", dcc.Token)

	// Do request
	res, err := dcc.Client.Do(req)

	// Handle response
	if res != nil {
		defer res.Body.Close()
	}

	return err
}

func dataCollectorRunStart(nodeName string, orgName string, runUUID uuid.UUID, nodeUUID uuid.UUID, startTime time.Time, config chefLoadConfig) error {
	msgBody := map[string]interface{}{
		"chef_server_fqdn": config.ChefServerURL,
		"entity_uuid":      nodeUUID.String(),
		"id":               runUUID.String(),
		"message_version":  "1.0.0",
		"message_type":     "run_start",
		"node_name":        nodeName,
		"organization":     orgName,
		"run_id":           runUUID.String(),
		"source":           "chef_client",
		"start_time":       startTime.Format(iso8601DateTime),
	}

	client, err := NewDataCollectorClient(&DataCollectorConfig{
		Token:   config.DataCollectorToken,
		URL:     config.DataCollectorURL,
		SkipSSL: true,
	})

	if err != nil {
		fmt.Printf("Error creating DataCollectorClient: %+v \n", err)
	}

	res := client.Update(msgBody)

	return res
}

func dataCollectorRunStop(node chef.Node, nodeName string, orgName string, runList runList, expandedRunList runList, runUUID uuid.UUID, nodeUUID uuid.UUID, startTime time.Time, endTime time.Time, config chefLoadConfig) error {
	var expandedRunListItems []expandedRunListItem
	for _, runListItem := range expandedRunList {
		erli := expandedRunListItem{
			Name:     runListItem.name,
			ItemType: runListItem.itemType,
			Skipped:  false,
		}
		if runListItem.version != "" {
			version := runListItem.version
			erli.Version = &version
		}
		expandedRunListItems = append(expandedRunListItems, erli)
	}

	expandedRunListMap := map[string]interface{}{
		"id":       config.ChefEnvironment,
		"run_list": expandedRunListItems,
	}

	msgBody := map[string]interface{}{
		"chef_server_fqdn":       config.ChefServerURL,
		"entity_uuid":            nodeUUID.String(),
		"id":                     runUUID.String(),
		"message_version":        "1.0.0",
		"message_type":           "run_converge",
		"node_name":              nodeName,
		"organization":           orgName,
		"run_id":                 runUUID.String(),
		"source":                 "chef_client",
		"start_time":             startTime.Format(iso8601DateTime),
		"end_time":               endTime.Format(iso8601DateTime),
		"status":                 "success",
		"run_list":               runList.toStringSlice(),
		"expanded_run_list":      expandedRunListMap,
		"node":                   node,
		"resources":              []interface{}{},
		"total_resource_count":   0,
		"updated_resource_count": 0,
	}

	client, err := NewDataCollectorClient(&DataCollectorConfig{
		Token:   config.DataCollectorToken,
		URL:     config.DataCollectorURL,
		SkipSSL: true,
	})

	if err != nil {
		fmt.Printf("Error creating DataCollectorClient: %+v \n", err)
	}

	res := client.Update(msgBody)

	return res
}
