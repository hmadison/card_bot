package main

import "github.com/bwmarrin/discordgo"
import "os"
import "sync"
import "log"
import "regexp"
import "net/http"
import "net/url"
import "bytes"
import "encoding/json"
import "errors"

var (
	DiscordToken string
	ApiBase string
	BotUserId string
	wg           sync.WaitGroup
	botGuilds    []*discordgo.UserGuild
)

type Card struct {
	Name, Cost, Text, Power, Toughness string
}

func init() {
	DiscordToken = os.Getenv("DISCORD_TOKEN")
	ApiBase = "https://api.deckbrew.com/mtg/"
	
	if DiscordToken == "" {
		panic("Discord token missing")
	}
}

func main() {
	dg, err := discordgo.New("Bot " + DiscordToken)
	if err != nil {
		panic(err)
	}

	dg.AddHandler(msgCardByName)
	dg.AddHandler(disconnect)

	user, err := dg.User("@me")
	if err != nil {
		panic(err)
	}

	BotUserId = user.ID
	log.Printf("%+v", user)

	botGuilds, err = dg.UserGuilds()

	wg.Add(1)
	err = dg.Open()
	if err != nil {
		panic(err)
	}

	wg.Wait()
}

func msgCardByName(s *discordgo.Session, m *discordgo.MessageCreate) {
	commandRegexp := regexp.MustCompile(`\[{2}([^\]]+)\]{2}`)
	matches := commandRegexp.FindAllStringSubmatch(m.Content, -1)

	if matches == nil {
		return
	}

	if m.Author.ID == BotUserId {
		return
	}

	for _, match := range matches {
		name := match[1]
		cards, err := cardsByName(name)

		if err != nil {
			return
		}
		
		s.ChannelMessageSend(m.ChannelID, cardToString(cards[0]))
	}
}


func disconnect(s *discordgo.Session, d *discordgo.Disconnect) {
	wg.Done()
}

func cardToString(card Card) (res string) {
	res = "**" + card.Name + "** " + card.Cost
	if card.Power != "" {
		res += " [" + card.Power + "," + card.Toughness + "]"
	}
	res += "\n" + card.Text + "\n"
	res += "<" + "https://magiccards.info/query?q=!" + url.QueryEscape(card.Name) + ">"
	return
}

func cardsByName(name string) ([]Card, error) {
	requestUrl := ApiBase + "cards?name=" + url.QueryEscape(name)
	resp, err := http.Get(requestUrl)

	if err != nil {
		return nil, errors.New("Error communicating with DeckBrew")
	} else if resp.StatusCode == 404 {
		return nil, errors.New("Unable to find a card with `" + name + "`")
	} else if resp.StatusCode != 200 {
		return nil, errors.New("Error communicating with DeckBrew")
	}

	var cards []Card
	
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	err = json.Unmarshal(buf.Bytes(), &cards)

	if err != nil {
		return  nil, errors.New("Error parsing response")
	}
	
	return cards, nil
}
