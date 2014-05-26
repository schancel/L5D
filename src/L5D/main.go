package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"ootv"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func ProcessDecklist(filename string) []ootv.DeckItem {
	fileContents, err := ioutil.ReadFile(filename)
	if nil != err {
		panic("Error reading file")
	}

	deckList := make([]ootv.DeckItem, 80)[0:0]

	lines := strings.Split(string(fileContents[:]), "\n")
	parseLine := regexp.MustCompile("^([0-9]*) ?(.*)$") //[ \t]{0,1}(.*)")

	outputChan := make(chan ootv.DeckItem, 500)

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

			go func(count int, c chan ootv.DeckItem) {
				cardData, err := ootv.GetCardByExactName(parms[2])
				if nil != err {
					fmt.Println(parms[2], err)
					c <- ootv.DeckItem{Count: 0}
				} else {
					c <- ootv.DeckItem{Count: count, CardData: cardData}
				}
			}(count, outputChan)
		}
	}

	for deckItem := range outputChan {
		deckList = append(deckList, deckItem)
		if cardCount == len(deckList) {
			break
		}
	}

	return deckList
}

func PrintCard(card ootv.Card) {
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

func CountKeywords(deck []ootv.DeckItem) []KeywordPair {
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

func WeightedRunningAvg(average float32, curWeight int, value int, weight int) float32 {
	return (float32(value*weight) + float32(curWeight)*average) / float32(curWeight+weight)
}

func CalculateFocus(deck []ootv.DeckItem) (average float32, distribution [6]int) {
	var cardCount int = 0

	for _, card := range deck {
		//Only Focusable cards have a printed focus Value
		if ootv.Fate == card.CardData.Deck {
			average = WeightedRunningAvg(average, cardCount, card.CardData.FocusValue, card.Count)
			cardCount += card.Count
			distribution[card.CardData.FocusValue] += card.Count
		}
	}

	return
}

type DeckGoldCost struct {
	Deck         ootv.DeckType
	Average      float32
	TotalWeight	 int
	CardCount    int
	Distribution map[int]int
}

func CalculateGC(deck []ootv.DeckItem) (output map[ootv.DeckType]DeckGoldCost) {
	output = make(map[ootv.DeckType]DeckGoldCost)
	var curDeck DeckGoldCost
	var found bool

	for _, card := range deck {
		curDeck, found = output[card.CardData.Deck]

		if !found {
			curDeck.Deck = card.CardData.Deck
			curDeck.Distribution = make(map[int]int)
		}
	
		curDeck.CardCount += card.Count

		if card.CardData.GoldCost > 0 {
			curDeck.Average = WeightedRunningAvg(curDeck.Average, curDeck.TotalWeight, card.CardData.GoldCost, card.Count)
			curDeck.Distribution[card.CardData.GoldCost] += card.Count
			curDeck.TotalWeight += card.Count
		}

		output[card.CardData.Deck] = curDeck
	}

	return
}

func CalculateGP(deck []ootv.DeckItem) (totalGP int, normalizedCost float32, totalHoldings int) {
	var totalGC int
	for _, card := range deck {
		if card.CardData.GoldProduction > 0 && card.CardData.GoldCost > 0 {
			totalGP += card.CardData.GoldProduction * card.Count
			totalHoldings += card.Count
			totalGC += card.Count * card.CardData.GoldCost
		}
	}

	normalizedCost = float32(totalGC)/float32(totalGP)
	return
}

func CalculateFGRatio(deck []ootv.DeckItem) (FGRatio float32) {
	var totalForce int
	var totalCost int
	for _, card := range deck {
		if card.CardData.GoldCost > 0 && card.CardData.Force > 0 {
			totalForce += card.CardData.Force * card.Count
			totalCost += card.Count * card.CardData.GoldCost
		}
	}

	FGRatio = float32(totalForce)/float32(totalCost)
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
	totalGP, normalizedCost, totalHoldings := CalculateGP(deckList)

	fmt.Println("-----------")
	fmt.Printf("Gold Production Statistics:\nTotal GP: %v, Normalized Cost: %v, Total Holdings: %v\n", totalGP, normalizedCost, totalHoldings)
	fmt.Printf("Force/Gold Ratio: %v\n", CalculateFGRatio(deckList))
	//Print GoldCosts
	fmt.Println("-----------")
	for deck, results := range goldCosts {
		fmt.Printf("Deck #%v\n", deck)
		fmt.Printf("Avg GC: %v Card Count: %v\n", results.Average, results.CardCount)

		goldCosts := make([]int, len(results.Distribution))
		var i = 0
		for gc, _ := range results.Distribution {
			goldCosts[i] = gc
			i++
		}

		sort.Ints(goldCosts)

		for _, gc := range goldCosts {
			fmt.Printf("\t%v: %v\n", gc, strings.Repeat("+", results.Distribution[gc]))
		}
	}
	fmt.Println("-----------")
	fmt.Println("Average Focus Value: ", avgFocusValue)
	for i, fv := range dist {
		fmt.Printf("%d: %v\n", i, strings.Repeat("+", fv))
	}

	//Output Keywords
	fmt.Println("-----------")
	for _, keyword := range keywordCount {
		fmt.Printf("%v: %d\n", keyword.Keyword, keyword.Count)
	}

	/*fmt.Println("-----------")
	for _, card := range deckList {
		PrintCard(card.CardData)
		fmt.Println("---------")
	}*/

	return
}
