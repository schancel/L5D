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
	//"time"
)

func main() {
	outputFile, err := os.OpenFile("ivoryLegal.csv", syscall.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)

	if nil != err {
		panic(err)
	}

	cardIDs := ootv.GetAllCardIDs("Ivory Edition Part 2")

	defer outputFile.Close()

	csvWriter := csv.NewWriter(outputFile)

	defer csvWriter.Flush()

	csvWriter.Write([]string{"Type", "Clan", "Deck", "Title", "GoldCost", "GoldProduction", "Force", "Chi", "FocusValue", "PersonalHonor", "HonorRequirement", "Keywords", "CardText"})

	var outputChan = make(chan ootv.Card)
	var cardIDChan = make(chan string)
	var wg sync.WaitGroup

	go func(in chan ootv.Card) {
		for cardData := range in {
			csvWriter.Write([]string{cardData.Type,
				cardData.Clan,
				strconv.Itoa(int(cardData.Deck)),
				cardData.Title,
				strconv.Itoa(cardData.GoldCost),
				strconv.Itoa(cardData.GoldProduction),
				strconv.Itoa(cardData.Force),
				strconv.Itoa(cardData.Chi),
				strconv.Itoa(cardData.FocusValue),
				strconv.Itoa(cardData.PersonalHonor),
				cardData.HonorRequirement,
				strings.Join(cardData.Keywords, ","), cardData.CardText})
		}
	}(outputChan)

	for i := 0; i < 10; i++ {
		go func(in chan string, out chan ootv.Card) {
			wg.Add(1)

			for cId := range in {
				cardData := ootv.GetCardData(cId)
				out <- cardData
			}
			wg.Done()
		}(cardIDChan, outputChan)
	}

	for _, cardID := range cardIDs {
		println(cardID)
		cardIDChan <- cardID
	}

	close(cardIDChan)
	wg.Wait()
	close(outputChan)

	fmt.Println("Ended")
}
