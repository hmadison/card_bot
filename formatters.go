package main

import "github.com/bwmarrin/discordgo"
import "strings"
import "net/url"
import "regexp"
import "fmt"

type cardFormatter interface {
	Respond(string, Card, *discordgo.Session, *discordgo.MessageCreate)
}

type TextFormatter struct{}
type ImageFormatter struct{}

func (t TextFormatter) Respond(g string, c Card, s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, cardToString(c))
	return
}

func (i ImageFormatter) Respond(g string, c Card, s *discordgo.Session, m *discordgo.MessageCreate) {
	parts := strings.Split(g, "/")
	edition := c.Editions[0]

	for _, e := range c.Editions {
		if e.MultiverseId > edition.MultiverseId {
			edition = e
		}
	}

	if len(parts) > 1 {
		for _, e := range c.Editions {
			uS := strings.ToUpper(parts[1])
			if (e.SetId == uS || e.Set == uS) && e.MultiverseId != 0 {
				edition = e
			}
		}
	}

	s.ChannelMessageSend(m.ChannelID, edition.ImageUrl)
	return
}

func cardToString(card Card) (res string) {
	edition := card.Editions[0]

	res = "**" + card.Name + "** " + formatMana(card.Cost)

	if card.Power != "" {
		res += " [" + card.Power + "/" + card.Toughness + "]"
	}

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
