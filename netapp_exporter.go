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

const namespace string = "netapp"

var (
	diskLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "quota_disk_limit_kbytes"),
		"Qtree disk soft limit in kbytes",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	diskUsed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "quota_disk_use_kbytes"),
		"Qtree disk current use in kbytes",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	fileLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "quota_file_limit"),
		"Qtree number of file soft limit",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	fileUsed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "quota_file_use"),
		"Qtree number of file soft limit",
		[]string{"qtree", "volume", "vserver"},
		nil,
	)

	status = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "quota_status"),
		"Quota status of volume",
		[]string{"volume", "vserver", "status"},
		nil,
	)

	volumeTotalUsedBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_total_used_kbytes"),
		"Total usage of volume (kbytes)",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeTotalUseRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_total_use_rate"),
		"Total use rate of volume",
		[]string{"volume", "vserver"},
		nil,
	)

	volumePhysicalUsedBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_physical_used_kbytes"),
		"Physical usage of volume (kbytes)",
		[]string{"volume", "vserver"},
		nil,
	)

	volumePhysicalUseRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_physical_use_rate"),
		"Physical use rate of volume",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeUserUsedBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_user_used_kbytes"),
		"User usage of volume (kbytes)",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeUserUseRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_user_use_rate"),
		"User use rate of volume",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeFilesystemMetadataUsedBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_filesystem_metadata_used_kbytes"),
		"FilesystemMetadata usage of volume (kbytes)",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeFilesystemMetadataUseRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_filesystem_metadata_use_rate"),
		"FilesystemMetadata use rate of volume",
		[]string{"volume", "vserver"},
		nil,
	)

	volumePerformanceMetadataUsedBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_performance_metadata_used_kbytes"),
		"PerformanceMetadata usage of volume (kbytes)",
		[]string{"volume", "vserver"},
		nil,
	)

	volumePerformanceMetadataUseRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_performance_metadata_use_rate"),
		"PerformanceMetadata use rate of volume",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeSnapshotReserveUsedBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_snapshot_reserve_used_kbytes"),
		"Snapshot reserve usage of volume (kbytes)",
		[]string{"volume", "vserver"},
		nil,
	)

	volumeSnapshotReserveUseRate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "volume_snapshot_reserve_use_rate"),
		"Snapshot reserve use rate of volume",
		[]string{"volume", "vserver"},
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

// go-netappからの取得値はint or stringなので、exporter用にfloat64に変換する
func toFloat(val interface{}) (float64, error) {
	floatVal, isFloat := val.(float64)
	if isFloat {
		// already float
		return floatVal, nil
	}
	intVal, isInt := val.(int)

	if isInt {
		return (float64)(intVal), nil
	}

	strVal, isStr := val.(string)
	if isStr {
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			return 0.0, err
		}
		return (float64)(intVal), nil
	}

	return 0.0, fmt.Errorf("value (%v) is neither Int or String or Float", val)
}

func sendMetric(desc *prometheus.Desc, value interface{}, labels []string, ch chan<- prometheus.Metric) {
	metricVal, err := toFloat(value)
	if err != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(
		desc,
		prometheus.GaugeValue,
		metricVal,
		labels...,
	)
}

func sendQuotaMetric(desc *prometheus.Desc, value interface{}, quota netapp.QuotaReportEntry, ch chan<- prometheus.Metric) {
	sendMetric(desc, value, []string{quota.Tree, quota.Volume, quota.Vserver}, ch)
}

func sendVolumeMetric(desc *prometheus.Desc, value interface{}, volume netapp.VolumeSpaceInfo, ch chan<- prometheus.Metric) {
	sendMetric(desc, value, []string{volume.Volume, volume.Vserver}, ch)
}

