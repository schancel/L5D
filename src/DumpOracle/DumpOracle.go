package main

import (
	"encoding/csv"
	"fmt"
	"ootv"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	outputFile, err := os.OpenFile("ivoryLegal.csv", syscall.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)

	if nil != err {
		panic(err)
	}

	cardIDs := ootv.GetAllCardIDs("Ivory Edition")

	defer outputFile.Close()

	csvWriter := csv.NewWriter(outputFile)

	defer csvWriter.Flush()

	csvWriter.Write([]string{"Type", "Clan", "Deck", "Title", "GoldCost", "GoldProduction", "Force", "Chi", "FocusValue", "PersonalHonor", "HonorRequirement", "Keywords", "CardText"})

	var outputChan = make(chan ootv.Card, 2000)
	var wg sync.WaitGroup

	go func(in chan ootv.Card) {
		for cardData := range in {
			csvWriter.Write([]string{cardData.Type, cardData.Clan, strconv.Itoa(int(cardData.Deck)), cardData.Title, strconv.Itoa(cardData.GoldCost), strconv.Itoa(cardData.GoldProduction), strconv.Itoa(cardData.Force), strconv.Itoa(cardData.Chi), strconv.Itoa(cardData.FocusValue), strconv.Itoa(cardData.PersonalHonor), cardData.HonorRequirement, strings.Join(cardData.Keywords, ","), cardData.CardText})
		}
	}(outputChan)

	for i, cardID := range cardIDs {
		if i%50 == 0 {
			fmt.Println("Processing result:", i)
			wg.Wait()
			fmt.Println("Done waiting.")
			time.Sleep(1000)
		}

		wg.Add(1)
		go func(id string, out chan ootv.Card) {
			cardData := ootv.GetCardData(id)
			out <- cardData
			wg.Done()
		}(cardID, outputChan)
	}

	fmt.Println("Ended")
	wg.Wait()
}
