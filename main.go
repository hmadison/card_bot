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
import "math/rand"
import "fmt"

var (
	DiscordToken  string
	ApiBase       string
	NotFoundEmoji string
	wg            sync.WaitGroup
)

var Formats = [...]string{"Standard", "Modern", "Legacy", "Vintage", "Limited", "Two-Headed Giant", "Commander", "Archenemy", "Planechase"}

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
	card, err := cardByName(name)

	if err != nil {
		s.MessageReactionAdd(m.ChannelID, m.ID, NotFoundEmoji)
		log.Printf("[LOOKUP_ERROR]\tname=%s\terror=%s", name, err)
		return
	}

	log.Printf("[FOUND]\tcard=%s\tdistance=%i", card.Name)

	s.UpdateStatus(0, Formats[rand.Intn(len(Formats))])
	res := fmt.Sprintf("%s ($%s)\n%s", card.Name, card.Price, card.ImageUrl)
	s.ChannelMessageSend(m.ChannelID, res)
}

func cardByName(name string) (card Card, err error) {
	requestUrl := ApiBase + "cards/named?fuzzy=" + url.QueryEscape(name)
	resp, err := http.Get(requestUrl)
	
	if err != nil {
		return
	} else if resp.StatusCode == 404 {
		err = errors.New("Unable to find a card with `" + name + "`")
		return
	} else if resp.StatusCode != 200 {
		err = errors.New("Error communicating with Scryfall")
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	err = json.Unmarshal(buf.Bytes(), &card)

	if err != nil {
		return card, err
	}

	return card, nil
}
