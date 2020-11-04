# netapp-exporter

export netapp information

```
netapp_exporter \
  --api.endpoint="https://netapp-api:1443/" \
  --api.user="you" \
  --api.password="YOUR_PASSWORD" \
  --web.listen-address=":9797" \
  --web.telemetry-path="/metrics"
```

Use `--api.search-config` flag and create a config file if you want to filter quota searches.
(This is useful when there are too many quotas and the API request takes a long time.)

The following example exports all quotas on volume1 and a quota of qtree1 on volume2.

```yaml
quota_search_condition:
- volume: "volume1"
- volume: "volume2"
  qtree: "qtree-1"
```

### metrics

| metric                                         | label                   |
| ---------------------------------------------- | ----------------------- |
| netapp_quota_disk_limit_kbytes                 | qtree, volume, vserver  |
| netapp_quota_disk_use_kbytes                   | qtree, volume, vserver  |
| netapp_quota_file_limit                        | qtree, volume, vserver  |
| netapp_quota_file_use                          | qtree, volume, vserver  |
| netapp_quota_status                            | volume, vserver, status |
| netapp_volume_filesystem_metadata_used_kbytes  | volume, vserver         |
| netapp_volume_filesystem_metadata_use_rate     | volume, vserver         |
| netapp_volume_performance_metadata_used_kbytes | volume, vserver         |
| netapp_volume_performance_metadata_use_rate    | volume, vserver         |
| netapp_volume_physical_used_kbytes             | volume, vserver         |
| netapp_volume_physical_use_rate                | volume, vserver         |
| netapp_volume_snapshot_reserve_used_kbytes     | volume, vserver         |
| netapp_volume_snapshot_reserve_use_rate        | volume, vserver         |
| netapp_volume_total_used_kbytes                | volume, vserver         |
| netapp_volume_total_use_rate                   | volume, vserver         |
| netapp_volume_user_used_kbytes                 | volume, vserver         |
| netapp_volume_user_use_rate                    | volume, vserver         |
