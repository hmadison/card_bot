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
import "github.com/arbovm/levenshtein"

var (
	DiscordToken string
	ApiBase      string
	BotUserId    string
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

	dg.AddHandler(msgDirectCardByName)
	dg.AddHandler(msgInlineCardByName)
	dg.AddHandler(disconnect)

	user, err := dg.User("@me")
	if err != nil {
		panic(err)
	}

	BotUserId = user.ID
	log.Printf("Connected as %s (%s)", user.Username, BotUserId)

	botGuilds, err = dg.UserGuilds()

	wg.Add(1)
	err = dg.Open()
	if err != nil {
		panic(err)
	}

	wg.Wait()
}

func msgDirectCardByName(s *discordgo.Session, m *discordgo.MessageCreate) {
	commandRegexp := regexp.MustCompile(`\!(.+)$`)
	matches := commandRegexp.FindStringSubmatch(m.Content)

	if matches == nil {
		return
	}

	if m.Author.ID == BotUserId {
		return
	}

	name := matches[1]
	sendCardMessage(s, name, m.ChannelID)
}

func msgInlineCardByName(s *discordgo.Session, m *discordgo.MessageCreate) {
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
		sendCardMessage(s, name, m.ChannelID)
	}
}

func disconnect(s *discordgo.Session, d *discordgo.Disconnect) {
	log.Printf("Disconnected from Discord.")
	wg.Done()
}

func sendCardMessage(s *discordgo.Session, name string, channelID string) {
	cards, err := cardsByName(name)

	if err != nil {
		log.Printf("[LOOKUP_ERROR] %s %s", name, err)
		return
	}

	if len(cards) == 0 {
		return
	}

	var card Card
	card = cards[0]

	for i, c := range cards {
		if levenshtein.Distance(c.Name, name) < levenshtein.Distance(card.Name, name) {
			card = cards[i]
		}
	}

	s.ChannelMessageSend(channelID, cardToString(card))
}

func cardToString(card Card) (res string) {
	res = "**" + card.Name + "** " + card.Cost
	if card.Power != "" {
		res += " [" + card.Power + "," + card.Toughness + "]"
	}
	res += "\n" + card.Text + "\n"
	res += "<" + "http://magiccards.info/query?q=!" + url.QueryEscape(card.Name) + ">"
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
		return nil, errors.New("Error parsing response")
	}

	return cards, nil
}
