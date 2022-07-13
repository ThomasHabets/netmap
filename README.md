
## Importing network map

### FRR / Quagga
```
sudo vtysh -c 'show ipv ospf database intra-prefix detail json' \
    | jq -r '.areaScopedLinkStateDb | .[].lsa | .[] | .advertisingRouter as $adv | .prefix | .[] | [$adv,.prefix,.metric] | @csv' | ./import
```

## TODO

* Point in time network maps
* Multiple named network layouts
* Hilight named but missing routers and links
* Customize colors on routers and links
* Move nodes with mouse
