# Locate selection explorer

A self-contained page for comparing Locate's current server selection with the proposed distance-weighted selection, and for experimenting with its parameters (EQ radius, decay exponent, distance cutoff). Open `index.html` in a browser; no build step or server is needed.

Each map point is colored by the site most likely to be returned as the first result for a client at that location. The left map models the current algorithm in `heartbeat/location.go` (strict distance sort, then an exponential draw with rate 6, which picks the nearest site ~95% of the time, effectively a Voronoi diagram). The right map models weighted sampling with `weight = 1/max(distance, EQradius)^k` and an optional relative distance cutoff (sites farther than `ratio` times the nearest site's effective distance are dropped, always keeping at least the 4 nearest). Opacity encodes the winning site's share, so soft boundaries mean traffic is spread across sites. Click any point for a table of per-site first-result probabilities under both algorithms; drag to pan and scroll to zoom (both maps share the view).

Site data is fetched live from `https://siteinfo.mlab-oti.measurementlab.net/v2/sites/registration.json`; if the fetch fails (offline, CORS), an embedded snapshot of the site list is used instead, and a local `registration.json` can be loaded with the file picker.

Limitations, deliberate for simplicity:

- Only the first result of the up-to-4 returned targets is modeled (later draws are without replacement).
- Machine health and site `Probability` are ignored; every site is treated as healthy with probability 1.
- The legacy draw rate is fixed at 6, matching production.
- Distances use the same haversine (R=6371 km) as `github.com/m-lab/go/mathx`.
