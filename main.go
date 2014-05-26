package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
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
	ID               int
	CardNumber       int
	Title            string
	Type             string
	Deck             DeckType
	Keywords         []string
	CardText         string
	GoldCost         int
	ImageLocation    string
	FocusValue       int
	Set              []string
	Legality         []string
	PersonalHonor    int
	HonorRequirement int
}

func GetCardIDs(cardName string, keywords string) (cardIDs []string) {
	var cardidRegexp string = "cardid=(\\d+)"

	postData := url.Values{"search_13": []string{cardName}}

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

	results := strings.Split(cardName, "-")
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
			case "Printed Artist":
			case "Card Number":
				cardData.CardNumber, _ = strconv.Atoi(strings.TrimSpace(value[0]))
			case "Rarity":
			case "Printed Clan":
			case "Printed Force/Chi":
			case "Notes":
			case "Printed Storyline Credit":
			case "<span title=\"Province Strength, Gold Production, Starting Family Honor\">Printed PS/GP/SH</span>":
			case "Erratum":
			case "MRP":
			default:
				fmt.Println(key)
			}
		}
	}
	return cardData
}

func ProcessDecklist(filename string) []DeckItem {
	fileContents, err := ioutil.ReadFile(filename)
	if nil != err {
		panic("Error reading file")
	}

	deckList := make([]DeckItem, 80)[0:0]

	lines := strings.Split(string(fileContents[:]), "\n")
	parseLine := regexp.MustCompile("^([0-9]*) ?(.*)$") //[ \t]{0,1}(.*)")

	outputChan := make(chan DeckItem, 500)

	defer close(outputChan)

	var cardCount int = 0

	for _, line := range lines {
		if len(line) > 0 && line[0] != '#' {
			parms := parseLine.FindStringSubmatch(line)
			count, _ := strconv.Atoi(parms[1])
			if count == 0 {
				count++
			}

			cardCount++

			go func(count int, c chan DeckItem) {
				cardData, err := GetCardByExactName(parms[2])
				if nil != err {
					fmt.Println(parms[2], err)
					c <- DeckItem{Count: 0}
				} else {
					c <- DeckItem{Count: count, CardData: cardData}
				}
			}(count, outputChan)
		}
	}

	for deckItem := range outputChan {
		fmt.Println("Found Card")
		deckList = append(deckList, deckItem)
		if cardCount == len(deckList) {
			fmt.Println("Attempting to break?")
			break
		}
	}

	return deckList
}

func PrintCard(card Card) {
	jsonData, _ := json.Marshal(card)
	fmt.Println(string(jsonData[:]))
}

type KeywordPair struct {
	Keyword string
	Count   int
}
type KeywordsByCount []KeywordPair

func (a KeywordsByCount) Len() int           { return len(a) }
func (a KeywordsByCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a KeywordsByCount) Less(i, j int) bool { return a[i].Count > a[j].Count }

func CountKeywords(deck []DeckItem) []KeywordPair {
	keywordCount := make(map[string]int)

	for _, card := range deck {
		for _, keyword := range card.CardData.Keywords {
			if keywordCount[keyword] >= 0 {
				keywordCount[keyword]++
			} else {
				keywordCount[keyword] = 1
			}
		}
	}

	var sortedKeywords = make([]KeywordPair, len(keywordCount))
	var i int = 0
	for keyword, count := range keywordCount {
		sortedKeywords[i] = KeywordPair{Keyword: keyword, Count: count}
		i++
	}
	sort.Sort(KeywordsByCount(sortedKeywords))

	return sortedKeywords
}

func CalculateFocus(deck []DeckItem) (average float32, distribution [6]int) {
	var cardCount int = 0
	var curFV int

	for _, card := range deck {
		//Only Focusable cards have a printed focus Value
		if Fate == card.CardData.Deck {
			curFV = card.CardData.FocusValue
			average = (float32(curFV)*float32(card.Count) + float32(cardCount)*average) / float32(cardCount+card.Count)
			cardCount += card.Count
			distribution[curFV] += card.Count
		}
	}

	return
}

type DeckGoldCost struct {
	Deck         DeckType
	Average      float32
	CardCount    int
	Distribution map[int]int
}

func CalculateGC(deck []DeckItem) (output map[DeckType]DeckGoldCost) {

	var avgFun = func(average float32, curWeight int, value int, weight int) float32 {
		return (float32(value*weight) + float32(curWeight)*average) / float32(curWeight+weight)
	}

	output = make(map[DeckType]DeckGoldCost)

	var curDeck DeckGoldCost
	var found bool

	for _, card := range deck {
		if card.CardData.GoldCost == 0 {
			continue
		}

		if curDeck, found = output[card.CardData.Deck]; !found {
			curDeck.Deck = card.CardData.Deck
			curDeck.Distribution = make(map[int]int)
		}

		curDeck.Average = avgFun(curDeck.Average, curDeck.CardCount, card.CardData.GoldCost, card.Count)
		curDeck.CardCount += card.Count
		curDeck.Distribution[card.CardData.GoldCost] += card.Count

		output[card.CardData.Deck] = curDeck
	}

	return
}

func main() {
	if len(os.Args) != 2 {
		panic("Not enough arguments!")
	}

	//Get the card data.
	deckList := ProcessDecklist(os.Args[1])

	//Process Statistics
	keywordCount := CountKeywords(deckList)
	avgFocusValue, dist := CalculateFocus(deckList)
	goldCosts := CalculateGC(deckList)

	//Print GoldCosts
	fmt.Println("-----------")
	for deck, results := range goldCosts {
		fmt.Printf("Deck #%v\n", deck)
		fmt.Printf("Avg GC: %v CardCount: %v\n", results.Average, results.CardCount)

		goldCosts := make([]int, len(results.Distribution))
		var i = 0
		for gc, _ := range results.Distribution {
			goldCosts[i] = gc
			i++
		}

		sort.Ints(goldCosts)

		for _, gc := range goldCosts {
			fmt.Printf("%v: %d\n", gc, results.Distribution[gc])
		}
	}

	//Output
	fmt.Println("-----------")
	for _, keyword := range keywordCount {
		fmt.Printf("%v: %d\n", keyword.Keyword, keyword.Count)
	}

	fmt.Println("-----------")
	fmt.Println("Average Focus Value: ", avgFocusValue)
	for i, fv := range dist {
		fmt.Printf("%d: %v\n", i, strings.Repeat("+", fv))
	}

	fmt.Println("-----------")
	for _, card := range deckList {
		PrintCard(card.CardData)
		fmt.Println("---------")
	}

	return
}
