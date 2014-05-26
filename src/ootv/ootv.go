package ootv

import (
	"errors"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type DeckType int

const (
	Dynasty DeckType = iota
	Fate
	StartsInPlay
	Token
)

type DeckItem struct {
	Count    int
	CardData Card
}

type Card struct {
	ID                  int
	CardNumber          int
	Title               string
	Type                string
	Deck                DeckType
	Keywords            []string
	CardText            string
	GoldCost            int
	ImageLocation       string
	FocusValue          int
	Set                 []string
	Legality            []string
	PersonalHonor       int
	HonorRequirement    int
	FlavorText          string
	Artist              string
	Rarity              string
	Clan                string
	Force               int
	Chi                 int
	Notes               string
	StorylineCredit     string
	ProvinceStrength    int
	GoldProduction      int
	StartingFamilyHonor int
	Erratum             string
	MRP                 string
}

func GetCardIDs(cardName string, keywords string) (cardIDs []string) {
	var cardidRegexp string = "cardid=(\\d+)"

	postData := url.Values{"search_13": []string{html.EscapeString(cardName)}}

	if len(keywords) > 0 {
		re := regexp.MustCompile("(exp)(\\d+)?")
		keywords = re.ReplaceAllString(keywords, "Experienced $2")
		postData.Add("search_7", keywords)
	}

	resp, err := http.PostForm("http://ia.alderac.com/oracle/dosearch", postData)

	if nil != err {
		panic("Failed to retrieve card id")
	}

	byteData, err := ioutil.ReadAll(resp.Body)
	findResults := string(byteData[:])

	re := regexp.MustCompile(cardidRegexp)

	for _, results := range re.FindAllStringSubmatch(findResults, -1) {
		if len(cardIDs) == 0 || cardIDs[len(cardIDs)-1] != results[1] { //If there is more than one card, they get duplicated because of the image tag.
			cardIDs = append(cardIDs, results[1])
		}
	}

	return
}

func GetCardByExactName(cardName string) (Card, error) {
	var title, keywords string

	results := strings.Split(cardName, " - ")
	title = results[0]

	if len(results) == 2 {
		keywords = results[1]
	}

	for _, cardID := range GetCardIDs(title, keywords) {
		card := GetCardData(cardID)
		if strings.TrimSpace(card.Title) == strings.TrimSpace(title) {
			return card, nil
		}
	}
	return Card{}, errors.New("Card not found")
}

func GetCardData(cardid string) Card {
	var cardData Card

	resp, err := http.PostForm("http://ia.alderac.com/oracle/docard", url.Values{"cardid": {cardid}})
	if nil != err {
		panic("Error posting form.")
	}

	cardData.ID, _ = strconv.Atoi(cardid)

	const shadowDataExp = "<div class=\"shadowdatashadow\" style=\"display: none;\">([^&].*?)</div>.*?<div class=\"shadowdatashadow\" style=\"display: none;\">(.*?)</div>"
	const htmlTagExp = "<.*?>"
	const replaceGcExp = "<img class=\"inlinebutton\" src=\"/oracle/resources/icon-cards-small/g_(\\d+).png\" />"
	const findImgExp = "<img .*? src=\"(showimage\\?.*?)\">"
	const extractTdExp = "<td.*?>(.*?)</td>"
	const extractGoldPDExp = "Produce (\\d+) Gold."

	var deckMap = map[string]DeckType{
		"Strategy":    Fate,
		"Item":        Fate,
		"Spell":       Fate,
		"Ring":        Fate,
		"Follower":    Fate,
		"Holding":     Dynasty,
		"Personality": Dynasty,
		"Event":       Dynasty,
		"Stronghold":  StartsInPlay,
		"Sensei":      StartsInPlay,
	}

	re := regexp.MustCompile(shadowDataExp)
	stripHtml := regexp.MustCompile(htmlTagExp)
	replGC := regexp.MustCompile(replaceGcExp)
	findImg := regexp.MustCompile(findImgExp)
	extractTd := regexp.MustCompile(extractTdExp)
	extractGoldPD := regexp.MustCompile(extractGoldPDExp)

	byteData, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		panic("Error retrieving card data.")
	}

	rawData := string(byteData[:])

	if findImg.MatchString(rawData) {
		imgLocation := findImg.FindStringSubmatch(rawData)[1]
		cardData.ImageLocation = imgLocation
	}

	for _, ele := range re.FindAllStringSubmatch(string(byteData[:]), -1) {
		if len(ele) == 3 {
			key := html.UnescapeString(strings.Replace(ele[1], "&nbsp;", " ", -1))
			rawValue := strings.Replace(ele[2], "&nbsp;", " ", -1)
			rawValue = replGC.ReplaceAllString(rawValue, "$1")

			value := []string{stripHtml.ReplaceAllString(html.UnescapeString(rawValue), "")}
			switch key {
			case "Printed Text":
				cardData.CardText = value[0]
				if extractGoldPD.MatchString(cardData.CardText) {
					res := extractGoldPD.FindStringSubmatch(cardData.CardText)
					cardData.GoldProduction, _ = strconv.Atoi(strings.TrimSpace(res[1]))
				}
			case "Printed Focus Value":
				cardData.FocusValue, _ = strconv.Atoi(value[0])
			case "Printed Gold Cost":
				cardData.GoldCost, _ = strconv.Atoi(strings.TrimSpace(value[0]))
			case "Printed Card Title":
				cardData.Title = value[0]
			case "Printed Card Type":
				cardData.Type = value[0]
				cardData.Deck = deckMap[value[0]]
			case "Printed Keywords":
				cardData.Keywords = strings.Split(value[0], " • ")
			case "Legality":
				cardData.Legality = strings.Split(value[0], " • ")
			case "Set":
				cardData.Set = strings.Split(value[0], " • ")
			case "<span title=\"Honor Requirement, Gold Cost, Personal Honor\">Printed HR/GC/PH</span>":
				results := extractTd.FindAllStringSubmatch(rawValue, -1)
				if len(results) >= 3 {
					cardData.HonorRequirement, _ = strconv.Atoi(strings.TrimSpace(results[0][1]))
					cardData.GoldCost, _ = strconv.Atoi(strings.TrimSpace(results[1][1]))
					cardData.PersonalHonor, _ = strconv.Atoi(strings.TrimSpace(results[2][1]))
				}
			case "Printed Flavor Text":
				cardData.FlavorText = value[0]
			case "Printed Artist":
				cardData.Artist = value[0]
			case "Card Number":
				cardData.CardNumber, _ = strconv.Atoi(strings.TrimSpace(value[0]))
			case "Rarity":
				cardData.Rarity = value[0]
			case "Printed Clan":
				cardData.Clan = value[0]
			case "Printed Force/Chi":
				results := extractTd.FindAllStringSubmatch(rawValue, -1)
				if len(results) >= 2 {
					cardData.Force, _ = strconv.Atoi(strings.TrimSpace(results[0][1]))
					cardData.Chi, _ = strconv.Atoi(strings.TrimSpace(results[1][1]))
				}
			case "Notes":
				cardData.Notes = value[0]
			case "Printed Storyline Credit":
				cardData.StorylineCredit = value[0]
			case "<span title=\"Province Strength, Gold Production, Starting Family Honor\">Printed PS/GP/SH</span>":
				results := extractTd.FindAllStringSubmatch(rawValue, -1)
				if len(results) >= 3 {
					cardData.ProvinceStrength, _ = strconv.Atoi(strings.TrimSpace(results[0][1]))
					cardData.GoldProduction, _ = strconv.Atoi(strings.TrimSpace(results[1][1]))
					cardData.StartingFamilyHonor, _ = strconv.Atoi(strings.TrimSpace(results[2][1]))
				}
			case "Erratum":
				cardData.Erratum = value[0]
			case "MRP":
				cardData.MRP = value[0]
			default:
				panic("Unknown Data from Oracle of the Void")
			}
		}
	}
	return cardData
}
