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
