package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage/remote"
)

var query = "http_requests_total + http_requests_total offset 30m"
//var query = "rate(http_requests_total[1m]) + rate(http_requests_total[1m] offset 30m)"
var remoteUrl = "http://localhost:9090/api/v1/read"

func main() {
	parsedUrl, err := url.Parse(remoteUrl)
	if err != nil {
		panic(err)
	}

	cfg := &remote.ClientConfig{
		URL: &config_util.URL{parsedUrl},
		Timeout: model.Duration(time.Minute * 2),
	}
	client, err := remote.NewClient(1, cfg)
	if err != nil {
		panic(err)
	}

	engine := promql.NewEngine(nil, prometheus.DefaultRegisterer, 100, time.Minute)

	now := time.Now()
	
	instQuery, err := engine.NewInstantQuery(remote.QueryableClient(client), query, now)
	if err != nil {
        panic(err)
	}

	result := instQuery.Exec(context.Background())


	fmt.Println(result)
	fmt.Println(result.Value)
	fmt.Println(result.Err)
	fmt.Println("Done")
}
