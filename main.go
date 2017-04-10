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
import "fmt"
import "github.com/arbovm/levenshtein"

var (
	DiscordToken  string
	ApiBase       string
	NotFoundEmoji string
	wg            sync.WaitGroup
)

type Card struct {
	Name, Cost, Text, Power, Toughness string
	Types []string
	Editions                           []struct {
		SetId        string `json:"set_id"`
		Number       string
		MultiverseId int `json:"multiverse_id"`
		Layout       string
	}
}

func init() {
	DiscordToken = os.Getenv("DISCORD_TOKEN")
	ApiBase = "https://api.deckbrew.com/mtg/"
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

func sendCardMessage(s *discordgo.Session, m *discordgo.MessageCreate, name string) {
	cards, err := cardsByName(name)

	if err != nil || len(cards) == 0 {
		s.MessageReactionAdd(m.ChannelID, m.ID, NotFoundEmoji)
		log.Printf("[LOOKUP_ERROR]\tname=%s\terror=%s", name, err)
		return
	}

	var card Card
	card = cards[0]

	for i, c := range cards {
		if levenshtein.Distance(c.Name, name) < levenshtein.Distance(card.Name, name) {
			card = cards[i]
		}
	}

	s.ChannelMessageSend(m.ChannelID, cardToString(card))
}

func cardToString(card Card) (res string) {
	edition := card.Editions[0]

	res = "**" + card.Name + "** " + formatMana(card.Cost)

	if card.Power != "" {
		res += " [" + card.Power + "/" + card.Toughness + "]"
	}

	res += "\n" + formatTypes(card.Types)

	res += "\n" + formatMana(card.Text) + "\n"

	if edition.Layout == "split" {
		res += "<" + "http://magiccards.info/" + strings.ToLower(edition.SetId) + "/en/" + edition.Number + ".html>"
	} else {
		res += "<" + "http://magiccards.info/query?q=!" + url.QueryEscape(card.Name) + ">"
	}

	return
}

func formatTypes(inputs []string) (res string) {
	res += "*"
	res += strings.Join(inputs, " ")
	res += "*"
	res = strings.Title(res)
	return
}

func formatMana(input string) string {
	src := []byte(input)
	quote := regexp.MustCompile(`\{(.+)\}`)
	space := regexp.MustCompile(`}{`)

	src = quote.ReplaceAllFunc(src, func(s []byte) []byte {
		return []byte(fmt.Sprintf("`%s`", string(s)))
	})

	src = space.ReplaceAllLiteral(src, []byte("} {"))
	return string(src)
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
