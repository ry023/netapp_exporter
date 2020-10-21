package main

import (
	"fmt"
	"net/http"

	"sync"
	"github.com/pepabo/go-netapp/netapp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"strconv"
	"time"
)

const namespace string = "netapp_quota"

var (
	diskLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "disk_limit"),
		"Qtree disk soft limit in bytes",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	diskUsed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "disk_use"),
		"Qtree disk current use in bytes",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	fileLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "file_limit"),
		"Qtree number of file soft limit",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	fileUsed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "file_use"),
		"Qtree number of file soft limit",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)
)

type quotaCollector struct {
	client     *netapp.Client
	conditions []QuotaSeachCondition
}

func (m *quotaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- diskLimit
	ch <- diskUsed
	ch <- fileLimit
	ch <- fileUsed
}

func (c quotaCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	for _, condition := range c.conditions {
    wg.Add(1)
		go func(cond QuotaSeachCondition, ch chan<- prometheus.Metric) {
      defer wg.Done()
			qtrees, err := c.GetQtrees(cond)
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			for _, qtree := range qtrees {
				send := func(desc *prometheus.Desc, value string, qtree netapp.QuotaReportEntry, ch chan<- prometheus.Metric) {
					intValue, err := strconv.Atoi(value)
					if err != nil {
						return
					}
					ch <- prometheus.MustNewConstMetric(
						desc,
						prometheus.GaugeValue,
						(float64)(intValue),
						qtree.Tree, qtree.Volume, qtree.Vserver,
					)
				}
				send(diskLimit, qtree.DiskLimit, qtree, ch)
				send(diskUsed, qtree.DiskUsed, qtree, ch)
				send(fileLimit, qtree.DiskLimit, qtree, ch)
				send(fileUsed, qtree.DiskLimit, qtree, ch)
			}
		}(condition, ch)
	}
  wg.Wait()
}

func (c quotaCollector) GetQtrees(q QuotaSeachCondition) ([]netapp.QuotaReportEntry, error) {
	nextTag := ""

	qtrees := []netapp.QuotaReportEntry{}
	for {
		qRes, _, err := c.client.QuotaReport.Report(&netapp.QuotaReportOptions{
			MaxRecords: 1000,
			Query: &netapp.QuotaReportEntryQuery{
				QuotaReportEntry: &netapp.QuotaReportEntry{
					Volume:  q.Volume,
					Tree:    q.Qtree,
					Vserver: q.Vserver,
				},
			},
			Tag: nextTag,
		})
		if err != nil {
			return nil, err
		}

		qtrees = append(qtrees, qRes.Results.AttributesList.QuotaReportEntry...)
		nextTag = qRes.Results.NextTag
		if nextTag == "" {
			break
		}
	}

	return qtrees, nil
}

type QuotaSeachCondition struct {
	Qtree   string
	Volume  string
	Vserver string
}

func NewQuotaCollector(endpoint, user, password string, conditions []QuotaSeachCondition) (*quotaCollector, error) {
	client, err := netapp.NewClient(
		endpoint,
		"1.20",
		&netapp.ClientOptions{
			BasicAuthUser:     user,
			BasicAuthPassword: password,
			SSLVerify:         true,
			Timeout:           10 * time.Second,
		},
	)
	if err != nil {
		return nil, err
	}

	if len(conditions) == 0 {
		conditions = []QuotaSeachCondition{{}}
	}
	return &quotaCollector{
		client:     client,
		conditions: conditions,
	}, nil
}

func main() {
	c, err := NewQuotaCollector(
		"https://localhost:1443",
		"",
		"",
		[]QuotaSeachCondition{},
	)
	if err != nil {
		panic(err)
	}
	prometheus.Register(c)

	http.Handle("/metrics", promhttp.Handler())

	http.ListenAndServe(":2112", nil)
}
