package processor

import (
	"context"
	"log"
	"time"
)

func InstallPaymentProcessorWatcher(pp *PaymentProcessor) {
	ticker := time.NewTicker(5 * time.Second)
	client := pp.Client

	log.Println("started payment processor watcher for ", pp.Name)
	ctx := context.Background()
	for {
		<-ticker.C
		hr, err := client.GetHealth(ctx)
		if err != nil {
			log.Println("could not get payment processor health for ", pp.Name)
			continue
		}
		pp.SetInfo(Info{
			IsAvailable:     !hr.Failing,
			MinResponseTime: hr.MinResponseTime,
		})
		log.Println("pp ", pp.Name, " is ", !hr.Failing, " with min", hr.MinResponseTime)
	}

}
