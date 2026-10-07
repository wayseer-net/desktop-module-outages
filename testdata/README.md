# Test fixtures

Responses recorded from [IODA](https://ioda.inetintel.cc.gatech.edu/)'s API v2
(`https://api.ioda.inetintel.cc.gatech.edu/v2/`) on 2026-10-07, for a window ending at
1791381600 (14:00 UTC): outage events over the 24 hours before it, signals over the 2 hours
before it at most 4 points each. Each is trimmed to a few entities, and its `metadata`,
`perf` and `requestParameters` are left out; nothing else is changed.

> This data is Copyright (c) 2021-2025 Georgia Tech Research Corporation. All Rights Reserved.

IODA (Internet Outage Detection and Analysis) is run by the Internet Intelligence Lab at the
Georgia Institute of Technology. These few responses are here only to test this module without
reaching IODA.

| Fixture | Answers |
|---|---|
| `ioda/countries.json` | `entities/query?entityType=country`, trimmed to ET, NZ and TN |
| `ioda/country-events.json` | `outages/events?entityType=country`, trimmed to ET, NZ and TN |
| `ioda/country-<signal>.json` | `signals/raw/country/ET,NZ,TN?datasource=<signal>` for bgp, ping-slash24 and merit-nt |
| `ioda/nz-regions.json` | `entities/query?entityType=region&relatedTo=country/NZ`, trimmed to 3046 and 3047 |
| `ioda/region-events.json` | `outages/events?entityType=region&entityCode=3046,3047` |
| `ioda/region-bgp.json` | `signals/raw/region/3046,3047?datasource=bgp` |
| `ioda/asn-9500.json` | `entities/query?entityType=asn&entityCode=9500` |
| `ioda/asn-9500-countries.json` | `entities/query?entityType=country&relatedTo=asn/9500` |
| `ioda/asn-events.json` | `outages/events?entityType=asn&entityCode=9500` |
| `ioda/asn-bgp.json` | `signals/raw/asn/9500?datasource=bgp` |
| `ioda/nz-gtr-norm.json` | `signals/raw/country/NZ?datasource=gtr-norm` |
| `source/v2/...` | The source for `countries: [NZ]` and `signals: [bgp]`, one file per path |

`source/` is the fixture the Wayseer marketplace serves as IODA when it checks the module: its
sandbox serves the directory as plain files at `http://127.0.0.1:8080/`, ignoring query strings,
so the options it runs with ask only one question of each path:

```yaml
api: http://127.0.0.1:8080/v2
countries: [NZ]
signals: [bgp]
```
