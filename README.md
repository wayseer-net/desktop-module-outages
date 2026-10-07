# Wayseer module: outages

Internet outages on Wayseer's map, from [IODA](https://ioda.inetintel.cc.gatech.edu/)
(Internet Outage Detection and Analysis, Georgia Tech). It shows countries, their regions and
networks (autonomous systems) with their health, charts IODA's signals for each, and turns
IODA's outage events into events. It runs as its own program, from a signed package the
marketplace publishes; it is not built into the app.

## What it reads

IODA's public API v2, `https://api.ioda.inetintel.cc.gatech.edu/v2/`, which needs no key:

| Request | For |
|---|---|
| `entities/query` | The countries, regions and networks watched, and the countries each network serves |
| `outages/events` | Outages over the lookback, per entity type |
| `signals/raw/<type>/<codes>` | One signal for every watched entity of a type at once |

It reads one request at a time, every `interval`. Entities change rarely, so they are listed
again only every six hours. Signals are read since the last read, less three hours, since IODA
fills recent points late. IODA states no rate limit; a `429` is waited out as long as its
`Retry-After` asks, and failures back off from 10 seconds up to the interval. A read of every
country takes IODA about 25 seconds, so entities and outages are sent first and the series
fill in after.

It keeps only a working set in memory: the entities, each signal's points over the lookback,
the outages last seen, and the last 1000 events sent. Nothing is written to disk.

## What it shows

| IODA | In Wayseer |
|---|---|
| Country | `outages/country` (drawn as a cluster), placed at its Natural Earth label point |
| Region | `outages/region` (drawn as a node), placed at its Natural Earth label point, `member_of` its country |
| Network (ASN) | `outages/asn` (drawn as a service), unplaced, `member_of` each watched country it serves |
| Outage going on | The entity's status is `crit`, with which signals dropped and since when |
| Outage first seen | An `outage` event at its start, `warn` below score 1,000, `error` below 100,000, else `critical` |
| Outage seen going on, then over | An `outage-ended` event at its end, `info` |
| Signal | A series `ioda.<signal>` per entity |

Events carry the signal (`datasource`), detection `method`, IODA's `score`, `start`, `end` once
over, and an `ioda` link to the outage on IODA's dashboard. Each entity has its `code` and an
`ioda` link to its page; a network also has its `org`, its `addresses` and the `countries` IODA
says it serves.

| Metric | Unit | Kinds | IODA signal |
|---|---|---|---|
| `ioda.bgp` | count | all | /24 blocks visible in BGP to most full-feed peers |
| `ioda.ping-slash24` | count | all | /24 blocks that answer active probing |
| `ioda.merit-nt` | count | all | Unique source IPs a minute at the Merit network telescope |
| `ioda.gtr-norm` | ratio | countries | Google Transparency Report traffic, normalised |

IODA's other signals give their points as objects rather than numbers, and are not charted.

An empty answer from IODA is an empty world, not an error. A watched code IODA doesn't know is
a note in Health, such as `IODA knows no country ZZ`.

## Options

```yaml
modules:
  - kind: external
    name: outages
    options:
      module: wayseer-labs/outages
      options:
        countries: [NZ, AU]          # none for every country
        regions: true
        asns: [9500]
        signals: [bgp, ping-slash24]
```

| Option | Default | Meaning |
|---|---|---|
| `countries` | every country | ISO 3166-1 alpha-2 codes to watch, at most 300. |
| `regions` | `false` | Also each listed country's regions; needs `countries`, at most 20 of them. |
| `asns` | none | Autonomous systems to watch, at most 50. |
| `signals` | `[bgp, ping-slash24, merit-nt]` | Which of `bgp`, `ping-slash24`, `merit-nt` and `gtr-norm` to chart. |
| `interval` | `5m` | How often IODA is read; at least `1m`. |
| `lookback` | `24h` | How far back outages and series reach, from `1h` to `168h`, and at least the interval. |
| `timeout` | `1m` | Longest wait for one request, up to `5m`. |
| `api` | IODA's API v2 | Another only for a mirror, or for tests. |

## Data, licences and credit

The data is IODA's. Its responses say:

> This data is Copyright (c) 2021-2025 Georgia Tech Research Corporation. All Rights Reserved.

IODA publishes no other terms for its API, which its own public dashboard uses. The module reads
it as that dashboard does, names itself in its `User-Agent`, keeps what it reads only in memory,
and links every entity and outage back to IODA. IODA is run by the Internet Intelligence Lab at
the Georgia Institute of Technology; questions about the data go to `ioda-info@cc.gatech.edu`.

Places are label points from [Natural Earth](https://www.naturalearthdata.com/) 1:10m (public
domain), in `places/`: countries by their ISO code, and regions by the Natural Earth id IODA
gives them. `scripts/places.go` writes them from Natural Earth's GeoJSON. A few of IODA's
countries (`AN`, `AP`, `EU`) and regions have no point and stay unplaced.

The test fixtures are a few small responses recorded from IODA, credited in
`testdata/README.md`.

## Working on it

```
make check   # what CI runs: tests with the conformance suite, vet and lint for every platform, a key scan
make help    # every target
```

The tests never reach IODA: they serve the recorded fixtures from `httptest`, with a clock set to
when they were recorded. `TestConformance` runs the SDK's suite against `testdata/source` served
as plain files, as the marketplace's sandbox serves it.

The module imports only the SDK (`wayseer.dev/sdk`) and the standard library;
`TestImportsOnlyTheSDK` keeps it that way. `TestMakeSignPackagesTheModule` needs Wayseer's own
source in `../core`, and skips without it.

## Publishing

The marketplace builds the module from a tag and runs the conformance suite in a sandbox with no
network, serving `testdata/source` at `http://127.0.0.1:8080/`. The submission's options:

```yaml
conformance:
  options: |
    api: http://127.0.0.1:8080/v2
    countries: [NZ]
    signals: [bgp]
  failing: |
    api: http://127.0.0.1:1/v2
  fixture: testdata/source
```

To sign a package yourself instead, with a developer key and certificate:

```
export WAYSEER_DEV_KEY=~/.config/wayseer/dev.pem
export WAYSEER_DEV_CERT=~/.config/wayseer/dev.cert
make sign
```

## Licence

MIT; see `LICENSE`.
