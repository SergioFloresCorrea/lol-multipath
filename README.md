# Multipath connection for League of Legends

## Introduction
This project attempts to be a starter point for multipath proxy connections between the game client and the Riot's game servers.
A map what happens is as follows:
```
    ------------------- Proxy A -----------------
    |                                           |
Game Client------------ Proxy B ---------------- Riot's game server
(   includes all           |                    |
internet interfaces        |                    |
e.g WiFi, Ethernet)        |                    |
    |                      |                    |
    ------------------- Proxy N -----------------
```

Ideally, this multipath connection should ensure greater stability (and thus, a more stable ping) and, by reducing the distance a UDP packet
needs to travel between the game client and the game server, it may also reduce latency. The code provided ensures that only the connections
with the lowest ping are used by leveraging WinDivert and the net-package. 

Currently, the project only works for Windows. 

## Usage
As I don't own any proxy servers (*), a direct executable won't be of any use. The code still needs to be slightly altered to ensure direct 
communications with the proxies. However, it does provide an example on how that may be done, as well as flags which ensure the multipath
configuration. 

```
-cleanup-interval duration
        how long to wait before cleaning the packet cache involved in the deduplicating package process (default 1s)
-dynamic
      enable periodic proxy reselection
-max-connections int
      maximum number of connections for multipath routing (default 2)
-probe-interval duration
      interval at which to probe for down connections (default 10s)
-proxy-listen-addr string
      comma-separated list of proxy listen addresses (e.g. "A:9029,B:9030")
-proxy-ping-listen-addr string
      comma-separated list of proxy ping addresses   (e.g. "A:10001,B:10002")
-server string
      league of legends server. Available servers: NA, LAS, EUW, OCE, EUNE, RU, TR, JP, KR
-threshold-factor float
      exclude connections whose ping exceeds thresholdFactorxthe lowest observed ping. Must be greater than 1.0 (default 1.4)
-timeout duration
      ping response timeout (default 1s)
-update-interval duration
      interval at which to refresh each connection's ping metrics (default 30s)
```

For example,
`lol-multipath.exe -proxy-listen-addr=IP1:PORT1,IP2:PORT2" -proxy-ping-listen-addr="IP1:PORT1X,IP2:PORT2X"`
Take into account that the ping address and ping listen address must have a 1-to-1 correspondence (as seen from the IPs in the example).

## Regarding the Proxy
As mentioned above, I do not own any proxy servers, so the code assumes some characteristics of them.
1. They must have a distinct listener for pings.
2. They must have a distinct listener for incoming packets. Depending on the sender, it will redirect them to their destination (server -> proxy -> client, client -> proxy -> server)
3. They must have a way to receive information about a) Riot's game server port and IP, b) Game Client's listen port and c) IP of the internet interfaces.

(*) All tests were done using my own internet interfaces, so it may not be fully complete.
