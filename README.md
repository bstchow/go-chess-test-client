# Go Chess Test Client

This was originally part of Minh Quang Bui's go-chess-server (https://github.com/yelaco/go-chess-server)[https://github.com/yelaco/go-chess-server]

Split out to its own module

## How to run

### Client

For the client side, I made a CLI for testing purpose. After the server and database are up and running, you can run the ```client.go``` file in the ```test``` folder
```console
$ cd test
$ go run test.go
```

Here is a video that showcase all the functionalities of the CLI

[client.webm](https://github.com/yelaco/go-chess-server/assets/100106895/d88f310b-b207-459a-b139-d11a64390293)

### Server

## API

### REST

 [References](https://documenter.getpostman.com/view/30874401/2sA3duEsiX)
 
- ```POST /api/users```: To register user
- ```POST /api/login```: To log in to the server
- ```GET /api/sessions```: Retrieve match records played by user
- ```GET /api/sessions/{sessionid}```: Retrieve single match record based on ID

### WebSocket

After login, user can now join a match by sending matching request
```json
{
    "action": "matching",
    "data": {
        "playerId": "12345"
    }
}
```

If the ```action``` and ```data``` is valid, server pushes that user into the matching queue. When a match happens, the two connections are forwarded to game management module, where a game instance will be initialized and binded with the player pair. Then, a message is sent back to the user to notify about the match.
```json
{
    "type": "matched",
    "session_id": "1232524",
    "game_state": {
        "status": "ACTIVE",
        "board": [[]]
        "is_white_turn": true,
    },
    "player_state": {
        "is_white_side": true
    }
}
```

On the contrary, if there are any errors in the process or the matching request is timeout, the server replies with
- Error (Note that this error json is universal for all the error response to users)
```json
{
    "type": "error",
    "error": "<err_msg>",
}
```

- Timeout
```json
{
    "type": "timeout",
    "error": "<err_msg>",
}
```

In a match, users can send move request with 
```json
{
    "action": "move",
    "data": {
        "playerId": "12345",
        "sessionId": "1719199808062498696",
        "move": "e2-e4"
    }
}
```

And get resonses as 
```json
{
    "type": "session",
    "game_state": {
        "status": "STALEMATE",
        "board": [[]]
        "is_white_turn": false,
    }
}
```

After the game reaches end state, the server notifies both players and close their connections.