func sendVolumeRateMetric(desc *prometheus.Desc, value interface{}, volume netapp.VolumeSpaceInfo, ch chan<- prometheus.Metric) {
	floatVal, err := toFloat(value)
	if err != nil {
		return
	}
	// go-netappからはパーセントで取得されるので、比率に変換する(exporterのベストプラクティス)
	sendVolumeMetric(desc, floatVal/100, volume, ch)
}

func (c quotaCollector) Collect(ch chan<- prometheus.Metric) {
	volumes, err := c.GetVolumeSpaces()

	if err == nil {
		for _, v := range volumes {
			// export volume value
			sendVolumeMetric(volumeTotalUsedBytes, v.TotalUsed, v, ch)
			sendVolumeMetric(volumePhysicalUsedBytes, v.PhysicalUsed, v, ch)
			sendVolumeMetric(volumeUserUsedBytes, v.UserData, v, ch)
			sendVolumeMetric(volumeFilesystemMetadataUsedBytes, v.FilesystemMetadata, v, ch)
			sendVolumeMetric(volumePerformanceMetadataUsedBytes, v.PerformanceMetadata, v, ch)
			sendVolumeMetric(volumeSnapshotReserveUsedBytes, v.SnapshotReserve, v, ch)

			// export volume use rate
			sendVolumeRateMetric(volumeTotalUseRate, v.TotalUsedPercent, v, ch)
			sendVolumeRateMetric(volumePhysicalUseRate, v.PhysicalUsedPercent, v, ch)
			sendVolumeRateMetric(volumeUserUseRate, v.UserDataPercent, v, ch)
			sendVolumeRateMetric(volumeFilesystemMetadataUseRate, v.FilesystemMetadataPercent, v, ch)
			sendVolumeRateMetric(volumePerformanceMetadataUseRate, v.PerformanceMetadataPercent, v, ch)
			sendVolumeRateMetric(volumeSnapshotReserveUseRate, v.SnapshotReservePercent, v, ch)

			s, err := c.GetQuotaStatus(v)
			if err != nil {
				continue
			}
			if s == "" {
				continue
			}
			ch <- prometheus.MustNewConstMetric(status, prometheus.GaugeValue, 1, v.Volume, v.Vserver, s)
		}
	}

	for _, condition := range c.conditions {
		quotas, err := c.GetQuotas(condition)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		for _, q := range quotas {
			// export quota metrics
			sendQuotaMetric(diskLimit, q.DiskLimit, q, ch)
			sendQuotaMetric(diskUsed, q.DiskUsed, q, ch)
			sendQuotaMetric(fileLimit, q.FileLimit, q, ch)
			sendQuotaMetric(fileUsed, q.FilesUsed, q, ch)
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

type SearchConfig struct {
	QuotaSearchCondition []QuotaSeachCondition `yaml:"quota_search_condition"`
}

func loadSearchConfig(path string) (*SearchConfig, error) {
	config := &SearchConfig{}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		switch err.(type) {
		case *os.PathError:
			// skip if notfound
		default:
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(b, config); err != nil {
			return nil, err
		}
	}

	if len(config.QuotaSearchCondition) == 0 {
		config.QuotaSearchCondition = []QuotaSeachCondition{{}}
	}
	return config, nil
}

func main() {
	configPath := kingpin.Flag("api.search-config", "search Config file path").Default("/etc/netapp_quota_exporter.conf").String()
	endpoint := kingpin.Flag("api.endpoint", "NetApp API endpoint").String()
	user := kingpin.Flag("api.user", "NetApp API auth user").String()
	password := kingpin.Flag("api.password", "NetApp API auth password").String()
	listenAddress := kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9797").String()
	metricsPath := kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	kingpin.Parse()
	sconf, err := loadSearchConfig(*configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	c, err := NewQuotaCollector(*endpoint, *user, *password, sconf.QuotaSearchCondition)
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}

	prometheus.Register(c)
	http.Handle(*metricsPath, promhttp.Handler())
	http.ListenAndServe(*listenAddress, nil)
}
