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
import "strings"
import "github.com/xrash/smetrics"

var (
	DiscordToken  string
	ApiBase       string
	NotFoundEmoji string
	Formatter     cardFormatter
	wg            sync.WaitGroup
)

type Card struct {
	Name, Cost, Text, Power, Toughness string
	Types                              []string
	Editions                           []struct {
		Set          string
		SetId        string `json:"set_id"`
		Number       string
		MultiverseId int    `json:"multiverse_id"`
		ImageUrl     string `json:"image_url"`
		Layout       string
	}
}

func init() {
	DiscordToken = os.Getenv("DISCORD_TOKEN")
	ApiBase = "https://api.deckbrew.com/mtg/"
	NotFoundEmoji = "👻"
	Formatter = ImageFormatter{}

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

	log.Printf("Connected as %s", user.Username)

	wg.Add(1)
	err = dg.Open()
	if err != nil {
		panic(err)
	}

	wg.Wait()
}

func msgDirectCardByName(s *discordgo.Session, m *discordgo.MessageCreate) {
	commandRegexp := regexp.MustCompile(`^\!(.+)$`)
	matches := commandRegexp.FindStringSubmatch(m.Content)

	if matches == nil || m.Author.Bot {
		return
	}

	name := matches[1]
	sendCardMessage(s, m, name)
}

func msgInlineCardByName(s *discordgo.Session, m *discordgo.MessageCreate) {
	commandRegexp := regexp.MustCompile(`\[{2}([^\]]+)\]{2}`)
	matches := commandRegexp.FindAllStringSubmatch(m.Content, -1)

	if matches == nil || m.Author.Bot {
		return
	}

	for _, match := range matches {
		name := match[1]
		sendCardMessage(s, m, name)
	}
}

func disconnect(s *discordgo.Session, d *discordgo.Disconnect) {
	wg.Done()
}

func sendCardMessage(s *discordgo.Session, m *discordgo.MessageCreate, given string) {
	name := strings.Split(given, "/")[0]
	name = strings.ToUpper(name)
	cards, err := cardsByName(name)

	if err != nil || len(cards) == 0 {
		s.MessageReactionAdd(m.ChannelID, m.ID, NotFoundEmoji)
		log.Printf("[LOOKUP_ERROR]\tname=%s\terror=%s", name, err)
		return
	}

	var card Card
	card = cards[0]
	currentBestDistance := 256

	for _, c := range cards {
		upperTest := strings.ToUpper(c.Name)
		testBestDistance := smetrics.WagnerFischer(name, upperTest, 1, 1, 2)

		if testBestDistance < currentBestDistance {
			card = c
			currentBestDistance = testBestDistance
		}
	}

	log.Printf("[FOUND]\tcard=%s\tdistance=%i", card.Name, currentBestDistance)

	Formatter.Respond(given, card, s, m)
}

func cardsByName(name string) ([]Card, error) {
	requestUrl := ApiBase + "cards?name=" + url.QueryEscape(name)
	resp, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
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
		return nil, err
	}

	return cards, nil
}
