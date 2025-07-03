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
The code uses WinDivert to intercept incoming packets; thus, admin priviledges are needed.
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
      (required) comma-separated list of proxy listen addresses (e.g. "A:9029,B:9030")
-proxy-ping-listen-addr string
      (required) comma-separated list of proxy ping addresses   (e.g. "A:10001,B:10002")
-server string
      (required) league of legends server. Available servers: NA, LAS, EUW, OCE, EUNE, RU, TR, JP, KR
-threshold-factor float
      exclude connections whose ping exceeds thresholdFactorxthe lowest observed ping. Must be greater than 1.0 (default 1.4)
-timeout duration
      ping response timeout (default 1s)
-update-interval duration
      interval at which to refresh each connection's ping metrics (default 30s)
```

For example,
`lol-multipath.exe -proxy-listen-addr=IP1:PORT1,IP2:PORT2" -proxy-ping-listen-addr="IP1:PORT1X,IP2:PORT2X" -server "NA" -dynamic`
Take into account that the ping address and ping listen address must have a 1-to-1 correspondence (as seen from the IPs in the example).


## Regarding the Proxy
As mentioned above, I do not own any proxy servers, so the code assumes some characteristics of them.
1. They must have a distinct listener for pings.
2. They must have a distinct listener for incoming packets. Depending on the sender, it will redirect them to their destination (server -> proxy -> client, client -> proxy -> server)
3. They must have a way to receive information about a) Riot's game server port and IP, b) Game Client's listen port and c) IP of the internet interfaces.

If you just want to test you may readily use the code as it is and use your own interfaces' IP in both `-proxy-listen-addr` and `-proxy-ping-listen-addr`. However, please note that
you won't see any improvement or even may find no in-game response due to if the "proxy" is located at an interface with a bigger metric (i.e not the preferred pathway). For example,
in my case, if my "proxy" is my WiFi the game won't reflect any changes and will even be unresponsive as the preferred interface is Ethernet.


## How it Works
First, it checks for every internet interface available (e.g WiFi, Ethernet) within the PC the code is running into and filters for those that are not virtual interfaces or loopback. 
Then, it creates a connection for every pair (interface, proxy listen address) and (interface, proxy ping listen address). After that, it selects at most `max-connections` pairs with
the lowest ping through a pinging process. This process consists of the following steps:
1. A timer is started. The program pings the proxy through its proxy listen address.
2. The proxy receives the ping and automatically pings the LoL `-server` through an HTTP Request (Many thanks to [LoL Ping Test](https://pingtestlive.com/league-of-legends)).
3. The program calculates the time it takes to make the TSP handshake + DNS Resolution + ...; or the time that is not the sending of the packet itself (I call it the `bloat`).
4. Once the response is received, the timer is stopped. The "expected ping" is a measure of all the time taken minus the `bloat`.

If `-dynamic` is on, this reselection process is repeated every `-update-interval`. However, the filtering only occurs once. After that, the proxy is in charge of redirecting the incoming
packets to the server or game client depending on the sender. 

## Known Limitations
1. The program handles down connections by probing them every `-probe-interval`. If there is a response, it is re-added to the available connections. However, this was not throughly tested
   as I have no proxy servers.
2. The program automatically notices when the game is running and starts all the multipath logic; however, it does not detect when the game ends so it needs to be manually restarted.
   Attempts to make this process automatic have been made (see commented `CheckIfLeagueIsActive()` section in `main.go`), but the performance was deplorable.
3. The way it finds for the league process is by executing Powershell commands every 5 seconds (can be changed in `WaitForLeagueAndResolve(ctx, 5*time.Second)`), but the process name is
   hardcoded to "League of Legends" in `leagueProcessName`. This shouldn't be a problem unless, for some ungodly reason, you have changed the name of the process or your OS uses a non-romanic
   alphabet (I am sorry).
4. This code **may** be upgraded for other games that use UDP as their protocol to send information. For that, only `Generate Server Map` and `leagueProcessName` would need to be changed.
   I don't play any other games so it is inconvenient to test it, feel free to do it though.
   

## Additional Notes
This section is just me sharing my insights into the process that, while may have been obvious at first, took me some effort to grasp. 
Firstly, all connections use UDP protocol. Second, each big Riot server (i.e "NA", "LAN") is subdivided into many "shards", or mini-servers that handle the incoming packets. 
An initial idea would be to find the addresses of those "shards", which will indeed make things faster, but I wasn't able to do it (and don't think it is possible unless you are a Riot employee).
If I remmeber something I will add it here.

(*) All tests were done using my own internet interfaces, so it may not be fully complete.
