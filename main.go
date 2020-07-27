package main //testing

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
import "math/rand"
import "fmt"

var (
	DiscordToken  string
	ApiBase       string
	NotFoundEmoji string
	wg            sync.WaitGroup
)

var Formats = [...]string{"Standard", "Modern", "Legacy", "Vintage", "Draft", "Two-Headed Giant", "Commander", "Archenemy", "Planechase"}

type Results struct {
	Data []Card
}

type Card struct {
	Name string
	Price string `json:"usd"`
	ImageUrl string `json:"image_uri"`
}

func init() {
	DiscordToken = os.Getenv("DISCORD_TOKEN")
	ApiBase = "https://api.scryfall.com/"
	NotFoundEmoji = "👻"

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
	cards, err := cardByString(given)
	
	if err != nil || len(cards) == 0 {
		s.MessageReactionAdd(m.ChannelID, m.ID, NotFoundEmoji)
		log.Printf("[LOOKUP_ERROR]\tname=%s\terror=%s", given, err)
		return
	}

	card := cards[0]

	for _, c := range cards {
		if strings.EqualFold(name, c.Name) {
			card = c
		}
	}

	log.Printf("[FOUND]\tcard=%s\tdistance=%i", card.Name)

	s.UpdateStatus(0, Formats[rand.Intn(len(Formats))])
	res := fmt.Sprintf("%s ($%s)\n%s", card.Name, card.Price, card.ImageUrl)
	s.ChannelMessageSend(m.ChannelID, res)
}

func cardByString(given string) ([]Card, error) {
	parts := strings.Split(given, "/")
	name := parts[0]
	
	requestUrl := ApiBase + "/cards/search?q=++" + url.QueryEscape(name)

	if len(parts) >= 2 {
		requestUrl += url.QueryEscape(" e:" + parts[1])
	}

	log.Printf("%s", requestUrl)
	
	resp, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
	} else if resp.StatusCode == 404 {
		return nil, errors.New("Unable to find a card with `" + name + "`")
	} else if resp.StatusCode != 200 {
		return nil, errors.New("Error communicating with Scryfall")
	}

	var results Results

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	err = json.Unmarshal(buf.Bytes(), &results)

	if err != nil {
		return nil, err
	}
	
	return results.Data, nil
}
