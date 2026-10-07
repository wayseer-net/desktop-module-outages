// Package outages shows Internet outages from IODA in Wayseer: countries, their regions and
// networks (autonomous systems) as entities, IODA's signals for each as series, and its outage
// events as events.
//
// IODA (Internet Outage Detection and Analysis, https://ioda.inetintel.cc.gatech.edu/) is run
// by the Internet Intelligence Lab at the Georgia Institute of Technology. The module reads its
// public API v2 at https://api.ioda.inetintel.cc.gatech.edu/v2/, which needs no key, as IODA's
// own dashboard does: one request at a time, entities listed again only every six hours,
// signals asked for only since the last read, and no read more often than once a minute. IODA
// states no rate limit; a 429 is waited out as its Retry-After asks.
//
// IODA's responses say: "This data is Copyright (c) 2021-2025 Georgia Tech Research
// Corporation. All Rights Reserved." IODA publishes no other terms for the API. The module
// keeps what it reads in memory only, credits IODA in its description and README, and links
// each entity and outage to its page on IODA's dashboard. Questions about the data go to IODA,
// ioda-info@cc.gatech.edu.
//
// Places are label points from Natural Earth (public domain), embedded in the module: a region
// is placed by the Natural Earth id IODA gives it. An empty answer from IODA is an empty world,
// not an error; a code IODA doesn't know shows as a note in Health.
package outages
