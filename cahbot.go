package main

import (
	"bufio"
	"fmt"
	"github.com/whyrusleeping/hellabot"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

//AWH3Q

type (
	Player struct {
		Nick       string
		Hand       []string
		Choice     chan []string
		HasChosen  bool
		CzarChoice chan int

		Points     int
		Save, Czar bool // if true then save hand for next round
	}

	ChoicePool struct {
		Cards []string
		Nick  string
	}
)

var (
	Players       = make(map[string]*Player)
	RoundPlayers  = make(map[string]*Player)
	GameMsg       *hbot.Message
	CardCzar      *Player
	RespNum       int
	PlayerNicks   []string
	CzarChoices   []ChoicePool
	CanCzarChoose bool
	Calls         []string
	Responses     []string
	UsedResponses []string
	UsedCalls     []string
	InGame        bool
	RPos          = 0
	CPos          = 0
)

var Triggers = []hbot.Trigger{
	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".help"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			irc.Reply(m, "\x02\x0304.join\x0F : join the game")
			irc.Reply(m, "\x02\x0304.leave\x0F : leave the game")
			irc.Reply(m, "\x02\x0304.start \x0302[maxscore]\x0F : start the game")
			irc.Reply(m, "\x02\x0304.stop\x0F : stop the game")
			irc.Reply(m, "\x02\x0304.score\x0F : display the score for all users")
			irc.Reply(m, "\x02\x0304.choose \x0302[number]\x0F : pick a card")
			irc.Reply(m, "\x02\x0304.players\x0F : list players currently in game")
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".randcall"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			irc.Reply(m, Calls[rand.Int()%len(Calls)])
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".randresponse"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			irc.Reply(m, Responses[rand.Int()%len(Responses)])
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
				irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F: \x02\x0302%d\x0F", k, v.Points))
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
				Choice:     make(chan []string),
				CzarChoice: make(chan int),
				Points:     0,
			}
			PlayerNicks = append(PlayerNicks, m.From)
			irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F has joined the game", m.From))
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
				irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F has left the game", m.From))
			}
			return false
		},
	},

	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".choose"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			opts := strings.Fields(m.Content)
			if len(opts) > 1 {
				if _, ok := Players[m.From]; ok {
					if CardCzar.Nick == m.From && CanCzarChoose {
						i, err := strconv.Atoi(opts[1])
						if err != nil || i > RespNum {
							irc.Reply(m, "Invalid Number")
							return false
						}
						i++
						irc.Reply(GameMsg, m.From+" has chosen their card")
						CardCzar.CzarChoice <- i
					} else if !RoundPlayers[m.From].HasChosen {
						// TODO choose a card
						var slice []string
						for k := range opts[1:] {
							i, err := strconv.Atoi(opts[k+1])
							if err != nil || i > 5 {
								irc.Msg(m.From, "Invalid Number")
								return false
							}
							slice = append(slice, RoundPlayers[m.From].Hand[i])

							Players[m.From].Hand = append(Players[m.From].Hand[:i], Players[m.From].Hand[i+1:]...)
							//add new cards to hand
							Players[m.From].Hand = append(Players[m.From].Hand, Responses[RPos])
							RPos++
							//delete(Players[m.From].Hand, i)
						}
						RoundPlayers[m.From].Choice <- slice
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
		func(bot *hbot.Bot, m *hbot.Message) bool {
			opts := strings.Fields(m.Content)
			if len(opts) == 2 {
				if len(Players) > 1 {
					i, err := strconv.Atoi(opts[1])
					if err != nil {
						bot.Reply(m, "Invalid Number")
						return false
					}
					start(bot, m, i)
				}
			} else {
				bot.Reply(m, "missing maxscore")
			}
			return false
		},
	},
	hbot.Trigger{
		func(bot *hbot.Bot, m *hbot.Message) bool {
			return strings.Split(m.Content, " ")[0] == ".stop"
		},
		func(irc *hbot.Bot, m *hbot.Message) bool {
			InGame = false
			RoundPlayers = make(map[string]*Player)
			Players = make(map[string]*Player)

			irc.Reply(m, fmt.Sprintf("game ended"))
			return false
		},
	},
}

func waitForCards(p map[string]*Player) []ChoicePool {
	var t []ChoicePool
	i := 0
	for k := range p {
		t = append(t, ChoicePool{Cards: <-p[k].Choice, Nick: k})
		i++
	}
	return t
}

func start(irc *hbot.Bot, m *hbot.Message, max int) bool {
	if !InGame {
		GameMsg = m
		InGame = true
		Calls = Shuffle(Calls)
		Responses = Shuffle(Responses)
		for k := range Players {
			Players[k].Hand = Responses[RPos*5 : 5+RPos*5]
			RPos = RPos + 5
		}
		for InGame { // game loop
			for k := range Players {
				if Players[k].Points == max {
					irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F has won the game!", Players[k].Nick))
					for k, v := range Players {
						irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F: \x02\x0302%d\x0F", k, v.Points))
					}

					InGame = false
					RoundPlayers = make(map[string]*Player)
					Players = make(map[string]*Player)
					return false
				}
			}
			c := rand.Intn(len(PlayerNicks))
			CardCzar = Players[PlayerNicks[c]]
			CardCzar.Czar = true

			for k, v := range Players {
				if !Players[k].Czar {
					RoundPlayers[k] = v
				}
			}
			irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F is the Card Czar this round!", CardCzar.Nick))

			irc.Reply(m, "\x02\x0300,01 Black Card \x0F : "+Calls[CPos])
			RespNum = strings.Count(Calls[CPos], "_")
			if RespNum == 0 {
				RespNum++
			}
			for k := range RoundPlayers {
				irc.Msg(RoundPlayers[k].Nick, "\x02\x0300,01 Black Card \x0F : "+Calls[CPos])
				irc.Msg(RoundPlayers[k].Nick, fmt.Sprintf("Please choose (\x02%d\x0F) \x02\x0301,00 White Card(s) \x0F with \x02\x0304.choose \x0302[int] [int]\x0399 ...\x0F", RespNum))
				i := 0
				for x := range RoundPlayers[k].Hand {
					irc.Msg(RoundPlayers[k].Nick, fmt.Sprintf("[\x02\x0302%d\x0F] %s", i, RoundPlayers[k].Hand[x]))
					i++
				}
			}

			czarChoices := waitForCards(RoundPlayers) // map[string][]string
			irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F, please choose the winning \x02\x0301,00 White Card \x0F combination", CardCzar.Nick))
			i := 0
			for k := range czarChoices {
				irc.Reply(m, fmt.Sprintf("[\x02\x0302%d\x0F] %s", i, strings.Join(czarChoices[k].Cards, " | ")))
				i++
			}
			CanCzarChoose = true

			r := <-CardCzar.CzarChoice
			WinningCard := czarChoices[r-1]

			irc.Reply(m, fmt.Sprintf("\x02\x0304%s\x0F has won the round with: %v", WinningCard.Nick, WinningCard.Cards))
			Players[WinningCard.Nick].Points++

			for k := range RoundPlayers {
				RoundPlayers[k].HasChosen = false
			}
			CardCzar.Czar = false
			CanCzarChoose = false
			RoundPlayers = make(map[string]*Player)
			CPos++
		}
		UsedResponses = []string{}
	} else {
		irc.Reply(m, "\x0304Game already in progress!")
	}
	return false
}

func Shuffle(vals []string) []string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]string, len(vals))
	n := len(vals)
	for i := 0; i < n; i++ {
		randIndex := r.Intn(len(vals))
		ret[i] = vals[randIndex]
		vals = append(vals[:randIndex], vals[randIndex+1:]...)
	}
	return ret
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	var err error
	Calls, err = readLines("black.txt")
	if err != nil {
		panic(err)
	}
	Responses, err = readLines("white.txt")
	if err != nil {
		panic(err)
	}

	channels := func(bot *hbot.Bot) {
		bot.Channels = []string{"#rekt"}
	}
	irc, err := hbot.NewBot("irc.rekt.network:6667", "cahbot", func(bot *hbot.Bot) {}, channels)
	if err != nil {
		panic(err)
	}

	for _, v := range Triggers {
		irc.AddTrigger(v)
	}

	irc.Run()

}
