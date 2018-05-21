package main

import (
	"encoding/json"
	"fmt"
	"github.com/whyrusleeping/hellabot"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//AWH3Q

type (
	Card struct {
		ID        string   `json:"id"`
		CreatedAt string   `json:"created_at"`
		NSFW      string   `json:"nsfw"`
		Text      []string `json:"text"`
	}

	Player struct {
		Nick       string
		Cards      []Card
		Choose     chan Card
		Choices    []Card
		CzarChoice chan ChoicePool

		Points     int
		Save, Czar bool // if true then save hand for next round
	}

	ChoicePool struct {
		Cards []Card
		Nick  string
	}
)

var (
	Players       = make(map[string]*Player)
	RoundPlayers  = make(map[string]*Player)
	PlayerNicks   []string
	CzarChoices   []ChoicePool
	Calls         []Card
	Responses     []Card
	UsedResponses []string
	UsedCalls     []string
	InGame        bool
)

var Triggers = []hbot.Trigger{
	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".help"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			irc.Reply(m, ".adddeck [code] : add deck to card pool")
			irc.Reply(m, ".players : list players currently in game")
			irc.Reply(m, ".join : join the game")
			irc.Reply(m, ".leave : leave the game")
			irc.Reply(m, ".choose [number] : pick a card")
			irc.Reply(m, ".start : start the game")
			irc.Reply(m, ".score : display the score for all users")
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".adddeck"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			opts := strings.Split(m.Content, " ")
			if len(opts) == 2 {
				calls := getCalls(opts[1])
				responses := getResponses(opts[1])
				if len(calls) == 0 || len(responses) == 0 {
					irc.Reply(m, "failed to add deck "+opts[1])
					return false
				}
				Calls = append(Calls, calls...)
				Responses = append(Responses, responses...)
				irc.Reply(m, "Added deck "+opts[1])
			} else {
				irc.Reply(m, "Please specify a CardCast deck code")
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".randcall"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			if len(Calls) != 0 {
				irc.Reply(m, strings.Join(Calls[rand.Int()%len(Calls)].Text, "___"))
			} else {
				irc.Reply(m, "Please add a deck with .adddeck [code]")
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".randresponse"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			if len(Responses) != 0 {
				irc.Reply(m, Responses[rand.Int()%len(Responses)].Text[0])
			} else {
				irc.Reply(m, "Please add a deck with .adddeck [code]")
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".players"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			irc.Reply(m, fmt.Sprintf("%v", PlayerNicks))
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".score"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			for k, v := range Players {
				irc.Reply(m, fmt.Sprintf("%s: %d", k, v.Points))
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".join"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			Players[m.From] = &Player{
				Nick:       m.From,
				Choose:     make(chan Card),
				CzarChoice: make(chan ChoicePool),
			}
			PlayerNicks = append(PlayerNicks, m.From)
			irc.Reply(m, m.From+" has joined the game")
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".leave"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			if _, ok := Players[m.From]; ok {
				delete(Players, m.From)
				for i, v := range PlayerNicks {
					if v == m.From {
						PlayerNicks = append(PlayerNicks[:i], PlayerNicks[i+1:]...)
						break
					}
				}
				irc.Reply(m, m.From+" has left the game")
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".choose"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			opts := strings.Split(m.Content, " ")
			if len(opts) == 2 {
				if _, ok := RoundPlayers[m.From]; ok {
					n, e := strconv.Atoi(opts[1])
					if Players[m.From].Czar {
						if e == nil && n >= 0 && n < len(PlayerNicks) {
							Players[m.From].CzarChoice <- CzarChoices[n]
							//						Players[m.From].Choices = append(Players[m.From].Choices, Players[m.From].Cho)
						}
					} else {
						if e == nil && n >= 0 && n <= 5 {
							Players[m.From].Choices = append(Players[m.From].Choices, Players[m.From].Cards[n])
							Players[m.From].Choose <- Players[m.From].Cards[n]
						} else {
							irc.Notice(m.From, "Please enter a valid option")
						}
					}
				}
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".start"
		},
		start,
	},
	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".stop"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			InGame = false
			return false
		},
	},
}

func start(irc *hbot.Bot, m *hbot.Message) bool {
	if !InGame {
		InGame = true
		for InGame { // game loop
			czar := PlayerNicks[rand.Int()%len(PlayerNicks)]
			RoundPlayers = Players
			RoundPlayers[czar].Czar = true
			irc.Reply(m, czar+" is the czar this round")

			/* random cards */
			Calls = Shuffle(Calls)
			irc.Reply(m, "This rounds black card is: "+strings.Join(Calls[0].Text, "___"))
			Responses = Shuffle(Responses)
			n := 0
			for k := range RoundPlayers {
				RoundPlayers[k].Cards = Responses[n*5 : 5+n*5]
				n++
			}

			for i := 0; i < len(Calls[0].Text)-1; i++ {
				for k := range RoundPlayers {
					if !RoundPlayers[k].Czar {
						var cardpool []string
						i := 0
						for _, v := range RoundPlayers[k].Cards {
							cardpool = append(cardpool, fmt.Sprintf("%d. %s", i, v.Text[0]))
							i++
						}
						irc.Notice(k, "Please choose a(nother) card with .choose [number]")
						irc.Notice(k, strings.Join(cardpool, " | "))
					}
				}
				for k := range RoundPlayers {
					if !RoundPlayers[k].Czar {
						<-RoundPlayers[k].Choose
					}
				}
			}
			// wait for all players to choose a card

			irc.Notice(czar, "Please choose a response with .choose [nunmber] for: "+strings.Join(Calls[0].Text, "___"))
			i := 0
			for k := range RoundPlayers {
				if !RoundPlayers[k].Czar {
					var cardpool []string
					for i := 0; i < len(Calls[0].Text)-1; i++ {
						cardpool = append(cardpool, RoundPlayers[k].Choices[i].Text[0])
					}
					CzarChoices = append(CzarChoices, ChoicePool{
						Cards: RoundPlayers[k].Choices,
						Nick:  RoundPlayers[k].Nick,
					})
					irc.Notice("#cah", fmt.Sprintf("%d. %s", i, strings.Join(cardpool, " | ")))
					i++
				}
			}

			WinningCard := <-RoundPlayers[czar].CzarChoice
			var cardpool []string
			for i := 0; i < len(Calls[0].Text)-1; i++ {
				cardpool = append(cardpool, WinningCard.Cards[i].Text[0])
			}

			irc.Reply(m, WinningCard.Nick+" has won the round with: "+strings.Join(cardpool, "|"))
			RoundPlayers[WinningCard.Nick].Points++

			/* reset player choices */
			for k := range RoundPlayers {
				RoundPlayers[k].Choices = []Card{}
			}
			CzarChoices = []ChoicePool{}
			RoundPlayers[czar].Czar = false
		}
		UsedResponses = []string{}
	} else {
		irc.Reply(m, "Game already in progress!")
	}
	return false
}

func Shuffle(vals []Card) []Card {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]Card, len(vals))
	n := len(vals)
	for i := 0; i < n; i++ {
		randIndex := r.Intn(len(vals))
		ret[i] = vals[randIndex]
		vals = append(vals[:randIndex], vals[randIndex+1:]...)
	}
	return ret
}

func getCalls(id string) (calls []Card) {
	resp, e := http.Get("https://api.cardcastgame.com/v1/decks/" + id + "/calls")
	if e != nil {
		return
	}
	dec := json.NewDecoder(resp.Body)
	dec.Decode(&calls)
	return
}

func getResponses(id string) (responses []Card) {
	resp, e := http.Get("https://api.cardcastgame.com/v1/decks/" + id + "/responses")
	if e != nil {
		return
	}
	dec := json.NewDecoder(resp.Body)
	dec.Decode(&responses)
	return
}

func main() {
	rand.Seed(time.Now().UnixNano())

	channels := func(bot *hbot.Bot) {
		bot.Channels = []string{"#cah"}
	}
	irc, err := hbot.NewBot("irc.rekt.network:6667", "cahbot", func(bot *hbot.Bot) {}, channels)
	if err != nil {
		panic(err)
	}

	for _, v := range Triggers {
		irc.AddTrigger(v)
	}

	irc.Run()

	fmt.Printf("%v\n", getCalls("AWH3Q")[0])
	fmt.Printf("%v\n", getResponses("AWH3Q")[0])

}
