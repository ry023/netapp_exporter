# netapp-quota-exporter

export netapp quota for prometheus

```
netapp_quota_exporter --config /path/to/config.yaml
```

### config

```yaml
endpoint: "https://netapp-api-address:1443/"
user: "user"
password: "password"
```

Use `quota_search_condition` if you want to filter quota searches.
(This is useful when there are too many quotas and the API request takes a long time.)

The following example exports all quotas on volume1 and a quota of qtree1 on volume2.

```yaml
endpoint: "https://netapp-api-address:1443/"
user: "user"
password: "password"
quota_search_condition:
- volume: "volume1"
- volume: "volume2"
  qtree: "qtree-1"
```
