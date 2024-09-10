package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/bstchow/go-chess-server/pkg/corenet"
	"github.com/bstchow/go-chess-server/pkg/session"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
	"github.com/rivo/tview"
)

type matchResponse struct {
	Type        string              `json:"type"`
	SessionID   string              `json:"session_id"`
	GameState   string              `json:"game_state"`
	PlayerState session.PlayerState `json:"player_state"`
}

type User struct {
	ID       string `json:"id"`
	JWTToken string `json:"jwt_token"`
}

type PrivyLoginRequest struct {
	PrivyJWTToken string `json:"privy_jwt_token"`
}

// TODO: Should be imported from the server
// Should have a pkg declaring expected output of API endpoints
type Session struct {
	SessionID string   `json:"session_id"`
	Player1ID string   `json:"player1_id"`
	Player2ID string   `json:"player2_id"`
	Moves     []string `json:"moves"`
}

var (
	app         *tview.Application
	loginForm   *tview.Form
	currentUser *User
	gameResult  string
	gameMessage string
)

func main() {
	app = tview.NewApplication()

	setupForms(app)

	app.SetRoot(mainMenu(), true).Run()

}

func setupForms(app *tview.Application) {
	loginForm = tview.NewForm().
		AddInputField("Privy JWT Token", "", 20, nil, nil).
		AddButton("Login", func() {
			privyJwtToken := loginForm.GetFormItemByLabel("Privy JWT Token").(*tview.InputField).GetText()
			login(PrivyLoginRequest{privyJwtToken})
		}).
		AddButton("Back", func() {
			app.SetRoot(mainMenu(), true).Run()
		})
}

func mainMenu() *tview.Flex {
	headerBox := tview.NewBox().
		SetBorder(true).
		SetTitle("Go Chess Server").
		SetTitleAlign(tview.AlignLeft)

	headerText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	updateHeader := func() {
		if currentUser == nil {
			headerText.SetText("Please login to the server")
		} else {
			headerText.SetText(fmt.Sprintf("User: %s", currentUser.ID))
		}
	}

	updateHeader()

	menu := tview.NewList().
		AddItem("Login", "Login to your account", '1', func() {
			app.SetRoot(loginForm, true).Run()
		}).
		AddItem("Quit", "Exit the application", '3', func() {
			app.Stop()
			clearScreen()
			os.Exit(0)
		})

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(headerBox, 3, 1, false).
		AddItem(headerText, 1, 1, false).
		AddItem(menu, 0, 1, true)

	return flex
}

func postLoginMenu() *tview.Flex {
	headerBox := tview.NewBox().
		SetBorder(true).
		SetTitle("Go Chess Server").
		SetTitleAlign(tview.AlignLeft)

	headerText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	updateHeader := func() {
		if currentUser == nil {
			headerText.SetText("Please login to the server")
		} else {
			headerText.SetText(fmt.Sprintf("User: %s", currentUser.ID))
		}
	}

	updateHeader()

	menu := tview.NewList().
		AddItem("Join match", "Join a new match", '1', func() {
			app.Stop()
			app = tview.NewApplication()
			gameResult = ""
			gameMessage = ""
			joinMatch()
			if gameResult == "timeout" {
				showMatchingErrorDialog("Matching timeout" + gameMessage)
			} else if gameResult == "queueing" {
				showMatchingErrorDialog("You are queueing elsewhere" + gameMessage)
			} else if gameResult == "error" {
				showMatchingErrorDialog("You are playing elsewhere" + gameMessage)
			} else {
				showLoginSuccessDialog("Game ended with " + gameResult + gameMessage)
			}
		}).
		AddItem("Logout", "Logout from your account", '3', func() {
			currentUser = nil
			app.SetRoot(mainMenu(), true).Run()
		})

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(headerBox, 3, 1, false).
		AddItem(headerText, 1, 1, false).
		AddItem(menu, 0, 1, true)

	return flex
}

func showLoginErrorDialog(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(loginForm, true).Run()
		})
	app.SetRoot(modal, true).Run()
}

func showMatchingErrorDialog(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(postLoginMenu(), true).Run()
		})
	app.SetRoot(modal, true).Run()
}

func showLoginSuccessDialog(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(postLoginMenu(), true).Run()
		})
	app.SetRoot(modal, true).Run()
}

func login(loginRequest PrivyLoginRequest) {
	url := "http://localhost:7202/api/privyLogin"
	userJSON, err := json.Marshal(loginRequest)
	if err != nil {
		showLoginErrorDialog(fmt.Sprintf("Error marshalling login request: %v", err))
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(userJSON))
	if err != nil {
		showLoginErrorDialog(fmt.Sprintf("Error making POST request: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		showLoginErrorDialog(fmt.Sprintf("Login failed: %s", body))
		return
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		showLoginErrorDialog(fmt.Sprintf("Error decoding response: %v", err))
		return
	}

	id, ok := result["user_id"].(string)
	if !ok {
		showLoginErrorDialog("Player ID not found in response")
		return
	}

	jwtToken, ok := result["jwt_token"].(string)
	if !ok {
		showLoginErrorDialog("JWT token not found in response")
		return
	}

	currentUser = &User{ID: id, JWTToken: jwtToken}

	showLoginSuccessDialog("Login successful!")
}

func joinMatch() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:7201", Path: "/ws"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	log.Println("Connected to game server")

	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := c.WriteJSON(corenet.Message{
			Action: "matching",
			Data: map[string]interface{}{
				"jwt_token": currentUser.JWTToken,
			},
		}); err != nil {
			log.Fatal("ws write", err)
		}

		clearScreen()
		log.Println("Attempt matchmaking...")

		var resp matchResponse
		if err := c.ReadJSON(&resp); err != nil {
			log.Fatal("ws match:", err)
			return
		}

		if resp.Type != "matched" {

			gameResult = resp.Type
			return
		}

		sessionResp := session.SessionResponse{
			Type:      "session",
			GameState: resp.GameState,
		}
		var state string
		scanner := bufio.NewScanner(os.Stdin)
		for {
			if sessionResp.Type == "session" {
				state = sessionResp.GameState
			}
			clearScreen()
			chessGameFenUpdate, err := chess.FEN(state)
			if err != nil {
				log.Fatal(err)
			}
			chessGame := chess.NewGame(chessGameFenUpdate)
			fmt.Println(chessGame.Position().Board().Draw())
			if chessGame.Outcome() != chess.NoOutcome {
				gameResult = chessGame.Outcome().String()
				return
			}
			if resp.PlayerState.IsWhiteSide == (chessGame.Position().Turn() == chess.White) {
				if sessionResp.Type == "session" {
					fmt.Print("Enter your move (e.g., e2-e4): ")
				} else {
					fmt.Print("[Invalid] Enter new move (e.g., e2-e4):")
				}
				scanner.Scan()
				move := scanner.Text()

				c.WriteJSON(corenet.Message{
					Action: "move",
					Data: map[string]interface{}{
						"session_id": resp.SessionID,
						"jwt_token":  currentUser.JWTToken,
						"move":       move,
					},
				})

				if err := c.ReadJSON(&sessionResp); err != nil {
					log.Fatal(err)
				}
			} else {
				fmt.Print("Wait for your opponent...")
				if err := c.ReadJSON(&sessionResp); err != nil {
					log.Fatal(err)
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			os.Exit(1)
		}
	}
}

func clearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin", "linux":
		cmd = exec.Command("clear")
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		fmt.Println("Unsupported OS")
		return
	}

	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error clearing screen: %v\n", err)
	}
}
