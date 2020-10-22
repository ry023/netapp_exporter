package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pepabo/go-netapp/netapp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
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

	status = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "status"),
		"Quota status of volume",
		[]string{"volume", "vserver", "status"},
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
	ch <- status
}

func (c quotaCollector) Collect(ch chan<- prometheus.Metric) {
	volumes, err := c.GetVolumeSpaces()

	if err == nil {
		for _, v := range volumes {
			s, err := c.GetQuotaStatus(v)
			if err != nil {
				continue
			}

			if s == "" {
				continue
			}
			ch <- prometheus.MustNewConstMetric(
				status,
				prometheus.GaugeValue,
				1,
				v.Volume, v.Vserver, s,
			)
		}
	}

	for _, condition := range c.conditions {
		quotas, err := c.GetQuotas(condition)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		for _, q := range quotas {
			send := func(desc *prometheus.Desc, value string, quota netapp.QuotaReportEntry, ch chan<- prometheus.Metric) {
				intValue, err := strconv.Atoi(value)
				if err != nil {
					return
				}
				ch <- prometheus.MustNewConstMetric(
					desc,
					prometheus.GaugeValue,
					(float64)(intValue),
					quota.Tree, quota.Volume, quota.Vserver,
				)
			}
			send(diskLimit, q.DiskLimit, q, ch)
			send(diskUsed, q.DiskUsed, q, ch)
			send(fileLimit, q.DiskLimit, q, ch)
			send(fileUsed, q.DiskLimit, q, ch)
		}
	}
}

func (c quotaCollector) GetQuotas(q QuotaSeachCondition) ([]netapp.QuotaReportEntry, error) {
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

func (c quotaCollector) GetVolumeSpaces() ([]netapp.VolumeSpaceInfo, error) {
	r, _, err := c.client.VolumeSpace.List(nil)
	if err != nil {
		return nil, err
	}
	return r.Results.AttributesList.SpaceInfo, nil
}

func (c quotaCollector) GetQuotaStatus(v netapp.VolumeSpaceInfo) (string, error) {
	r, _, err := c.client.Quota.Status(v.Vserver, v.Volume)
	if err != nil {
		return "", err
	}
	return r.Results.QuotaStatus, nil
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

type Config struct {
	Endpoint string
	User     string
	Password string

	QuotaSearchCondition []QuotaSeachCondition `yaml:"quota_search_condition"`
}

func loadConfig(path string) (*Config, error) {
	b, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}
	config := &Config{}
	if err := yaml.Unmarshal(b, config); err != nil {
		return nil, err
	}

	if len(config.QuotaSearchCondition) == 0 {
		config.QuotaSearchCondition = []QuotaSeachCondition{{}}
	}
	return config, nil
}

func main() {
	configPath := kingpin.Flag("config", "Config file path").Default("/etc/netapp_quota_exporter.conf").String()

	kingpin.Parse()
	config, err := loadConfig(*configPath)
	if err != nil {
		os.Exit(2)
	}
	c, err := NewQuotaCollector(config.Endpoint, config.User, config.Password, config.QuotaSearchCondition)
	if err != nil {
		os.Exit(3)
	}

	prometheus.Register(c)
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
