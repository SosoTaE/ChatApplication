package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"encoding/json"
)


type user struct {
	Name string `json:"name"`
}

// var users []user


// func remove(array []user, name string) []user {
// 	var updatedArray []user
// 	for i := 0; i < len(array);i++ {
// 		if array[i].Name == name {
// 			continue
// 		}

// 		updatedArray = append(updatedArray,array[i])
// 	}

// 	return updatedArray
// }


func headerMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Next()
}

type message struct {
	User string `json:"user"`
	Message string `json:"message"`
}

var connectionCounter uint64

func generateConnectionID() string {
	id := atomic.AddUint64(&connectionCounter, 1)
	return "conn" + strconv.FormatUint(id, 10)
}


type client struct {
	name string
	conn websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin
		return true
	},
}

var clients = make(map[*client]bool)
var broadcast = make(chan []byte)

func handleWebSocket(w gin.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}

	var clientName = make(map[string]string)

	err = conn.ReadJSON(&clientName);
	if err != nil {
		panic(err)
	}

	fmt.Println(clientName["newUser"])

	client := &client{
		name:  clientName["newUser"],
		conn: *conn,
		send: make(chan []byte),
	}
	clients[client] = true

	data, err := json.Marshal(clientName)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(data))

	broadcast <- data

	go client.readPump()
	go client.writePump()
}

func (c *client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			fmt.Println(string(message))
			if !ok {
				return
			}
			fmt.Println(message)
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("Error writing message:", err)
				return
			}
		}
	}
}

func (c *client) readPump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Println("Error reading message:", err)
			}
			break
		}

		broadcast <- message
	}
}

func broadcastMessages() {
	for {
		message := <-broadcast
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(clients, client)
			}
		}
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(headerMiddleware)

	// r.Static("/", "./static")

	r.POST("/newuser", func(c *gin.Context) {
		var person user
		err := c.ShouldBindJSON(&person)
		if err != nil {
			panic(err)
		}


		for each, _ := range clients {
			fmt.Println("client name:",each.name)
			if each.name == person.Name {
				fmt.Println("user named " + person.Name + " already exists")
				c.JSON(403, gin.H{
					"message": "user named " + person.Name + " already exists",
				})
			
				return
			}
		}
		// users = append(users, person)
 		fmt.Println(person)
		c.JSON(202, gin.H{
			"message": "user is added",
		})
	})

	r.POST("/user/exit", func(ctx *gin.Context) {
		var person user
		ctx.ShouldBind(person)
		for each, _ := range clients {
			if each.name == person.Name {
				ctx.JSON(403, gin.H{
					"message": "user named " + person.Name + " already exists",
				})

				each.conn.Close()
			    delete(clients, each)
			
				return
			}
		}
		
		ctx.JSON(202,gin.H{
			"meesage": "user is deleted",
		})
	
	})

	r.GET("/ws", func (c * gin.Context) {
		go handleWebSocket(c.Writer, c.Request)
	})

	r.Static("/chat", "./build")

	go broadcastMessages()
	r.Run();
}