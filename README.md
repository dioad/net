

# net/http/auth

Create `BasicAuthHandler` and populate with `BasicAuthMap`
```
var handler http.Handler

server := dch.NewServerWithLogger(serverConfig, logger)

authMap := dca.BasicAuthMap{}
authMap.AddUserWithPlainPassword("userA", "passwordA")

server.AddHandler("/status", dca.NewBasicAuthHandler(handler, authMap))

```§
